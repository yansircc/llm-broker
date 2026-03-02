package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yansir/cc-relayer/internal/auth"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/identity"
	"github.com/yansir/cc-relayer/internal/oauth"
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
	ClaudeAPIURL     string
	ClaudeAPIVersion string
	ClaudeBetaHeader string
	CodexAPIURL      string

	MaxRequestBodyMB int
	MaxRetryAccounts int
	SessionBindingTTL time.Duration
}

// Relay orchestrates the request forwarding pipeline.
type Relay struct {
	pool        *pool.Pool
	tokens      *oauth.TokenManager
	transformer *identity.Transformer
	store       StoreWriter
	cfg         Config
	transport   TransportProvider
}

func New(
	p *pool.Pool,
	tm *oauth.TokenManager,
	trans *identity.Transformer,
	sw StoreWriter,
	cfg Config,
	tp TransportProvider,
) *Relay {
	return &Relay{
		pool:        p,
		tokens:      tm,
		transformer: trans,
		store:       sw,
		cfg:         cfg,
		transport:   tp,
	}
}

// Handle processes a Claude relay request end-to-end.
func (r *Relay) Handle(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	keyInfo := auth.GetKeyInfo(ctx)
	if keyInfo == nil {
		writeError(w, http.StatusUnauthorized, "authentication_error", "not authenticated")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, int64(r.cfg.MaxRequestBodyMB)<<20)

	body, rawBody, err := parseBody(req)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds size limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		return
	}

	model, _ := body["model"].(string)
	isStream, _ := body["stream"].(bool)
	isOpus := isOpusModel(model)

	// Warmup interception
	if identity.IsWarmupRequest(body) {
		slog.Debug("warmup intercepted", "model", model)
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		for _, event := range identity.WarmupEvents(model) {
			w.Write([]byte(event))
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		}
		return
	}

	// Session binding
	sessionUUID := extractSessionUUID(body)
	oldSession := isOldSession(body)
	var sessionBoundAccountID string
	if sessionUUID != "" {
		if boundID, ok := r.pool.GetSessionBinding(sessionUUID); ok {
			acct := r.pool.Get(boundID)
			if acct != nil && acct.Status == domain.StatusActive && acct.Schedulable {
				sessionBoundAccountID = boundID
				r.pool.RenewSessionBinding(sessionUUID, r.cfg.SessionBindingTTL)
			} else if oldSession {
				slog.Warn("session pollution detected", "sessionUUID", sessionUUID, "boundAccountId", boundID)
				writeError(w, http.StatusBadRequest, "session_binding_error",
					"bound account unavailable, please start a new session")
				return
			}
		}
	}

	// Retry loop
	var excludeIDs []string
	var lastErr error
	var lastUpstreamStatus int
	var lastUpstreamBody []byte
	var forbiddenRetries int

	for attempt := 0; attempt <= r.cfg.MaxRetryAccounts; attempt++ {
		if ctx.Err() != nil {
			return
		}

		boundID := keyInfo.BoundAccountID
		if attempt == 0 && sessionBoundAccountID != "" && boundID == "" {
			boundID = sessionBoundAccountID
		}

		acct, err := r.pool.Pick(domain.ProviderClaude, excludeIDs, isOpus, boundID)
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

		// Re-parse body for clean state each attempt
		var attemptBody map[string]interface{}
		if err := json.Unmarshal(rawBody, &attemptBody); err != nil {
			lastErr = fmt.Errorf("body re-parse: %w", err)
			break
		}

		result := r.transformer.Transform(attemptBody, req.Header, acct)

		upstreamBody, err := json.Marshal(result.Body)
		if err != nil {
			lastErr = fmt.Errorf("marshal body: %w", err)
			break
		}

		upstreamURL, err := appendRawQuery(r.cfg.ClaudeAPIURL, req.URL.RawQuery)
		if err != nil {
			lastErr = err
			break
		}

		upReq, err := http.NewRequestWithContext(ctx, "POST", upstreamURL, strings.NewReader(string(upstreamBody)))
		if err != nil {
			lastErr = err
			break
		}

		for k, vals := range result.Headers {
			for _, v := range vals {
				upReq.Header.Add(k, v)
			}
		}
		identity.SetRequiredHeaders(upReq.Header, accessToken, r.cfg.ClaudeAPIVersion, r.cfg.ClaudeBetaHeader)
		if isStream {
			upReq.Header.Set("Accept", "text/event-stream")
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
		if shouldRetry(resp.StatusCode) && attempt < r.cfg.MaxRetryAccounts {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			slog.Warn("retriable upstream error", "status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			lastUpstreamStatus = resp.StatusCode
			lastUpstreamBody = errBody

			// 403 non-ban: retry same account up to 2 times without Observe/cooldown.
			// Only on the 3rd 403 do we Observe (which triggers cooldown) and exclude.
			if resp.StatusCode == 403 && !pool.IsBanSignal(string(errBody)) {
				forbiddenRetries++
				if forbiddenRetries <= 2 {
					lastErr = fmt.Errorf("upstream 403 (retry %d)", forbiddenRetries)
					continue
				}
			}

			r.pool.Observe(pool.UpstreamResult{
				AccountID: acct.ID, StatusCode: resp.StatusCode,
				Headers: resp.Header, ErrBody: errBody,
				Model: model, IsOpus: isOpus,
			})

			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = fmt.Errorf("upstream %d", resp.StatusCode)
			continue
		}

		// Non-retriable error
		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			r.pool.ObserveSuccess(acct.ID, resp.Header) // capture rate limit headers even on non-retriable errors

			slog.Warn("upstream non-retriable error",
				"status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			if isStream {
				sanitizedStatus, _ := SanitizeError(resp.StatusCode, errBody)
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(sanitizedStatus)
				fmt.Fprint(w, SanitizeSSEError(resp.StatusCode, errBody))
			} else {
				sanitizedStatus, sanitizedBody := SanitizeError(resp.StatusCode, errBody)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(sanitizedStatus)
				w.Write(sanitizedBody)
			}
			return
		}

		// Success
		defer resp.Body.Close()
		r.pool.ObserveSuccess(acct.ID, resp.Header)

		if result.SessionHash != "" && sessionUUID != "" {
			r.pool.SetSessionBinding(sessionUUID, acct.ID, r.cfg.SessionBindingTTL)
		}

		startTime := time.Now()
		var usage *usageData
		if isStream {
			var completed bool
			completed, usage = streamResponse(ctx, w, resp)
			if completed {
				r.pool.MarkLastUsed(acct.ID)
			}
		} else {
			usage = jsonResponse(w, resp)
			r.pool.MarkLastUsed(acct.ID)
		}

		if usage != nil {
			cost := calcCost(model, usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheCreateTokens)
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
		slog.Error("all relay attempts failed", "error", lastErr)
	}
	if lastUpstreamBody != nil {
		sanitizedStatus, sanitizedBody := SanitizeError(lastUpstreamStatus, lastUpstreamBody)
		if isStream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(sanitizedStatus)
			fmt.Fprint(w, SanitizeSSEError(lastUpstreamStatus, lastUpstreamBody))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(sanitizedStatus)
			w.Write(sanitizedBody)
		}
		return
	}
	writeError(w, http.StatusServiceUnavailable, "overloaded_error", "no available accounts")
}

