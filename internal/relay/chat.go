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
	"github.com/yansir/cc-relayer/internal/compat"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/identity"
	"github.com/yansir/cc-relayer/internal/pool"
)

// HandleChatCompletions processes an OpenAI-format /v1/chat/completions request:
// converts to Anthropic Messages format, relays through the standard pipeline,
// and converts the response back to OpenAI format.
func (r *Relay) HandleChatCompletions(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	keyInfo := auth.GetKeyInfo(ctx)
	if keyInfo == nil {
		writeChatError(w, http.StatusUnauthorized, "invalid_api_key", "missing or invalid API key")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, int64(r.cfg.MaxRequestBodyMB)<<20)
	rawBody, err := io.ReadAll(req.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeChatError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds size limit")
			return
		}
		writeChatError(w, http.StatusBadRequest, "invalid_request_error", "failed to read request body")
		return
	}

	var chatReq compat.ChatCompletionRequest
	if err := json.Unmarshal(rawBody, &chatReq); err != nil {
		writeChatError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		return
	}

	if chatReq.Model == "" {
		writeChatError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if len(chatReq.Messages) == 0 {
		writeChatError(w, http.StatusBadRequest, "invalid_request_error", "messages is required")
		return
	}

	// Convert OpenAI request → Anthropic body
	anthropicBody, err := compat.ConvertRequest(&chatReq)
	if err != nil {
		slog.Warn("chat completions conversion failed", "error", err)
		writeChatError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	model := chatReq.Model
	isStream := chatReq.Stream
	isOpus := isOpusModel(model)
	includeUsage := chatReq.StreamOptions != nil && chatReq.StreamOptions.IncludeUsage

	// Retry loop — mirrors Handle() but with converted body and response conversion
	var excludeIDs []string
	var lastErr error
	var lastUpstreamStatus int
	var lastUpstreamBody []byte
	var forbiddenRetries int

	for attempt := 0; attempt <= r.cfg.MaxRetryAccounts; attempt++ {
		if ctx.Err() != nil {
			return
		}

		acct, err := r.pool.Pick(domain.ProviderClaude, excludeIDs, isOpus, keyInfo.BoundAccountID)
		if err != nil {
			lastErr = err
			break
		}

		accessToken, err := r.tokens.EnsureValidToken(ctx, acct.ID)
		if err != nil {
			slog.Warn("chat: token invalid, excluding", "accountId", acct.ID, "error", err)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = err
			continue
		}

		// Re-marshal for clean state (Transform mutates the map)
		attemptBody := deepCopyBody(anthropicBody)

		result := r.transformer.Transform(attemptBody, req.Header, acct)

		upstreamBytes, err := json.Marshal(result.Body)
		if err != nil {
			lastErr = fmt.Errorf("marshal body: %w", err)
			break
		}

		upReq, err := http.NewRequestWithContext(ctx, "POST", r.cfg.ClaudeAPIURL, strings.NewReader(string(upstreamBytes)))
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
			slog.Error("chat: upstream request failed", "accountId", acct.ID, "error", err)
			excludeIDs = append(excludeIDs, acct.ID)
			lastErr = err
			continue
		}

		// Retriable errors
		if shouldRetry(resp.StatusCode) && attempt < r.cfg.MaxRetryAccounts {
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			slog.Warn("chat: retriable upstream error", "status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			lastUpstreamStatus = resp.StatusCode
			lastUpstreamBody = errBody

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
			r.pool.ObserveSuccess(acct.ID, resp.Header)

			slog.Warn("chat: upstream non-retriable error",
				"status", resp.StatusCode, "accountId", acct.ID, "model", model,
				"body", truncate(string(errBody), 500))

			_, sanitizedBody := SanitizeError(resp.StatusCode, errBody)
			writeChatErrorRaw(w, resp.StatusCode, sanitizedBody)
			return
		}

		// Success
		defer resp.Body.Close()
		r.pool.ObserveSuccess(acct.ID, resp.Header)

		startTime := time.Now()
		var usage *compat.ChatUsage

		if isStream {
			var completed bool
			completed, usage = compat.StreamChatResponse(ctx, w, resp, includeUsage)
			if completed {
				r.pool.MarkLastUsed(acct.ID)
			}
		} else {
			usage = r.chatJSONResponse(w, resp)
			r.pool.MarkLastUsed(acct.ID)
		}

		if usage != nil {
			cost := calcCost(model, usage.PromptTokens, usage.CompletionTokens, 0, 0)
			go func() {
				_ = r.store.InsertRequestLog(context.Background(), &domain.RequestLog{
					UserID:       keyInfo.ID,
					AccountID:    acct.ID,
					Model:        model,
					InputTokens:  usage.PromptTokens,
					OutputTokens: usage.CompletionTokens,
					CostUSD:      cost,
					Status:       "ok",
					DurationMs:   time.Since(startTime).Milliseconds(),
					CreatedAt:    time.Now().UTC(),
				})
			}()
		}
		return
	}

	// All attempts failed
	if lastErr != nil {
		slog.Error("all chat relay attempts failed", "error", lastErr)
	}
	if lastUpstreamBody != nil {
		_, sanitizedBody := SanitizeError(lastUpstreamStatus, lastUpstreamBody)
		writeChatErrorRaw(w, lastUpstreamStatus, sanitizedBody)
		return
	}
	writeChatError(w, http.StatusServiceUnavailable, "server_error", "no available accounts")
}

// chatJSONResponse reads a non-streaming Anthropic response,
// converts it to OpenAI format, and writes it to the client.
func (r *Relay) chatJSONResponse(w http.ResponseWriter, resp *http.Response) *compat.ChatUsage {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeChatError(w, http.StatusBadGateway, "server_error", "failed to read upstream response")
		return nil
	}

	data, usage, err := compat.ConvertResponse(body)
	if err != nil {
		slog.Error("chat: response conversion failed", "error", err)
		writeChatError(w, http.StatusInternalServerError, "server_error", "failed to process upstream response")
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
	return usage
}

// deepCopyBody produces a clean copy of the body map via JSON round-trip.
func deepCopyBody(body map[string]any) map[string]any {
	data, _ := json.Marshal(body)
	var copy map[string]any
	json.Unmarshal(data, &copy)
	return copy
}

// ---------------------------------------------------------------------------
// OpenAI-format error helpers
// ---------------------------------------------------------------------------

func writeChatError(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	}
	data, _ := json.Marshal(resp)
	w.Write(data)
}

func writeChatErrorRaw(w http.ResponseWriter, status int, body []byte) {
	// Convert Anthropic error format to OpenAI error format
	var anthropicErr struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &anthropicErr) == nil && anthropicErr.Error.Message != "" {
		writeChatError(w, status, anthropicErr.Error.Type, anthropicErr.Error.Message)
		return
	}
	// Fallback: write as-is
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}
