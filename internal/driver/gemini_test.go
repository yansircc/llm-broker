package driver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func TestGeminiBuildRequestInjectsProjectID(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{APIURL: "https://cloudcode-pa.googleapis.com"})
	input := &RelayInput{
		RawBody:  []byte(`{"contents":[]}`),
		Headers:  make(http.Header),
		Path:     "/gemini/v1internal:streamGenerateContent",
		Model:    "gemini-2.5-flash",
		IsStream: true,
	}
	acct := &domain.Account{
		Provider:          domain.ProviderGemini,
		ProviderStateJSON: `{"project_id":"proj-123"}`,
	}

	req, err := d.BuildRequest(context.Background(), input, acct, "tok")
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if got, want := req.URL.String(), "https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("Accept"); got != "*/*" {
		t.Fatalf("Accept = %q", got)
	}
	wantUA := "GeminiCLI/0.32.1/gemini-2.5-flash (" + runtime.GOOS + "; " + runtime.GOARCH + ") google-api-nodejs-client/10.6.1"
	if got := req.Header.Get("User-Agent"); got != wantUA {
		t.Fatalf("User-Agent = %q, want %q", got, wantUA)
	}
	if got := req.Header.Get("X-Goog-Api-Client"); got != geminiAPIClientHeader {
		t.Fatalf("X-Goog-Api-Client = %q", got)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(body), `"project":"proj-123"`) {
		t.Fatalf("request body %s missing project injection", body)
	}
}

func TestGeminiBuildRequestRequiresProjectForCoreEndpoints(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{APIURL: "https://cloudcode-pa.googleapis.com"})
	input := &RelayInput{
		RawBody: []byte(`{"contents":[]}`),
		Headers: make(http.Header),
		Path:    "/gemini/v1internal:generateContent",
	}

	_, err := d.BuildRequest(context.Background(), input, &domain.Account{Provider: domain.ProviderGemini}, "tok")
	if err == nil {
		t.Fatal("BuildRequest() error = nil, want missing project_id")
	}
}

func TestGeminiPlanDetectsStreamPath(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{})
	if !d.Plan(&RelayInput{
		Body: map[string]interface{}{},
		Path: "/gemini/v1internal:streamGenerateContent",
	}).IsStream {
		t.Fatal("Plan().IsStream = false, want true for stream path")
	}
}

func TestParseGeminiRetryDelayPrefersRetryInfo(t *testing.T) {
	body := []byte(`[
		{
			"error": {
				"details": [
					{
						"@type": "type.googleapis.com/google.rpc.ErrorInfo",
						"quotaResetDelay": "3s"
					},
					{
						"@type": "type.googleapis.com/google.rpc.RetryInfo",
						"retryDelay": "1500ms"
					}
				]
			}
		}
	]`)

	got := parseGeminiRetryDelay(body)
	if got != 1500*time.Millisecond {
		t.Fatalf("parseGeminiRetryDelay() = %v, want 1500ms", got)
	}
}

func TestRetrieveGeminiUserQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1internal:retrieveUserQuota" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
			"buckets": [
				{
					"resetTime": "2026-03-11T14:50:31Z",
					"tokenType": "REQUESTS",
					"modelId": "gemini-2.5-flash",
					"remainingFraction": 0.75
				},
				{
					"resetTime": "2026-03-11T14:50:31Z",
					"tokenType": "REQUESTS",
					"modelId": "gemini-2.5-pro",
					"remainingFraction": 0.5
				}
			]
		}`)
	}))
	defer srv.Close()

	info, err := retrieveGeminiUserQuota(context.Background(), srv.Client(), srv.URL, "tok")
	if err != nil {
		t.Fatalf("retrieveGeminiUserQuota() error = %v", err)
	}
	if info.DailyRequestsRemainingFraction != 0.5 {
		t.Fatalf("remaining = %v, want 0.5", info.DailyRequestsRemainingFraction)
	}
	if info.DailyRequestsResetAt != 1773240631 {
		t.Fatalf("reset = %d, want 1773240631", info.DailyRequestsResetAt)
	}
}

func TestGeminiInterpret429UsesBodyRetryDelay(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{
		Pauses: ErrorPauses{Pause429: 60 * time.Second},
	})
	body := []byte(`[
		{
			"error": {
				"details": [
					{
						"@type": "type.googleapis.com/google.rpc.RetryInfo",
						"retryDelay": "1500ms"
					}
				]
			}
		}
	]`)

	before := time.Now()
	effect := d.Interpret(429, make(http.Header), body, "", json.RawMessage(`{"project_id":"proj-123"}`))
	after := time.Now()

	if effect.Kind != EffectCooldown {
		t.Fatalf("Kind = %v, want cooldown", effect.Kind)
	}
	if effect.Scope != EffectScopeBucket {
		t.Fatalf("Scope = %v, want bucket", effect.Scope)
	}
	minUntil := before.Add(1400 * time.Millisecond)
	maxUntil := after.Add(2 * time.Second)
	if effect.CooldownUntil.Before(minUntil) || effect.CooldownUntil.After(maxUntil) {
		t.Fatalf("CooldownUntil = %v, want about 1.5s from now", effect.CooldownUntil)
	}
}

func TestGeminiQuotaStateProjection(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{})
	state := json.RawMessage(`{
		"project_id":"proj-123",
		"daily_requests_remaining_fraction":0.62,
		"daily_requests_reset_at":1773240631,
		"quota_updated_at":1773154231
	}`)

	if got := d.AutoPriority(state); got != 62 {
		t.Fatalf("AutoPriority() = %d, want 62", got)
	}
	if stale := d.IsStale(state, time.Unix(1773154231+int64(geminiQuotaRefreshInterval/time.Second)-1, 0)); stale {
		t.Fatal("IsStale() = true before refresh interval")
	}
	if stale := d.IsStale(state, time.Unix(1773154231+int64(geminiQuotaRefreshInterval/time.Second), 0)); !stale {
		t.Fatal("IsStale() = false at refresh interval")
	}

	windows := d.GetUtilization(state)
	if len(windows) != 1 {
		t.Fatalf("GetUtilization() len = %d, want 1", len(windows))
	}
	if windows[0].Label != "daily" || windows[0].Pct != 38 || windows[0].Reset != 1773240631 {
		t.Fatalf("window = %+v", windows[0])
	}
}

func TestGeminiParseJSONUsageHandlesStreamWrapper(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{})
	usage := d.ParseJSONUsage([]byte(`{
		"response": {
			"usageMetadata": {
				"promptTokenCount": 5,
				"candidatesTokenCount": 2,
				"cachedContentTokenCount": 7
			}
		}
	}`))
	if usage == nil {
		t.Fatal("ParseJSONUsage() = nil")
	}
	if usage.InputTokens != 5 || usage.OutputTokens != 2 || usage.CacheReadTokens != 7 {
		t.Fatalf("usage = %+v, want input=5 output=2 cache=7", usage)
	}
}

func TestGeminiStreamResponseCapturesUsage(t *testing.T) {
	d := NewGeminiDriver(GeminiConfig{})
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			"data: {\"response\":{\"usageMetadata\":{\"promptTokenCount\":5,\"candidatesTokenCount\":2,\"cachedContentTokenCount\":7}}}\n\n",
		)),
	}
	w := httptest.NewRecorder()

	completed, usage := d.StreamResponse(context.Background(), w, resp)
	if !completed {
		t.Fatal("StreamResponse() completed = false, want true")
	}
	if usage == nil {
		t.Fatal("StreamResponse() usage = nil")
	}
	if usage.InputTokens != 5 || usage.OutputTokens != 2 || usage.CacheReadTokens != 7 {
		t.Fatalf("usage = %+v, want input=5 output=2 cache=7", usage)
	}
	if got := w.Body.String(); !strings.Contains(got, "usageMetadata") {
		t.Fatalf("stream output %q missing forwarded SSE payload", got)
	}
}

func TestParseGeminiIDToken(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"google-sub","email":"user@example.com","name":"User"}`))
	token := "header." + payload + ".sig"

	info := parseGeminiIDToken(token)
	if info == nil {
		t.Fatal("parseGeminiIDToken() = nil")
	}
	if info.Subject != "google-sub" {
		t.Fatalf("Subject = %q", info.Subject)
	}
	if info.Email != "user@example.com" {
		t.Fatalf("Email = %q", info.Email)
	}
}
