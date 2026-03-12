package driver

import (
	"bufio"
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

var geminiBanPattern = regexp.MustCompile(`(?i)(security_policy_violated|account has been disabled)`)

func (d *GeminiDriver) Plan(input *RelayInput) RelayPlan {
	if input == nil {
		return RelayPlan{}
	}
	stream, _ := input.Body["stream"].(bool)
	return RelayPlan{
		IsStream: stream || strings.Contains(input.Path, ":streamGenerateContent"),
	}
}

func (d *GeminiDriver) BuildRequest(ctx context.Context, input *RelayInput, acct *domain.Account, token string) (*http.Request, error) {
	upstreamPath := strings.TrimPrefix(input.Path, "/gemini")
	if upstreamPath == "" || upstreamPath == input.Path {
		return nil, fmt.Errorf("invalid gemini relay path")
	}

	body := stripJSONField(input.RawBody, "model")
	if needsGeminiProject(upstreamPath) {
		state := parseGeminiState(json.RawMessage(acct.ProviderStateJSON))
		if state.ProjectID == "" {
			return nil, fmt.Errorf("gemini account missing project_id")
		}
		body = injectGeminiProject(body, state.ProjectID)
	}

	rawQuery := input.RawQuery
	if input.IsStream {
		rawQuery = withDefaultQuery(rawQuery, "alt", "sse")
	}

	upstreamURL, err := appendRawQuery(d.cfg.APIURL+upstreamPath, rawQuery)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if accept := input.Headers.Get("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	} else {
		req.Header.Set("Accept", "*/*")
	}
	req.Header.Set("User-Agent", geminiCLIUserAgent(input.Model))
	req.Header.Set("X-Goog-Api-Client", geminiAPIClientHeader)

	return req, nil
}

func (d *GeminiDriver) Interpret(statusCode int, headers http.Header, body []byte, _ string, prevState json.RawMessage) Effect {
	state := parseGeminiState(prevState)
	if info := parseGeminiLoadResponse(body); info.ProjectID != "" {
		state.ProjectID = info.ProjectID
		state.LastLoadAt = time.Now().Unix()
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
		if retryDelay := parseGeminiRetryDelay(body); retryDelay > 0 {
			until = time.Now().Add(retryDelay)
		} else if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			until = time.Now().Add(retryAfter)
		}
		return Effect{
			Kind:          EffectCooldown,
			Scope:         EffectScopeBucket,
			CooldownUntil: until,
			UpdatedState:  mustMarshalJSON(state),
		}
	case 403:
		if geminiBanPattern.Match(body) {
			return Effect{
				Kind:          EffectBlock,
				CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401),
				ErrorMessage:  fmt.Sprintf("ban signal detected: %s", truncate(string(body), 200)),
				UpdatedState:  mustMarshalJSON(state),
			}
		}
		return Effect{
			Kind:          EffectCooldown,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause403),
			UpdatedState:  mustMarshalJSON(state),
		}
	case 401:
		return Effect{
			Kind:          EffectAuthFail,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401Refresh),
			UpdatedState:  mustMarshalJSON(state),
		}
	default:
		return Effect{Kind: EffectSuccess, Scope: EffectScopeBucket, UpdatedState: mustMarshalJSON(state)}
	}
}

func (d *GeminiDriver) StreamResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) (bool, *Usage) {
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
			if u := d.ParseJSONUsage([]byte(line[6:])); u != nil {
				capturedUsage = u
			}
		}
	}
	flusher.Flush()
	return completed, capturedUsage
}

func (d *GeminiDriver) ForwardResponse(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (d *GeminiDriver) ParseJSONUsage(body []byte) *Usage {
	meta := parseGeminiUsageMetadata(body)
	if meta == nil {
		return nil
	}
	return &Usage{
		InputTokens:     meta.PromptTokenCount,
		OutputTokens:    meta.CandidatesTokenCount,
		CacheReadTokens: meta.CachedContentTokenCount,
	}
}

func (d *GeminiDriver) ShouldRetry(statusCode int) bool {
	return statusCode == 529 || statusCode == 429 || statusCode == 401 || statusCode == 403
}

func (d *GeminiDriver) RetrySameAccount(_ int, _ []byte, _ int) bool { return false }

func (d *GeminiDriver) ParseNonRetriable(_ int, _ []byte) bool { return false }

func (d *GeminiDriver) WriteError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"message":"%s","code":%d}}`, msg, status)
}

func (d *GeminiDriver) WriteUpstreamError(w http.ResponseWriter, statusCode int, body []byte, _ bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

func (d *GeminiDriver) InterceptRequest(_ http.ResponseWriter, _ map[string]interface{}, _ string) bool {
	return false
}
