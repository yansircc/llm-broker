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
	"strconv"
	"strings"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
)

// CodexState holds the provider-specific rate-limit state for Codex accounts.
type CodexState struct {
	PrimaryUtil    float64 `json:"primary_util"`
	PrimaryReset   int64   `json:"primary_reset"`
	SecondaryUtil  float64 `json:"secondary_util"`
	SecondaryReset int64   `json:"secondary_reset"`
}

// CodexConfig holds the configuration needed by the Codex driver.
type CodexConfig struct {
	APIURL string
	Pauses ErrorPauses
}

// CodexDriver implements Driver for Codex (OpenAI).
type CodexDriver struct {
	cfg CodexConfig
}

// Codex-specific ban patterns (separate from Claude).
var codexBanPattern = regexp.MustCompile(`(?i)(account has been disabled|organization has been disabled)`)

func NewCodexDriver(cfg CodexConfig) *CodexDriver {
	return &CodexDriver{cfg: cfg}
}

func (d *CodexDriver) Provider() domain.Provider { return domain.ProviderCodex }

func (d *CodexDriver) Info() ProviderInfo {
	return ProviderInfo{
		Label:               "Codex",
		RelayPaths:          []string{"/openai/responses"},
		OAuthStateRequired:  false,
		CallbackPlaceholder: "http://localhost:1455/auth/callback?code=...",
		CallbackHint:        "account metadata is extracted from the id_token.",
		ProbeLabel:          "codex",
	}
}

func (d *CodexDriver) Models() []Model {
	return []Model{
		{ID: "gpt-5.4", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 1050000},
		{ID: "gpt-5.3-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.2-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex-max", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "gpt-5.1-codex-mini", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 400000},
		{ID: "codex-1", Object: "model", Created: 1709164800, OwnedBy: "openai", ContextWindow: 192000},
	}
}

// ---------------------------------------------------------------------------
// Relay
// ---------------------------------------------------------------------------

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
		return Effect{Kind: EffectSuccess, UpdatedState: state}

	case 529:
		return Effect{
			Kind:          EffectOverload,
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
			CooldownUntil: until,
			UpdatedState:  state,
		}

	case 403:
		if codexBanPattern.MatchString(string(body)) {
			return Effect{
				Kind:          EffectBlock,
				CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401),
				ErrorMessage:  fmt.Sprintf("ban signal detected: %s", truncate(string(body), 200)),
			}
		}
		return Effect{
			Kind:          EffectCooldown,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause403),
		}

	case 401:
		return Effect{
			Kind:          EffectAuthFail,
			CooldownUntil: time.Now().Add(d.cfg.Pauses.Pause401Refresh),
		}
	}

	return Effect{Kind: EffectSuccess}
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

func (d *CodexDriver) ExtractSessionUUID(_ map[string]interface{}) string {
	return "" // Codex has no session binding
}

// ---------------------------------------------------------------------------
// Lifecycle (OAuth)
// ---------------------------------------------------------------------------

func (d *CodexDriver) GenerateAuthURL() (string, OAuthSession, error) {
	return generateCodexAuthURL()
}

func (d *CodexDriver) ExchangeCode(ctx context.Context, code, verifier, _ string) (*ExchangeResult, error) {
	result, err := exchangeCodexCode(ctx, code, verifier)
	if err != nil {
		return nil, err
	}

	email := "codex-" + time.Now().Format("0102-1504")
	identity := make(map[string]interface{})
	var subject string

	if result.IDInfo != nil {
		if result.IDInfo.Email != "" {
			email = result.IDInfo.Email
		}
		subject = result.IDInfo.ChatGPTAccountID
		identity["chatgptAccountId"] = result.IDInfo.ChatGPTAccountID
		identity["email"] = result.IDInfo.Email
		identity["orgTitle"] = result.IDInfo.OrgTitle
	}

	if subject == "" {
		return nil, fmt.Errorf("could not obtain chatgptAccountId (subject) from ID token")
	}

	return &ExchangeResult{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		Subject:      subject,
		Email:        email,
		Identity:     identity,
	}, nil
}

