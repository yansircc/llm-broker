package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yansir/cc-relayer/internal/auth"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/pool"
)

// TransportProvider supplies per-account HTTP clients.
type TransportProvider interface {
	GetClient(acct *domain.Account) *http.Client
}

// StoreWriter writes request logs.
type StoreWriter interface {
	InsertRequestLog(ctx context.Context, log *domain.RequestLog) error
}

// Config holds relay-relevant configuration.
type Config struct {
	MaxRequestBodyMB  int
	MaxRetryAccounts  int
	SessionBindingTTL time.Duration
}

// Relay orchestrates the request forwarding pipeline.
type Relay struct {
	pool      *pool.Pool
	tokens    TokenProvider
	store     StoreWriter
	cfg       Config
	transport TransportProvider
	bus       *events.Bus
	drivers   map[domain.Provider]driver.Driver
}

// TokenProvider returns valid access tokens.
type TokenProvider interface {
	EnsureValidToken(ctx context.Context, accountID string) (string, error)
}

func New(
	p *pool.Pool,
	tp TokenProvider,
	sw StoreWriter,
	cfg Config,
	transport TransportProvider,
	bus *events.Bus,
	drivers map[domain.Provider]driver.Driver,
) *Relay {
	return &Relay{
		pool:      p,
		tokens:    tp,
		store:     sw,
		cfg:       cfg,
		transport: transport,
		bus:       bus,
		drivers:   drivers,
	}
}

func (r *Relay) driverFor(provider domain.Provider) driver.Driver {
	return r.drivers[provider]
}

// HandleProvider processes relay requests for a specific provider.
func (r *Relay) HandleProvider(provider domain.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		drv := r.driverFor(provider)
		if drv == nil {
			http.Error(w, "unknown provider", http.StatusNotFound)
			return
		}
		r.handleWithDriver(w, req, drv)
	}
}

func (r *Relay) handleWithDriver(w http.ResponseWriter, req *http.Request, drv driver.Driver) {
	ctx := req.Context()

	keyInfo := auth.GetKeyInfo(ctx)
	if keyInfo == nil {
		drv.WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, int64(r.cfg.MaxRequestBodyMB)<<20)

	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			drv.WriteError(w, http.StatusRequestEntityTooLarge, "request body exceeds size limit")
			return
		}
		drv.WriteError(w, http.StatusBadRequest, "failed to read request body")
		r.bus.Publish(events.Event{Type: events.EventRelayError, Message: fmt.Sprintf("%s: failed to read request body: %s", drv.Provider(), err.Error())})
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		drv.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	model, _ := body["model"].(string)
	isStream, _ := body["stream"].(bool)
	input := &driver.RelayInput{
		Body:     body,
		RawBody:  rawBody,
		Headers:  req.Header,
		RawQuery: req.URL.RawQuery,
		Model:    model,
		IsStream: isStream,
	}

	// Provider-specific interception (Claude: warmup; Codex: no-op)
	if drv.InterceptRequest(w, body, model) {
		return
	}

	// Count tokens: single attempt, no retry (Claude-only)
	if strings.HasSuffix(req.URL.Path, "/count_tokens") {
		input.IsCountTokens = true
		r.handleCountTokens(w, req, drv, input, keyInfo)
		return
	}

	// Session binding (returns "" for providers without sessions)
	sessionUUID := drv.ExtractSessionUUID(body)
	oldSession := isOldSession(body)
	var sessionBoundAccountID string
	if sessionUUID != "" {
		if boundID, ok := r.pool.GetSessionBinding(sessionUUID); ok {
			if r.pool.IsAvailableFor(boundID, drv, model) {
				sessionBoundAccountID = boundID
				r.pool.RenewSessionBinding(sessionUUID, r.cfg.SessionBindingTTL)
			} else if oldSession {
				slog.Warn("session pollution detected", "sessionUUID", sessionUUID, "boundAccountId", boundID)
				drv.WriteError(w, http.StatusBadRequest, "bound account unavailable, please start a new session")
				return
			}
		}
	}

	// Retry loop
	var excludeIDs []string
	var lastErr error
	var lastUpstreamStatus int
	var lastUpstreamBody []byte
	forbiddenRetries := make(map[string]int) // per-account same-account retry counter

	for attempt := 0; attempt <= r.cfg.MaxRetryAccounts; attempt++ {
		if ctx.Err() != nil {
			return
		}

		boundID := keyInfo.BoundAccountID
		if attempt == 0 && sessionBoundAccountID != "" && boundID == "" {
			boundID = sessionBoundAccountID
		}

		acct, err := r.pool.Pick(drv, excludeIDs, model, boundID)
		if err != nil {
			lastErr = err
			break
		}

		accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
		if err != nil {
			slog.Warn("token invalid, excluding account", "accountId", acct.ID, "error", err)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = err
			continue
		}

		upReq, err := drv.BuildRequest(ctx, input, acct, accessToken)
		if err != nil {
			lastErr = fmt.Errorf("build request: %w", err)
			break
		}

		client := r.transport.GetClient(acct)
		resp, err := client.Do(upReq)
		if err != nil {
			slog.Error("upstream request failed", "accountId", acct.ID, "error", err)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = err
			continue
		}

		// Handle retriable errors
		if drv.ShouldRetry(resp.StatusCode) && attempt < r.cfg.MaxRetryAccounts {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			slog.Warn("retriable upstream error", "status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			lastUpstreamStatus = resp.StatusCode
			lastUpstreamBody = errBody

			// Non-retriable permanent rejection (e.g., "Extra usage is required")
			if drv.ParseNonRetriable(resp.StatusCode, errBody) {
				drv.WriteUpstreamError(w, resp.StatusCode, errBody, input.IsStream)
				return
			}

			// Same-account retry (Claude: 403 non-ban retry ×2 before exclude)
			if drv.RetrySameAccount(resp.StatusCode, errBody, forbiddenRetries[acct.ID]) {
				forbiddenRetries[acct.ID]++
				lastErr = fmt.Errorf("upstream %d (retry %d)", resp.StatusCode, forbiddenRetries[acct.ID])
				continue
			}

			effect := drv.Interpret(resp.StatusCode, resp.Header, errBody, model, json.RawMessage(acct.ProviderStateJSON))
			r.pool.Observe(acct.ID, effect)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = fmt.Errorf("upstream %d", resp.StatusCode)
			continue
		}

		// Non-retriable error
		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// Capture rate-limit headers even on non-retriable errors
			effect := drv.Interpret(http.StatusOK, resp.Header, nil, model, json.RawMessage(acct.ProviderStateJSON))
			r.pool.Observe(acct.ID, effect)

			slog.Warn("upstream non-retriable error",
				"status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			drv.WriteUpstreamError(w, resp.StatusCode, errBody, input.IsStream)
			return
		}

		// Success
		defer resp.Body.Close()
		effect := drv.Interpret(http.StatusOK, resp.Header, nil, model, json.RawMessage(acct.ProviderStateJSON))
		r.pool.Observe(acct.ID, effect)

		if sessionUUID != "" {
			r.pool.SetSessionBinding(sessionUUID, acct.ID, r.cfg.SessionBindingTTL)
		}

		startTime := time.Now()
		var usage *driver.Usage
		if input.IsStream {
			_, usage = drv.StreamResponse(ctx, w, resp)
		} else {
			// Non-streaming: read body, extract usage, write response
			respBody, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				drv.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
				return
			}
			usage = drv.ParseJSONUsage(respBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
		}

		if usage != nil {
			cost := drv.CalcCost(model, usage)
			go func() {
				_ = r.store.InsertRequestLog(context.Background(), &domain.RequestLog{
					UserID:            keyInfo.ID,
					AccountID:         acct.ID,
					Model:             model,
					InputTokens:       usage.InputTokens,
					OutputTokens:      usage.OutputTokens,
					CacheReadTokens:   usage.CacheReadTokens,
					CacheCreateTokens: usage.CacheCreateTokens,
					CostUSD:           cost,
					Status:            "ok",
					DurationMs:        time.Since(startTime).Milliseconds(),
					CreatedAt:         time.Now().UTC(),
				})
			}()
		}
		return
	}

	// All attempts failed
	if lastErr != nil {
		slog.Error("all relay attempts failed", "error", lastErr, "provider", drv.Provider())
		r.bus.Publish(events.Event{Type: events.EventRelayError, Message: fmt.Sprintf("%s: all relay attempts failed: %s", drv.Provider(), lastErr.Error())})
	}
	if lastUpstreamBody != nil {
		drv.WriteUpstreamError(w, lastUpstreamStatus, lastUpstreamBody, input.IsStream)
		return
	}
	drv.WriteError(w, http.StatusServiceUnavailable, "no available accounts")
}

