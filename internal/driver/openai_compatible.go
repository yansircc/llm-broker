package driver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

type OpenAICompatibleDriver struct {
	pauses ErrorPauses
}

type openAICompatibleIdentity struct {
	Name              string   `json:"name"`
	BaseURL           string   `json:"base_url"`
	Models            []string `json:"models"`
	APIKeyFingerprint string   `json:"api_key_fingerprint"`
}

type openAICompatibleUsageFields struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Details      *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func NewOpenAICompatibleDriver(pauses ErrorPauses) *OpenAICompatibleDriver {
	return &OpenAICompatibleDriver{pauses: pauses}
}

func (d *OpenAICompatibleDriver) Provider() domain.Provider {
	return domain.ProviderOpenAICompatible
}

func (d *OpenAICompatibleDriver) Info() ProviderInfo {
	return ProviderInfo{
		Label:      "OpenAI-compatible",
		ProbeLabel: "openai-compatible",
	}
}

func (d *OpenAICompatibleDriver) Models() []Model { return nil }

func (d *OpenAICompatibleDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.Subject != "" {
		return string(domain.ProviderOpenAICompatible) + ":" + acct.Subject
	}
	return string(domain.ProviderOpenAICompatible) + ":" + acct.ID
}

func (d *OpenAICompatibleDriver) Plan(input *RelayInput) RelayPlan {
	if input == nil {
		return RelayPlan{}
	}
	stream, _ := input.Body["stream"].(bool)
	return RelayPlan{IsStream: stream}
}

func (d *OpenAICompatibleDriver) BuildRequest(ctx context.Context, input *RelayInput, acct *domain.Account, token string) (*http.Request, error) {
	identity := parseOpenAICompatibleIdentity(acct, nil)
	if identity.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible account missing base_url")
	}
	upstreamURL, err := openAICompatibleResponsesURL(identity.BaseURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, strings.NewReader(string(input.RawBody)))
	if err != nil {
		return nil, err
	}
	for _, h := range []string{"Content-Type", "Accept"} {
		if v := input.Headers.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}

func (d *OpenAICompatibleDriver) Interpret(statusCode int, headers http.Header, body []byte, _ string, _ json.RawMessage) Effect {
	errType, errMessage := parseOpenAICompatibleError(body)
	switch statusCode {
	case http.StatusOK:
		return Effect{Kind: EffectSuccess, Scope: EffectScopeAccount}
	case http.StatusUnauthorized:
		return Effect{
			Kind:                 EffectAuthFail,
			Scope:                EffectScopeAccount,
			CooldownUntil:        time.Now().Add(d.pauses.Pause401Refresh),
			UpstreamStatus:       statusCode,
			UpstreamErrorType:    errType,
			UpstreamErrorMessage: errMessage,
		}
	case http.StatusForbidden:
		return Effect{
			Kind:                 EffectBlock,
			Scope:                EffectScopeAccount,
			CooldownUntil:        time.Now().Add(d.pauses.Pause403),
			ErrorMessage:         fmt.Sprintf("upstream forbidden: %s", truncate(string(body), 200)),
			UpstreamStatus:       statusCode,
			UpstreamErrorType:    errType,
			UpstreamErrorMessage: errMessage,
		}
	case http.StatusTooManyRequests:
		until := time.Now().Add(d.pauses.Pause429)
		if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			until = time.Now().Add(retryAfter)
		}
		return Effect{
			Kind:                 EffectCooldown,
			Scope:                EffectScopeAccount,
			CooldownUntil:        until,
			UpstreamStatus:       statusCode,
			UpstreamErrorType:    errType,
			UpstreamErrorMessage: errMessage,
		}
	case http.StatusServiceUnavailable, 529:
		return Effect{
			Kind:                 EffectOverload,
			Scope:                EffectScopeAccount,
			CooldownUntil:        time.Now().Add(d.pauses.Pause529),
			UpstreamStatus:       statusCode,
			UpstreamErrorType:    errType,
			UpstreamErrorMessage: errMessage,
		}
	case http.StatusBadRequest:
		return Effect{
			Kind:                 EffectReject,
			Scope:                EffectScopeAccount,
			UpstreamStatus:       statusCode,
			UpstreamErrorType:    errType,
			UpstreamErrorMessage: errMessage,
		}
	}
	if statusCode >= 500 {
		return Effect{
			Kind:                 EffectServerError,
			Scope:                EffectScopeAccount,
			UpstreamStatus:       statusCode,
			UpstreamErrorType:    errType,
			UpstreamErrorMessage: errMessage,
		}
	}
	return Effect{
		Kind:                 EffectReject,
		Scope:                EffectScopeAccount,
		UpstreamStatus:       statusCode,
		UpstreamErrorType:    errType,
		UpstreamErrorMessage: errMessage,
	}
}

func (d *OpenAICompatibleDriver) StreamResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response) (bool, *Usage) {
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
	completed := true
	var captured *Usage
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
			if usage := parseOpenAICompatibleUsage(line[6:]); usage != nil {
				captured = usage
			}
		}
	}
	flusher.Flush()
	return completed, captured
}

func (d *OpenAICompatibleDriver) ForwardResponse(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		d.WriteError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (d *OpenAICompatibleDriver) ParseJSONUsage(body []byte) *Usage {
	return parseOpenAICompatibleUsage(string(body))
}

func (d *OpenAICompatibleDriver) ShouldRetry(statusCode int) bool {
	return statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == 529 ||
		statusCode >= 500
}

func (d *OpenAICompatibleDriver) RetrySameAccount(int, []byte, int) bool { return false }
func (d *OpenAICompatibleDriver) ParseNonRetriable(int, []byte) bool     { return false }

func (d *OpenAICompatibleDriver) WriteError(w http.ResponseWriter, status int, msg string) {
	writeDriverJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "error",
			"code":    status,
		},
	})
}

