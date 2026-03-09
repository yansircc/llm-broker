package driver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/identity"
)

// ClaudeState holds the provider-specific rate-limit state for Claude accounts.
type ClaudeState struct {
	FiveHourUtil      float64 `json:"five_hour_util"`
	FiveHourReset     int64   `json:"five_hour_reset"`
	SevenDayUtil      float64 `json:"seven_day_util"`
	SevenDayReset     int64   `json:"seven_day_reset"`
	OpusCooldownUntil int64   `json:"opus_cooldown_until,omitempty"`
}

// ClaudeConfig holds the configuration needed by the Claude driver.
type ClaudeConfig struct {
	APIURL     string
	APIVersion string
	BetaHeader string
	Pauses     ErrorPauses
}

// ClaudeDriver implements Driver for Claude.
type ClaudeDriver struct {
	cfg         ClaudeConfig
	transformer *identity.Transformer
}

var banSignalPattern = regexp.MustCompile(`(?i)(organization has been disabled|account has been disabled|Too many active sessions|only authorized for use with claude code)`)

func NewClaudeDriver(cfg ClaudeConfig, transformer *identity.Transformer) *ClaudeDriver {
	return &ClaudeDriver{cfg: cfg, transformer: transformer}
}

func (d *ClaudeDriver) Provider() domain.Provider { return domain.ProviderClaude }

func (d *ClaudeDriver) Info() ProviderInfo {
	return ProviderInfo{
		Label:               "Claude",
		RelayPaths:          []string{"/v1/messages", "/v1/messages/count_tokens"},
		OAuthStateRequired:  true,
		CallbackPlaceholder: "https://platform.claude.com/oauth/code/callback?code=...",
		CallbackHint:        "email and organization metadata are fetched after token exchange.",
		ProbeLabel:          "haiku",
	}
}

func (d *ClaudeDriver) Models() []Model {
	return []Model{
		{ID: "claude-opus-4-6", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-opus-4-5", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-opus-4-1", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-opus-4", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-sonnet-4-6", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-sonnet-4-5", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-sonnet-4", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
		{ID: "claude-haiku-4-5", Object: "model", Created: 1709164800, OwnedBy: "anthropic", ContextWindow: 200000},
	}
}

// ---------------------------------------------------------------------------
// Relay
// ---------------------------------------------------------------------------

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
		return Effect{Kind: EffectSuccess, UpdatedState: mustMarshalJSON(state)}

	case 529:
		return Effect{
			Kind:          EffectOverload,
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
			CooldownUntil: until,
			UpdatedState:  mustMarshalJSON(state),
		}

	case 403:
		if banSignalPattern.MatchString(string(body)) {
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
	}

	return Effect{Kind: EffectSuccess, UpdatedState: mustMarshalJSON(state)}
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

func (d *ClaudeDriver) ExtractSessionUUID(body map[string]interface{}) string {
	if metadata, ok := body["metadata"].(map[string]interface{}); ok {
		if uid, ok := metadata["user_id"].(string); ok {
			return identity.ExtractSessionUUID(uid)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Lifecycle (OAuth)
// ---------------------------------------------------------------------------

func (d *ClaudeDriver) GenerateAuthURL() (string, OAuthSession, error) {
	return generateClaudeAuthURL()
}

func (d *ClaudeDriver) ExchangeCode(ctx context.Context, code, verifier, state string) (*ExchangeResult, error) {
	result, err := exchangeClaudeCode(ctx, code, verifier, state)
	if err != nil {
		return nil, err
	}

	orgUUID, email, orgName, err := fetchClaudeOrgWithToken(ctx, result.AccessToken)
	if err != nil {
		// Fallback: try API header method
		orgUUID = fetchOrgUUIDFromAPIHeader(ctx, d.cfg.APIURL, result.AccessToken, d.cfg.APIVersion, d.cfg.BetaHeader)
		email = "account-" + time.Now().Format("0102-1504")
	}

	if orgUUID == "" {
		return nil, fmt.Errorf("could not obtain organization UUID (subject)")
	}

	return &ExchangeResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		Subject:      orgUUID,
		Email:        email,
		Identity: map[string]interface{}{
			"orgUUID": orgUUID,
			"orgName": orgName,
			"email":   email,
		},
	}, nil
}

func (d *ClaudeDriver) RefreshToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	return refreshClaudeToken(ctx, client, refreshToken)
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

func (d *ClaudeDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error) {
	body := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", d.cfg.APIURL, strings.NewReader(body))
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(req.Header, token, d.cfg.APIVersion, d.cfg.BetaHeader)
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProbeResult{}, err
	}

	result := ProbeResult{
		Effect:  d.Interpret(resp.StatusCode, resp.Header, bodyBytes, "", json.RawMessage(acct.ProviderStateJSON)),
		Observe: true,
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	result.ClearCooldown = true
	return result, nil
}

func (d *ClaudeDriver) DescribeAccount(acct *domain.Account) []AccountField {
	if acct == nil || acct.Identity == nil {
		return nil
	}
	if orgName, ok := acct.Identity["orgName"].(string); ok && orgName != "" {
		return []AccountField{{Label: "organization", Value: orgName}}
	}
	return nil
}

func (d *ClaudeDriver) AutoPriority(state json.RawMessage) int {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return 50
	}
	fiveRemain := 100.0
	if s.FiveHourUtil > 0 {
		fiveRemain = (1.0 - s.FiveHourUtil) * 100
	}
	sevenRemain := 100.0
	if s.SevenDayUtil > 0 {
		sevenRemain = (1.0 - s.SevenDayUtil) * 100
	}
	pri := fiveRemain
	if sevenRemain < pri {
		pri = sevenRemain
	}
	return int(pri)
}

func (d *ClaudeDriver) IsStale(state json.RawMessage, now time.Time) bool {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return false
	}
	nowUnix := now.Unix()
	return (s.FiveHourReset > 0 && s.FiveHourReset < nowUnix) ||
		(s.SevenDayReset > 0 && s.SevenDayReset < nowUnix) ||
		(s.FiveHourUtil > 0 && s.FiveHourReset == 0) ||
		(s.SevenDayUtil > 0 && s.SevenDayReset == 0)
}

func (d *ClaudeDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return time.Time{}
	}
	nowUnix := now.Unix()
	var cooldownUntil int64
	if s.FiveHourUtil >= 0.99 && s.FiveHourReset > nowUnix {
		cooldownUntil = s.FiveHourReset
	}
	if s.SevenDayUtil >= 0.99 && s.SevenDayReset > nowUnix && s.SevenDayReset > cooldownUntil {
		cooldownUntil = s.SevenDayReset
	}
	if cooldownUntil > 0 {
		return time.Unix(cooldownUntil, 0).UTC()
	}
	return time.Time{}
}

