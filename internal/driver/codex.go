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
)

// Codex-specific ban patterns (separate from Claude).
var codexBanPattern = regexp.MustCompile(`(?i)(account has been disabled|organization has been disabled)`)

const codexSSEPreflightMaxBytes = 8 * 1024

var codexSSECapacityPatterns = []string{
	"selected model is at capacity",
	"model_at_capacity",
	"server_is_overloaded",
	"service_unavailable_error",
}

func codexCapacityPattern(body []byte) (string, bool) {
	lower := strings.ToLower(string(body))
	for _, pattern := range codexSSECapacityPatterns {
		if strings.Contains(lower, pattern) {
			return pattern, true
		}
	}
	return "", false
}

func firstCompleteSSEEvent(data []byte) ([]byte, bool) {
	end := -1
	separatorLen := 0
	for _, separator := range [][]byte{[]byte("\n\n"), []byte("\r\n\r\n")} {
		if idx := bytes.Index(data, separator); idx >= 0 && (end < 0 || idx < end) {
			end = idx
			separatorLen = len(separator)
		}
	}
	if end < 0 {
		return nil, false
	}
	return data[:end+separatorLen], true
}

func codexSSECapacityPattern(event []byte) (string, bool) {
	pattern, found := codexCapacityPattern(event)
	if !found {
		return "", false
	}

	var eventName string
	var dataLines [][]byte
	for _, line := range bytes.Split(event, []byte("\n")) {
		line = bytes.TrimSpace(line)
		switch {
		case bytes.HasPrefix(line, []byte("event:")):
			eventName = strings.ToLower(strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("event:")))))
		case bytes.HasPrefix(line, []byte("data:")):
			dataLines = append(dataLines, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
		}
	}
	if strings.Contains(eventName, "error") || strings.Contains(eventName, "failed") {
		return pattern, true
	}

	payload := bytes.Join(dataLines, []byte("\n"))
	var envelope map[string]any
	if json.Unmarshal(payload, &envelope) != nil {
		return "", false
	}
	eventType, _ := envelope["type"].(string)
	eventType = strings.ToLower(eventType)
	if strings.Contains(eventType, "error") || strings.Contains(eventType, "failed") {
		return pattern, true
	}
	if envelope["error"] != nil {
		return pattern, true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Relay
// ---------------------------------------------------------------------------

func (d *CodexDriver) Plan(input *RelayInput) RelayPlan {
	if input == nil {
		return RelayPlan{}
	}
	stream, _ := input.Body["stream"].(bool)
	affinity := codexRouteAffinity(input)
	return RelayPlan{IsStream: stream, Affinity: affinity}
}

func codexRouteAffinity(input *RelayInput) RouteAffinity {
	if input == nil {
		return RouteAffinity{}
	}

	continuity := AffinityPrefer
	if nonEmptyString(input.Body["previous_response_id"]) != "" || codexConversationID(input.Body["conversation"]) != "" {
		continuity = AffinityRequire
	}

	if key := nonEmptyString(input.Body["prompt_cache_key"]); key != "" {
		return RouteAffinity{RawKey: key, Kind: "prompt-cache-key", Continuity: continuity}
	}
	if key := codexConversationID(input.Body["conversation"]); key != "" {
		return RouteAffinity{RawKey: key, Kind: "conversation", Continuity: continuity}
	}
	if input.Headers != nil {
		if key := strings.TrimSpace(input.Headers.Get("session-id")); key != "" {
			return RouteAffinity{RawKey: key, Kind: "session-id", Continuity: continuity}
		}
	}
	return RouteAffinity{Continuity: continuity}
}

func nonEmptyString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func codexConversationID(value any) string {
	switch conversation := value.(type) {
	case string:
		return strings.TrimSpace(conversation)
	case map[string]any:
		return nonEmptyString(conversation["id"])
	default:
		return ""
	}
}

func (d *CodexDriver) BuildRequest(ctx context.Context, input *RelayInput, acct *domain.Account, token string) (*http.Request, error) {
	if acct.Subject == "" {
		return nil, fmt.Errorf("codex account missing subject")
	}
	body, err := normalizeCodexRequestBody(input.RawBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", d.cfg.APIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for _, h := range []string{"Content-Type", "Accept", "Codex-Version"} {
		if v := input.Headers.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Host", "chatgpt.com")
	req.Header.Set("Chatgpt-Account-Id", acct.Subject)

	return req, nil
}

func (d *CodexDriver) Interpret(statusCode int, headers http.Header, body []byte, model string, prevState json.RawMessage) Effect {
	upstreamErrorType, upstreamErrorMessage := parseCodexErrorInfo(body)
	switch statusCode {
	case http.StatusOK:
		d.noteGoodModel(model)
		state := d.captureHeaders(headers, prevState)
		return Effect{Kind: EffectSuccess, Scope: EffectScopeBucket, UpdatedState: state}

	case 529:
		return Effect{
			Kind:                 EffectOverload,
			Scope:                EffectScopeBucket,
			CooldownUntil:        time.Now().Add(d.cfg.Pauses.Pause529),
			UpstreamStatus:       529,
			UpstreamErrorType:    upstreamErrorType,
			UpstreamErrorMessage: upstreamErrorMessage,
		}

	case 429:
		state := d.captureHeaders(headers, prevState)

		until := time.Now().Add(d.cfg.Pauses.Pause429)
		if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			until = time.Now().Add(retryAfter)
		} else if len(body) > 0 {
			if resetsIn := parseCodexResetsIn(body); resetsIn > 0 {
				until = time.Now().Add(resetsIn)
			}
		}

		return Effect{
			Kind:                 EffectCooldown,
			Scope:                EffectScopeBucket,
			CooldownUntil:        until,
			UpstreamStatus:       429,
			UpstreamErrorType:    upstreamErrorType,
			UpstreamErrorMessage: upstreamErrorMessage,
			UpdatedState:         state,
		}

	case 403:
		if codexBanPattern.MatchString(string(body)) {
			return Effect{
				Kind:                 EffectBlock,
				Scope:                EffectScopeBucket,
				CooldownUntil:        time.Now().Add(d.cfg.Pauses.Pause401),
				ErrorMessage:         fmt.Sprintf("ban signal detected: %s", truncate(string(body), 200)),
				UpstreamStatus:       403,
				UpstreamErrorType:    upstreamErrorType,
				UpstreamErrorMessage: upstreamErrorMessage,
			}
		}
		return Effect{
			Kind:                 EffectCooldown,
			Scope:                EffectScopeBucket,
			CooldownUntil:        time.Now().Add(d.cfg.Pauses.Pause403),
			UpstreamStatus:       403,
			UpstreamErrorType:    upstreamErrorType,
			UpstreamErrorMessage: upstreamErrorMessage,
		}

	case 401:
		return Effect{
			Kind:                 EffectAuthFail,
			Scope:                EffectScopeBucket,
			CooldownUntil:        time.Now().Add(d.cfg.Pauses.Pause401Refresh),
			UpstreamStatus:       401,
			UpstreamErrorType:    upstreamErrorType,
			UpstreamErrorMessage: upstreamErrorMessage,
		}

	case http.StatusBadRequest:
		// A 400 is a request-level rejection (e.g. an unsupported model), not the
		// credential's fault. Surface it as an observable reject without applying
		// any cooldown or state change — and never let it fall through to the
		// success default, which once masked dead-model probe failures.
		return Effect{
			Kind:                 EffectReject,
			Scope:                EffectScopeBucket,
			UpstreamStatus:       http.StatusBadRequest,
			UpstreamErrorType:    upstreamErrorType,
			UpstreamErrorMessage: upstreamErrorMessage,
		}
	}

	return Effect{Kind: EffectSuccess, Scope: EffectScopeBucket}
}

func (d *CodexDriver) StreamResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) (bool, *Usage) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		d.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return false, nil
	}

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

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
		if strings.HasPrefix(line, "data: ") {
			if u := parseCodexUsage(line[6:]); u != nil {
				capturedUsage = u
			}
		}
	}
	flusher.Flush()
	return completed, capturedUsage
}