// handleCountTokens handles count_tokens with single attempt, no retry.
func (r *Relay) handleCountTokens(w http.ResponseWriter, req *http.Request, drv driver.Driver, input *driver.RelayInput, keyInfo *auth.KeyInfo) {
	ctx := req.Context()

	acct, err := r.pool.Pick(drv, nil, input.Model, keyInfo.BoundAccountID)
	if err != nil {
		slog.Warn("count_tokens: account selection failed", "error", err)
		drv.WriteError(w, http.StatusServiceUnavailable, "no available accounts")
		return
	}

	accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		slog.Warn("count_tokens: token unavailable", "error", err, "accountId", acct.ID)
		drv.WriteError(w, http.StatusServiceUnavailable, "token unavailable")
		return
	}

	upReq, err := drv.BuildRequest(ctx, input, acct, accessToken)
	if err != nil {
		drv.WriteError(w, http.StatusInternalServerError, "failed to build request")
		return
	}

	client := r.transport.GetClient(acct)
	resp, err := client.Do(upReq)
	if err != nil {
		slog.Error("count_tokens upstream failed", "error", err)
		drv.WriteError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer resp.Body.Close()

	drv.ForwardResponse(w, resp)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isOpusModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "opus")
}

func isOldSession(body map[string]interface{}) bool {
	messages, _ := body["messages"].([]interface{})
	if len(messages) > 1 {
		return true
	}
	if len(messages) == 1 {
		if m, ok := messages[0].(map[string]interface{}); ok {
			if content, ok := m["content"].([]interface{}); ok {
				userTexts := 0
				for _, block := range content {
					if b, ok := block.(map[string]interface{}); ok {
						if b["type"] == "text" {
							userTexts++
						}
					}
				}
				if userTexts > 1 {
					return true
				}
			}
		}
	}
	tools, _ := body["tools"].([]interface{})
	if len(tools) == 0 {
		return true
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