func (d *ClaudeDriver) CanServe(state json.RawMessage, model string, now time.Time) bool {
	if !isOpusModel(model) {
		return true
	}
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return true
	}
	return s.OpusCooldownUntil == 0 || now.Unix() >= s.OpusCooldownUntil
}

func (d *ClaudeDriver) CalcCost(model string, usage *Usage) float64 {
	if usage == nil {
		return 0
	}
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
	return (float64(usage.InputTokens)*inPrice + float64(usage.OutputTokens)*outPrice +
		float64(usage.CacheReadTokens)*cacheReadPrice + float64(usage.CacheCreateTokens)*cacheCreatePrice) / 1_000_000
}

func (d *ClaudeDriver) GetUtilization(state json.RawMessage) []UtilWindow {
	var s ClaudeState
	if json.Unmarshal(state, &s) != nil {
		return nil
	}
	var windows []UtilWindow
	if s.FiveHourUtil > 0 || s.FiveHourReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "5h",
			Pct:   int(s.FiveHourUtil * 100),
			Reset: s.FiveHourReset,
		})
	}
	if s.SevenDayUtil > 0 || s.SevenDayReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "7d",
			Pct:   int(s.SevenDayUtil * 100),
			Reset: s.SevenDayReset,
		})
	}
	return windows
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (d *ClaudeDriver) captureState(headers http.Header, prevState json.RawMessage) ClaudeState {
	var prev ClaudeState
	if len(prevState) > 0 {
		_ = json.Unmarshal(prevState, &prev)
	}
	s := ClaudeState{
		OpusCooldownUntil: prev.OpusCooldownUntil,
	}
	if headers == nil {
		return s
	}
	if v := headers.Get("anthropic-ratelimit-unified-5h-utilization"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			s.FiveHourUtil = f
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-5h-reset"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			s.FiveHourReset = secs
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-7d-utilization"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			s.SevenDayUtil = f
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-7d-reset"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			s.SevenDayReset = secs
		}
	}
	return s
}

func parseClaudeUsage(data string) *Usage {
	var wrapper struct {
		Usage *struct {
			InputTokens       int `json:"input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			CacheReadTokens   int `json:"cache_read_input_tokens"`
			CacheCreateTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &wrapper) == nil && wrapper.Usage != nil {
		return &Usage{
			InputTokens:       wrapper.Usage.InputTokens,
			OutputTokens:      wrapper.Usage.OutputTokens,
			CacheReadTokens:   wrapper.Usage.CacheReadTokens,
			CacheCreateTokens: wrapper.Usage.CacheCreateTokens,
		}
	}
	return nil
}

func (d *ClaudeDriver) ParseJSONUsage(body []byte) *Usage {
	return parseClaudeUsage(string(body))
}

func sanitizeClaudeError(statusCode int, body []byte) (int, []byte) {
	// Re-use the relay package's SanitizeError if available,
	// or inline minimal version for the driver.
	var parsed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Type != "" {
		return statusCode, buildClaudeErrorJSON(parsed.Error.Type, parsed.Error.Message)
	}
	return statusCode, buildClaudeErrorJSON("api_error", "unexpected upstream error")
}

func sanitizeClaudeErrorJSON(statusCode int, body []byte) []byte {
	_, sanitized := sanitizeClaudeError(statusCode, body)
	return sanitized
}

func buildClaudeErrorJSON(errType, msg string) []byte {
	resp := struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Type: "error",
		Error: struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}{Type: errType, Message: msg},
	}
	data, _ := json.Marshal(resp)
	return data
}

func fetchOrgUUIDFromAPIHeader(ctx context.Context, apiURL, accessToken, apiVersion, betaHeader string) string {
	body := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(req.Header, accessToken, apiVersion, betaHeader)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return resp.Header.Get("Anthropic-Organization-Id")
}

func isOpusModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "opus")
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(value); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