type replayReadCloser struct {
	io.Reader
	io.Closer
}

// PreflightStream detects Codex capacity errors embedded in HTTP-200 SSE before
// relay commits downstream bytes. Accepted prefixes are restored byte-for-byte.
func (d *CodexDriver) PreflightStream(ctx context.Context, resp *http.Response, _ string, prevState json.RawMessage) (StreamPreflight, error) {
	if resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK ||
		!strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return StreamPreflight{}, nil
	}

	original := resp.Body
	prefix := make([]byte, 0, codexSSEPreflightMaxBytes)
	chunk := make([]byte, 1024)
	capacityResult := func(pattern string) StreamPreflight {
		body := append([]byte(nil), prefix...)
		return StreamPreflight{
			Effect: &Effect{
				Kind:                 EffectCooldown,
				Scope:                EffectScopeBucket,
				CooldownUntil:        time.Now().Add(d.cfg.Pauses.Pause429),
				UpstreamStatus:       http.StatusTooManyRequests,
				UpstreamErrorType:    "model_at_capacity",
				UpstreamErrorMessage: pattern,
				UpdatedState:         d.captureHeaders(resp.Header, prevState),
			},
			ErrorBody: body,
		}
	}
	for len(prefix) < codexSSEPreflightMaxBytes {
		if err := ctx.Err(); err != nil {
			_ = original.Close()
			return StreamPreflight{}, err
		}
		remaining := codexSSEPreflightMaxBytes - len(prefix)
		if remaining < len(chunk) {
			chunk = chunk[:remaining]
		}
		n, err := original.Read(chunk)
		if n > 0 {
			prefix = append(prefix, chunk[:n]...)
			if event, complete := firstCompleteSSEEvent(prefix); complete {
				if pattern, ok := codexSSECapacityPattern(event); ok {
					_ = original.Close()
					return capacityResult(pattern), nil
				}
				break
			}
		}
		if err != nil {
			if err != io.EOF {
				_ = original.Close()
				return StreamPreflight{}, err
			}
			if pattern, ok := codexSSECapacityPattern(prefix); ok {
				_ = original.Close()
				return capacityResult(pattern), nil
			}
			break
		}
	}

	resp.Body = &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix), original),
		Closer: original,
	}
	return StreamPreflight{}, nil
}