// HandleCodex processes a Codex relay request.
func (r *Relay) HandleCodex(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	keyInfo := auth.GetKeyInfo(ctx)
	if keyInfo == nil {
		writeCodexError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, int64(r.cfg.MaxRequestBodyMB)<<20)

	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeCodexError(w, http.StatusRequestEntityTooLarge, "request body exceeds size limit")
			return
		}
		writeCodexError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		writeCodexError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	model, _ := body["model"].(string)

	var excludeIDs []string
	var lastErr error
	var lastUpstreamStatus int
	var lastUpstreamBody []byte

	for attempt := 0; attempt <= r.cfg.MaxRetryAccounts; attempt++ {
		if ctx.Err() != nil {
			return
		}

		acct, err := r.pool.Pick(domain.ProviderCodex, excludeIDs, false, keyInfo.BoundAccountID)
		if err != nil {
			lastErr = err
			break
		}

		accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
		if err != nil {
			slog.Warn("codex token invalid, excluding account", "accountId", acct.ID, "error", err)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = err
			continue
		}

		upReq, err := http.NewRequestWithContext(ctx, "POST", r.cfg.CodexAPIURL, strings.NewReader(string(rawBody)))
		if err != nil {
			lastErr = err
			break
		}

		for _, h := range []string{"Content-Type", "Accept", "Codex-Version"} {
			if v := req.Header.Get(h); v != "" {
				upReq.Header.Set(h, v)
			}
		}
		if upReq.Header.Get("Content-Type") == "" {
			upReq.Header.Set("Content-Type", "application/json")
		}

		upReq.Header.Set("Authorization", "Bearer "+accessToken)
		upReq.Header.Set("Host", "chatgpt.com")
		if acct.ExtInfo != nil {
			if accountID, ok := acct.ExtInfo["chatgptAccountId"].(string); ok && accountID != "" {
				upReq.Header.Set("Chatgpt-Account-Id", accountID)
			}
		}

		client := r.transport.GetClient(acct)
		resp, err := client.Do(upReq)
		if err != nil {
			slog.Error("codex upstream request failed", "accountId", acct.ID, "error", err)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = err
			continue
		}

		if shouldRetry(resp.StatusCode) && attempt < r.cfg.MaxRetryAccounts {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			slog.Warn("codex retriable upstream error", "status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			r.pool.Observe(pool.UpstreamResult{
				AccountID: acct.ID, StatusCode: resp.StatusCode,
				Headers: resp.Header, ErrBody: errBody, Model: model,
			})

			lastUpstreamStatus = resp.StatusCode
			lastUpstreamBody = errBody
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = fmt.Errorf("upstream %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			r.pool.ObserveSuccess(acct.ID, resp.Header)

			slog.Warn("codex upstream error", "status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			if msg := extractErrorMessage(errBody); msg != "" {
				writeCodexError(w, resp.StatusCode, msg)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(resp.StatusCode)
				w.Write(errBody)
			}
			return
		}

		// Success
		defer resp.Body.Close()
		r.pool.ObserveSuccess(acct.ID, resp.Header)

		startTime := time.Now()
		usage := streamCodexResponse(ctx, w, resp)

		r.pool.MarkLastUsed(acct.ID)

		if usage != nil {
			cost := calcCodexCost(model, usage.InputTokens, usage.OutputTokens, usage.InputTokensCached)
			go func() {
				_ = r.store.InsertRequestLog(context.Background(), &domain.RequestLog{
					UserID:            keyInfo.ID,
					AccountID:         acct.ID,
					Model:             model,
					InputTokens:       usage.InputTokens,
					OutputTokens:      usage.OutputTokens,
					CacheReadTokens:   usage.InputTokensCached,
					CostUSD:           cost,
					Status:            "ok",
					DurationMs:        time.Since(startTime).Milliseconds(),
					CreatedAt:         time.Now().UTC(),
				})
			}()
		}
		return
	}

	if lastErr != nil {
		slog.Error("all codex relay attempts failed", "error", lastErr)
	}
	if lastUpstreamBody != nil {
		if msg := extractErrorMessage(lastUpstreamBody); msg != "" {
			writeCodexError(w, lastUpstreamStatus, msg)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(lastUpstreamStatus)
			w.Write(lastUpstreamBody)
		}
		return
	}
	writeCodexError(w, http.StatusServiceUnavailable, "no available codex accounts")
}