func (d *OpenAICompatibleDriver) WriteUpstreamError(w http.ResponseWriter, statusCode int, body []byte, _ bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

func (d *OpenAICompatibleDriver) InterceptRequest(http.ResponseWriter, map[string]interface{}, string) bool {
	return false
}

func (d *OpenAICompatibleDriver) Probe(ctx context.Context, acct *domain.Account, token string, client *http.Client) (ProbeResult, error) {
	identity := parseOpenAICompatibleIdentity(acct, nil)
	if len(identity.Models) == 0 {
		return ProbeResult{}, fmt.Errorf("openai-compatible account missing models")
	}
	body := fmt.Sprintf(`{"model":%q,"input":"ping","store":false}`, identity.Models[0])
	input := &RelayInput{
		RawBody: []byte(body),
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Model:   identity.Models[0],
	}
	req, err := d.BuildRequest(ctx, input, acct, token)
	if err != nil {
		return ProbeResult{}, err
	}
	resp, err := httpClientOrDefault(client, 30*time.Second).Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProbeResult{}, err
	}
	result := ProbeResult{
		Effect:  d.Interpret(resp.StatusCode, resp.Header, bodyBytes, identity.Models[0], json.RawMessage(acct.ProviderStateJSON)),
		Observe: true,
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	result.ClearCooldown = true
	return result, nil
}

func (d *OpenAICompatibleDriver) DescribeAccount(acct *domain.Account) []AccountField {
	identity := parseOpenAICompatibleIdentity(acct, nil)
	fields := make([]AccountField, 0, 3)
	if identity.BaseURL != "" {
		fields = append(fields, AccountField{Label: "base_url", Value: identity.BaseURL})
	}
	if len(identity.Models) > 0 {
		fields = append(fields, AccountField{Label: "models", Value: strings.Join(identity.Models, ", ")})
	}
	if identity.APIKeyFingerprint != "" {
		fields = append(fields, AccountField{Label: "key", Value: identity.APIKeyFingerprint})
	}
	return fields
}

func (d *OpenAICompatibleDriver) AutoPriority(json.RawMessage) int { return 50 }
func (d *OpenAICompatibleDriver) IsStale(json.RawMessage, time.Time) bool {
	return false
}
func (d *OpenAICompatibleDriver) ComputeExhaustedCooldown(json.RawMessage, time.Time) time.Time {
	return time.Time{}
}
func (d *OpenAICompatibleDriver) CanServe(acct *domain.Account, state json.RawMessage, model string, _ time.Time) bool {
	identity := parseOpenAICompatibleIdentity(acct, state)
	if model == "" {
		return len(identity.Models) > 0
	}
	for _, candidate := range identity.Models {
		if candidate == model {
			return true
		}
	}
	return false
}
func (d *OpenAICompatibleDriver) CalcCost(string, *Usage) float64 { return 0 }
func (d *OpenAICompatibleDriver) GetUtilization(json.RawMessage) []UtilWindow {
	return nil
}

func parseOpenAICompatibleIdentity(acct *domain.Account, state json.RawMessage) openAICompatibleIdentity {
	var identity openAICompatibleIdentity
	if acct != nil {
		if acct.Identity == nil {
			acct.HydrateRuntime()
		}
		if acct.Identity != nil {
			identity.Name = acct.Identity["name"]
			identity.BaseURL = strings.TrimSpace(acct.Identity["base_url"])
			identity.APIKeyFingerprint = acct.Identity["api_key_fingerprint"]
			identity.Models = parseOpenAICompatibleModelList(acct.Identity["models"])
		}
	}
	if len(identity.Models) == 0 && len(state) > 0 {
		var stateModels struct {
			Models []string `json:"models"`
		}
		if json.Unmarshal(state, &stateModels) == nil {
			identity.Models = stateModels.Models
		}
	}
	return identity
}

func parseOpenAICompatibleModelList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var models []string
	if json.Unmarshal([]byte(raw), &models) == nil {
		return compactStrings(models)
	}
	return compactStrings(strings.Split(raw, ","))
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func openAICompatibleResponsesURL(base string) (string, error) {
	base, err := NormalizeOpenAICompatibleBaseURL(base)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/responses"
	u.RawQuery = ""
	return u.String(), nil
}

func NormalizeOpenAICompatibleBaseURL(base string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("base_url must use http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("base_url must include host")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func parseOpenAICompatibleUsage(data string) *Usage {
	var direct struct {
		Usage *openAICompatibleUsageFields `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &direct) == nil {
		if usage := openAICompatibleUsageToUsage(direct.Usage); usage != nil {
			return usage
		}
	}
	var completed struct {
		Type     string `json:"type"`
		Response struct {
			Usage *openAICompatibleUsageFields `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(data), &completed) == nil {
		return openAICompatibleUsageToUsage(completed.Response.Usage)
	}
	return nil
}

func openAICompatibleUsageToUsage(u *openAICompatibleUsageFields) *Usage {
	if u == nil {
		return nil
	}
	usage := &Usage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
	}
	if u.Details != nil {
		usage.CacheReadTokens = u.Details.CachedTokens
	}
	return usage
}

func parseOpenAICompatibleError(body []byte) (string, string) {
	var envelope struct {
		Error struct {
			Type    string `json:"type"`
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return "", ""
	}
	errType := envelope.Error.Type
	if errType == "" && envelope.Error.Code != nil {
		errType = fmt.Sprint(envelope.Error.Code)
	}
	return errType, envelope.Error.Message
}