func (d *CodexDriver) ForwardResponse(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (d *CodexDriver) ShouldRetry(statusCode int) bool {
	return statusCode == 529 || statusCode == 429 || statusCode == 401 || statusCode == 403
}

func (d *CodexDriver) RetrySameAccount(_ int, _ []byte, _ int) bool {
	return false // Codex doesn't do same-account retry
}

func (d *CodexDriver) ParseNonRetriable(statusCode int, body []byte) bool {
	if statusCode == 429 && bytes.Contains(body, []byte("Extra usage is required")) {
		return true
	}
	return false
}

func (d *CodexDriver) WriteError(w http.ResponseWriter, status int, msg string) {
	writeDriverJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "error",
			"code":    status,
		},
	})
}

func (d *CodexDriver) WriteUpstreamError(w http.ResponseWriter, statusCode int, body []byte, _ bool) {
	if msg := extractCodexErrorMessage(body); msg != "" {
		d.WriteError(w, statusCode, msg)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(body)
	}
}

// ---------------------------------------------------------------------------
// Provider-specific hooks
// ---------------------------------------------------------------------------

func (d *CodexDriver) InterceptRequest(_ http.ResponseWriter, _ map[string]interface{}, _ string) bool {
	return false // Codex has no warmup interception
}

func (d *CodexDriver) ParseJSONUsage(body []byte) *Usage {
	// Try SSE response.completed format first ({"response":{"usage":{...}}})
	if u := parseCodexUsage(string(body)); u != nil {
		return u
	}
	// Non-streaming: usage is at top level ({"usage":{"input_tokens":...}})
	var wrapper struct {
		Usage *codexUsageFields `json:"usage"`
	}
	if json.Unmarshal(body, &wrapper) != nil {
		return nil
	}
	return codexUsageToUsage(wrapper.Usage)
}