// HandleCountTokens proxies token counting requests.
func (r *Relay) HandleCountTokens(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	keyInfo := auth.GetKeyInfo(ctx)
	if keyInfo == nil {
		writeError(w, http.StatusUnauthorized, "authentication_error", "not authenticated")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, int64(r.cfg.MaxRequestBodyMB)<<20)
	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "failed to read body")
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		return
	}
	model, _ := body["model"].(string)

	acct, err := r.pool.Pick(domain.ProviderClaude, nil, isOpusModel(model), keyInfo.BoundAccountID)
	if err != nil {
		slog.Warn("count_tokens: account selection failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "overloaded_error", "no available accounts")
		return
	}

	accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		slog.Warn("count_tokens: token unavailable", "error", err, "accountId", acct.ID)
		writeError(w, http.StatusServiceUnavailable, "api_error", "token unavailable")
		return
	}

	result := r.transformer.Transform(body, req.Header, acct)
	upstreamBody, err := json.Marshal(result.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "api_error", "failed to marshal request body")
		return
	}

	upstreamURL, err := appendRawQuery(r.cfg.ClaudeAPIURL+"/count_tokens", req.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "api_error", "failed to build upstream url")
		return
	}

	upReq, err := http.NewRequestWithContext(ctx, "POST", upstreamURL, strings.NewReader(string(upstreamBody)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "api_error", "failed to create request")
		return
	}
	for k, vals := range result.Headers {
		for _, v := range vals {
			upReq.Header.Add(k, v)
		}
	}
	identity.SetRequiredHeaders(upReq.Header, accessToken, r.cfg.ClaudeAPIVersion, r.cfg.ClaudeBetaHeader)

	client := r.transport.GetClient(acct)
	resp, err := client.Do(upReq)
	if err != nil {
		slog.Error("count_tokens upstream failed", "error", err)
		writeError(w, http.StatusBadGateway, "api_error", "upstream request failed")
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "failed to read upstream response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

func streamResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) (bool, *usageData) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "api_error", "streaming not supported")
		return false, nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var lastEventType string
	var capturedUsage *usageData
	completed := true

	for scanner.Scan() {
		if ctx.Err() != nil {
			completed = false
			break
		}
		line := scanner.Text()
		fmt.Fprintf(w, "%s\n", line)
		if line == "" {
			flusher.Flush()
		}
		if strings.HasPrefix(line, "event: ") {
			lastEventType = strings.TrimPrefix(line, "event: ")
		}
		if lastEventType == "message_delta" && strings.HasPrefix(line, "data: ") {
			if u := parseUsage(line[6:]); u != nil {
				capturedUsage = u
			}
		}
	}
	flusher.Flush()
	return completed, capturedUsage
}

