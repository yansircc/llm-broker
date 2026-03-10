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

// ---------------------------------------------------------------------------
// Relay
// ---------------------------------------------------------------------------

func (d *CodexDriver) Plan(input *RelayInput) RelayPlan {
	if input == nil {
		return RelayPlan{}
	}
	stream, _ := input.Body["stream"].(bool)
	return RelayPlan{IsStream: stream}
}

func (d *CodexDriver) BuildRequest(ctx context.Context, input *RelayInput, acct *domain.Account, token string) (*http.Request, error) {
	if acct.Subject == "" {
		return nil, fmt.Errorf("codex account missing subject")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", d.cfg.APIURL, strings.NewReader(string(input.RawBody)))
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

func (d *CodexDriver) Interpret(statusCode int, headers http.Header, body []byte, model string, _ json.RawMessage) Effect {
	switch statusCode {
	case http.StatusOK:
		state := d.captureHeaders(headers)
		return Effect{Kind: EffectSuccess, Scope: EffectScopeBucket, UpdatedState: state}

	case 529:
		return Effect{
			Kind:          EffectOverload,
			Scope:         EffectScopeBucket,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause529),
		}

	case 429:
		state := d.captureHeaders(headers)

		until := time.Now().Add(d.cfg.Pauses.Pause429)
		if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			until = time.Now().Add(retryAfter)
		} else if len(body) > 0 {
			if resetsIn := parseCodexResetsIn(body); resetsIn > 0 {
				until = time.Now().Add(resetsIn)
			}
		}

		return Effect{
			Kind:          EffectCooldown,
			Scope:         EffectScopeBucket,
			CooldownUntil: until,
			UpdatedState:  state,
		}

	case 403:
		if codexBanPattern.MatchString(string(body)) {
			return Effect{
				Kind:          EffectBlock,
				Scope:         EffectScopeBucket,
				CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401),
				ErrorMessage:  fmt.Sprintf("ban signal detected: %s", truncate(string(body), 200)),
			}
		}
		return Effect{
			Kind:          EffectCooldown,
			Scope:         EffectScopeBucket,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause403),
		}

	case 401:
		return Effect{
			Kind:          EffectAuthFail,
			Scope:         EffectScopeBucket,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401Refresh),
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"message":"%s","type":"error","code":%d}}`, msg, status)
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