func (d *CodexDriver) RefreshToken(ctx context.Context, client *http.Client, refreshToken string) (*TokenResponse, error) {
	return refreshCodexToken(ctx, client, refreshToken)
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

func (d *CodexDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error) {
	if acct.Subject == "" {
		return ProbeResult{}, fmt.Errorf("codex account missing subject")
	}
	body := `{"model":"gpt-5.1-codex","stream":true,"store":false,"instructions":"Reply: ok","input":[{"role":"user","content":"t"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", d.cfg.APIURL, strings.NewReader(body))
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Host", "chatgpt.com")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Chatgpt-Account-Id", acct.Subject)
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

	scanner := bufio.NewScanner(bytes.NewReader(bodyBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: response.output_text.delta") {
			result.ClearCooldown = true
			return result, nil
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"error":{`) {
			return result, fmt.Errorf("upstream error in stream")
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	return result, fmt.Errorf("stream ended without output")
}

func (d *CodexDriver) DescribeAccount(acct *domain.Account) []AccountField {
	if acct == nil || acct.Identity == nil {
		return nil
	}
	if orgTitle, ok := acct.Identity["orgTitle"].(string); ok && orgTitle != "" {
		return []AccountField{{Label: "organization", Value: orgTitle}}
	}
	return nil
}

func (d *CodexDriver) AutoPriority(state json.RawMessage) int {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return 50
	}
	primaryRemain := 100.0
	if s.PrimaryUtil > 0 {
		primaryRemain = (1.0 - s.PrimaryUtil) * 100
	}
	secondaryRemain := 100.0
	if s.SecondaryUtil > 0 {
		secondaryRemain = (1.0 - s.SecondaryUtil) * 100
	}
	pri := primaryRemain
	if secondaryRemain < pri {
		pri = secondaryRemain
	}
	return int(pri)
}

func (d *CodexDriver) IsStale(state json.RawMessage, now time.Time) bool {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return false
	}
	nowUnix := now.Unix()
	return (s.PrimaryReset > 0 && s.PrimaryReset < nowUnix) ||
		(s.SecondaryReset > 0 && s.SecondaryReset < nowUnix) ||
		(s.PrimaryUtil > 0 && s.PrimaryReset == 0) ||
		(s.SecondaryUtil > 0 && s.SecondaryReset == 0)
}

func (d *CodexDriver) ComputeExhaustedCooldown(state json.RawMessage, now time.Time) time.Time {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return time.Time{}
	}
	nowUnix := now.Unix()
	var cooldownUntil int64
	if s.PrimaryUtil >= 0.99 && s.PrimaryReset > nowUnix {
		cooldownUntil = s.PrimaryReset
	}
	if s.SecondaryUtil >= 0.99 && s.SecondaryReset > nowUnix && s.SecondaryReset > cooldownUntil {
		cooldownUntil = s.SecondaryReset
	}
	if cooldownUntil > 0 {
		return time.Unix(cooldownUntil, 0).UTC()
	}
	return time.Time{}
}

func (d *CodexDriver) CalcCost(model string, usage *Usage) float64 {
	if usage == nil {
		return 0
	}
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
	return (float64(usage.InputTokens)*inPrice + float64(usage.OutputTokens)*outPrice +
		float64(usage.CacheReadTokens)*cacheReadPrice) / 1_000_000
}

func (d *CodexDriver) GetUtilization(state json.RawMessage) []UtilWindow {
	var s CodexState
	if json.Unmarshal(state, &s) != nil {
		return nil
	}
	var windows []UtilWindow
	if s.PrimaryUtil > 0 || s.PrimaryReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "primary",
			Pct:   int(s.PrimaryUtil * 100),
			Reset: s.PrimaryReset,
		})
	}
	if s.SecondaryUtil > 0 || s.SecondaryReset > 0 {
		windows = append(windows, UtilWindow{
			Label: "secondary",
			Pct:   int(s.SecondaryUtil * 100),
			Reset: s.SecondaryReset,
		})
	}
	return windows
}

func (d *CodexDriver) CanServe(_ json.RawMessage, _ string, _ time.Time) bool {
	return true
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (d *CodexDriver) captureHeaders(headers http.Header) json.RawMessage {
	if headers == nil {
		return nil
	}
	var s CodexState
	if v := headers.Get("x-codex-primary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			s.PrimaryUtil = pct / 100
		}
	}
	if v := headers.Get("x-codex-primary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			s.PrimaryReset = time.Now().Unix() + int64(secs)
		}
	}
	if v := headers.Get("x-codex-secondary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			s.SecondaryUtil = pct / 100
		}
	}
	if v := headers.Get("x-codex-secondary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			s.SecondaryReset = time.Now().Unix() + int64(secs)
		}
	}
	data, _ := json.Marshal(s)
	return data
}

// codexUsageFields is the shared usage structure for Codex responses.
type codexUsageFields struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Details      *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func codexUsageToUsage(u *codexUsageFields) *Usage {
	if u == nil {
		return nil
	}
	result := &Usage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
	}
	if u.Details != nil {
		result.CacheReadTokens = u.Details.CachedTokens
	}
	return result
}

// parseCodexUsage parses usage from an SSE response.completed event.
func parseCodexUsage(data string) *Usage {
	var wrapper struct {
		Type     string `json:"type"`
		Response struct {
			Usage *codexUsageFields `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(data), &wrapper) != nil {
		return nil
	}
	return codexUsageToUsage(wrapper.Response.Usage)
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

func parseCodexResetsIn(body []byte) time.Duration {
	var envelope struct {
		Error struct {
			ResetsInSeconds int `json:"resets_in_seconds"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.ResetsInSeconds > 0 {
		return time.Duration(envelope.Error.ResetsInSeconds) * time.Second
	}
	return 0
}

func extractCodexErrorMessage(body []byte) string {
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