func jsonResponse(w http.ResponseWriter, resp *http.Response) *usageData {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "failed to read upstream response")
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
	return parseUsage(string(body))
}

func streamCodexResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) *codexUsageData {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeCodexError(w, http.StatusInternalServerError, "streaming not supported")
		return nil
	}

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var capturedUsage *codexUsageData
	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := scanner.Text()
		fmt.Fprintf(w, "%s\n", line)
		if line == "" {
			flusher.Flush()
		}
		if strings.HasPrefix(line, "data: ") {
			if u := parseCodexUsage(line[6:]); u != nil {
				capturedUsage = u
			}
		}
	}
	flusher.Flush()
	return capturedUsage
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func shouldRetry(statusCode int) bool {
	return statusCode == 529 || statusCode == 429 || statusCode == 401 || statusCode == 403
}

func appendRawQuery(rawURL, rawQuery string) (string, error) {
	if rawQuery == "" {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	additional, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}
	for k, vals := range additional {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func parseBody(req *http.Request) (map[string]interface{}, []byte, error) {
	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, nil, err
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, nil, err
	}
	return body, rawBody, nil
}

func isOpusModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "opus")
}

func extractSessionUUID(body map[string]interface{}) string {
	if metadata, ok := body["metadata"].(map[string]interface{}); ok {
		if uid, ok := metadata["user_id"].(string); ok {
			return identity.ExtractSessionUUID(uid)
		}
	}
	return ""
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

func parseUsage(data string) *usageData {
	var wrapper struct {
		Usage *usageData `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &wrapper) == nil && wrapper.Usage != nil {
		return wrapper.Usage
	}
	return nil
}

func parseCodexUsage(data string) *codexUsageData {
	var wrapper struct {
		Type     string `json:"type"`
		Response struct {
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				Details      *struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"input_tokens_details"`
			} `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(data), &wrapper) != nil || wrapper.Response.Usage == nil {
		return nil
	}
	u := &codexUsageData{
		InputTokens:  wrapper.Response.Usage.InputTokens,
		OutputTokens: wrapper.Response.Usage.OutputTokens,
	}
	if wrapper.Response.Usage.Details != nil {
		u.InputTokensCached = wrapper.Response.Usage.Details.CachedTokens
	}
	return u
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func writeError(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":"%s","message":"%s"}}`, errType, msg)
}

