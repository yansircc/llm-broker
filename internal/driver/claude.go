package driver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/identity"
)

var banSignalPattern = regexp.MustCompile(`(?i)(organization has been disabled|account has been disabled|Too many active sessions|only authorized for use with claude code)`)

// ---------------------------------------------------------------------------
// Relay
// ---------------------------------------------------------------------------

func (d *ClaudeDriver) Plan(input *RelayInput) RelayPlan {
	if input == nil {
		return RelayPlan{}
	}

	stream, _ := input.Body["stream"].(bool)
	sessionUUID := claudeSessionUUID(input.Body)
	return RelayPlan{
		IsStream:                 stream,
		IsCountTokens:            strings.HasSuffix(input.Path, "/count_tokens"),
		SessionUUID:              sessionUUID,
		RejectUnavailableSession: sessionUUID != "" && claudeRequiresFreshSession(input.Body),
	}
}

func (d *ClaudeDriver) BuildRequest(ctx context.Context, input *RelayInput, acct *domain.Account, token string) (*http.Request, error) {
	// Re-parse body for clean state
	var body map[string]interface{}
	if err := json.Unmarshal(input.RawBody, &body); err != nil {
		return nil, fmt.Errorf("body re-parse: %w", err)
	}

	result := d.transformer.Transform(body, input.Headers, acct)

	upstreamBody, err := json.Marshal(result.Body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	apiURL := d.cfg.APIURL
	if input.IsCountTokens {
		apiURL += "/count_tokens"
	}
	upstreamURL, err := appendRawQuery(apiURL, input.RawQuery)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", upstreamURL, strings.NewReader(string(upstreamBody)))
	if err != nil {
		return nil, err
	}

	for k, vals := range result.Headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	identity.SetRequiredHeaders(req.Header, token, d.cfg.APIVersion, d.cfg.BetaHeader)
	if input.IsStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	return req, nil
}

func (d *ClaudeDriver) Interpret(statusCode int, headers http.Header, body []byte, model string, prevState json.RawMessage) Effect {
	state := d.captureState(headers, prevState)
	if state.OpusCooldownUntil > 0 && state.OpusCooldownUntil <= time.Now().Unix() {
		state.OpusCooldownUntil = 0
	}
	switch statusCode {
	case http.StatusOK:
		return Effect{Kind: EffectSuccess, Scope: EffectScopeBucket, UpdatedState: mustMarshalJSON(state)}

	case 529:
		return Effect{
			Kind:          EffectOverload,
			Scope:         EffectScopeBucket,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause529),
			UpdatedState:  mustMarshalJSON(state),
		}

	case 429:
		until := time.Now().Add(d.cfg.Pauses.Pause429)
		if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			until = time.Now().Add(retryAfter)
		} else if resetStr := headers.Get("anthropic-ratelimit-unified-reset"); resetStr != "" {
			if resetTime, err := time.Parse(time.RFC3339, resetStr); err == nil {
				until = resetTime
			}
		}
		if isOpusModel(model) {
			if resetStr := headers.Get("anthropic-ratelimit-unified-reset"); resetStr != "" {
				if resetTime, err := time.Parse(time.RFC3339, resetStr); err == nil {
					state.OpusCooldownUntil = resetTime.Unix()
				}
			}
		}
		return Effect{
			Kind:          EffectCooldown,
			Scope:         EffectScopeBucket,
			CooldownUntil: until,
			UpdatedState:  mustMarshalJSON(state),
		}

	case 403:
		if banSignalPattern.MatchString(string(body)) {
			return Effect{
				Kind:          EffectBlock,
				Scope:         EffectScopeBucket,
				CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401),
				ErrorMessage:  fmt.Sprintf("ban signal detected: %s", truncate(string(body), 200)),
				UpdatedState:  mustMarshalJSON(state),
			}
		}
		return Effect{
			Kind:          EffectCooldown,
			Scope:         EffectScopeBucket,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause403),
			UpdatedState:  mustMarshalJSON(state),
		}

	case 401:
		return Effect{
			Kind:          EffectAuthFail,
			Scope:         EffectScopeBucket,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401Refresh),
			UpdatedState:  mustMarshalJSON(state),
		}
	}

	return Effect{Kind: EffectSuccess, Scope: EffectScopeBucket, UpdatedState: mustMarshalJSON(state)}
}

func (d *ClaudeDriver) StreamResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) (bool, *Usage) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		d.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return false, nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var lastEventType string
	var capturedUsage *Usage
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
			if u := parseClaudeUsage(line[6:]); u != nil {
				capturedUsage = u
			}
		}
	}
	flusher.Flush()
	return completed, capturedUsage
}

func (d *ClaudeDriver) ForwardResponse(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (d *ClaudeDriver) ShouldRetry(statusCode int) bool {
	return statusCode == 529 || statusCode == 429 || statusCode == 401 || statusCode == 403
}

func (d *ClaudeDriver) RetrySameAccount(statusCode int, body []byte, priorAttempts int) bool {
	if statusCode == 403 && !banSignalPattern.MatchString(string(body)) {
		return priorAttempts < 2
	}
	return false
}

func (d *ClaudeDriver) ParseNonRetriable(statusCode int, body []byte) bool {
	if statusCode == 429 && bytes.Contains(body, []byte("Extra usage is required")) {
		return true
	}
	return false
}

func (d *ClaudeDriver) WriteError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":"api_error","message":"%s"}}`, msg)
}

func (d *ClaudeDriver) WriteUpstreamError(w http.ResponseWriter, statusCode int, body []byte, isStream bool) {
	if isStream {
		sanitizedStatus, _ := sanitizeClaudeError(statusCode, body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(sanitizedStatus)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", sanitizeClaudeErrorJSON(statusCode, body))
	} else {
		sanitizedStatus, sanitizedBody := sanitizeClaudeError(statusCode, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(sanitizedStatus)
		w.Write(sanitizedBody)
	}
}

// ---------------------------------------------------------------------------
// Provider-specific hooks
// ---------------------------------------------------------------------------

func (d *ClaudeDriver) InterceptRequest(w http.ResponseWriter, body map[string]interface{}, model string) bool {
	if !identity.IsWarmupRequest(body) {
		return false
	}
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
	return true
}

func claudeSessionUUID(body map[string]interface{}) string {
	if metadata, ok := body["metadata"].(map[string]interface{}); ok {
		if uid, ok := metadata["user_id"].(string); ok {
			return identity.ExtractSessionUUID(uid)
		}
	}
	return ""
}

func claudeRequiresFreshSession(body map[string]interface{}) bool {
	messages, _ := body["messages"].([]interface{})
	if len(messages) > 1 {
		return true
	}
	if len(messages) == 1 {
		if m, ok := messages[0].(map[string]interface{}); ok {
			if content, ok := m["content"].([]interface{}); ok {
				userTexts := 0
				for _, block := range content {
					if b, ok := block.(map[string]interface{}); ok && b["type"] == "text" {
						userTexts++
					}
				}
				if userTexts > 1 {
					return true
				}
			}
		}
	}
	tools, _ := body["tools"].([]interface{})
	return len(tools) == 0
}

func (d *ClaudeDriver) ParseJSONUsage(body []byte) *Usage {
	return parseClaudeUsage(string(body))
}