func writeCodexError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"message":"%s","type":"error","code":%d}}`, msg, status)
}

// extractErrorMessage pulls the error.message string from an OpenAI-style JSON error body.
// Returns empty string if parsing fails.
func extractErrorMessage(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		return envelope.Error.Message
	}
	return ""
}

type usageData struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CacheReadTokens   int `json:"cache_read_input_tokens"`
	CacheCreateTokens int `json:"cache_creation_input_tokens"`
}

type codexUsageData struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	InputTokensCached int `json:"input_tokens_cached"`
}

// calcCost computes the estimated cost in USD based on model and token counts.
func calcCost(model string, input, output, cacheRead, cacheCreate int) float64 {
	lower := strings.ToLower(model)
	var inPrice, outPrice, cacheReadPrice, cacheCreatePrice float64
	switch {
	case strings.Contains(lower, "opus"):
		inPrice, outPrice, cacheReadPrice, cacheCreatePrice = 15, 75, 1.50, 18.75
	case strings.Contains(lower, "haiku"):
		inPrice, outPrice, cacheReadPrice, cacheCreatePrice = 0.80, 4, 0.08, 1
	default: // sonnet and unknown
		inPrice, outPrice, cacheReadPrice, cacheCreatePrice = 3, 15, 0.30, 3.75
	}
	return (float64(input)*inPrice + float64(output)*outPrice +
		float64(cacheRead)*cacheReadPrice + float64(cacheCreate)*cacheCreatePrice) / 1_000_000
}

// calcCodexCost computes the estimated cost in USD for Codex models.
func calcCodexCost(model string, input, output, cacheRead int) float64 {
	lower := strings.ToLower(model)
	var inPrice, outPrice, cacheReadPrice float64
	switch {
	case strings.Contains(lower, "o3"):
		inPrice, outPrice, cacheReadPrice = 2, 8, 0.50
	case strings.Contains(lower, "o4-mini"):
		inPrice, outPrice, cacheReadPrice = 1.10, 4.40, 0.275
	case strings.Contains(lower, "codex-mini"):
		inPrice, outPrice, cacheReadPrice = 1.50, 6, 0.375
	case strings.Contains(lower, "4.1-nano"):
		inPrice, outPrice, cacheReadPrice = 0.10, 0.40, 0.025
	case strings.Contains(lower, "4.1-mini"):
		inPrice, outPrice, cacheReadPrice = 0.40, 1.60, 0.10
	case strings.Contains(lower, "4.1"):
		inPrice, outPrice, cacheReadPrice = 2, 8, 0.50
	default:
		inPrice, outPrice, cacheReadPrice = 2, 8, 0.50
	}
	return (float64(input)*inPrice + float64(output)*outPrice +
		float64(cacheRead)*cacheReadPrice) / 1_000_000
}
