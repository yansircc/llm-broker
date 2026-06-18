package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

type captureResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *captureResponseWriter) Header() http.Header {
	return w.header
}

func (w *captureResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *captureResponseWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *captureResponseWriter) Flush() {}

func testOpenAICompatibleAccount(t *testing.T) *domain.Account {
	t.Helper()
	identity := map[string]string{
		"name":                "fallback-a",
		"base_url":            "https://third.example/v1/",
		"models":              `["gpt-5.5","gpt-5"]`,
		"api_key_fingerprint": "fp-123",
	}
	data, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("marshal identity: %v", err)
	}
	acct := &domain.Account{
		ID:           "acct-third-1",
		Provider:     domain.ProviderOpenAICompatible,
		Subject:      "third-subject",
		IdentityJSON: string(data),
	}
	acct.HydrateRuntime()
	return acct
}

func TestOpenAICompatibleBuildRequestUsesStaticBaseURLAndBearerKey(t *testing.T) {
	d := NewOpenAICompatibleDriver(ErrorPauses{})
	input := &RelayInput{
		RawBody: []byte(`{"model":"gpt-5.5","input":"hi"}`),
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
			"Accept":       []string{"text/event-stream"},
		},
		Model: "gpt-5.5",
	}

	req, err := d.BuildRequest(context.Background(), input, testOpenAICompatibleAccount(t), "sk-third-party")
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if got := req.URL.String(); got != "https://third.example/v1/responses" {
		t.Fatalf("upstream URL = %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer sk-third-party" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("Chatgpt-Account-Id"); got != "" {
		t.Fatalf("Chatgpt-Account-Id leaked into third-party request: %q", got)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(input.RawBody) {
		t.Fatalf("body = %s, want %s", body, input.RawBody)
	}
}

func TestNormalizeOpenAICompatibleBaseURLDropsQueryAndFragment(t *testing.T) {
	got, err := NormalizeOpenAICompatibleBaseURL(" https://third.example/v1/?debug=1#frag ")
	if err != nil {
		t.Fatalf("NormalizeOpenAICompatibleBaseURL() error = %v", err)
	}
	if got != "https://third.example/v1" {
		t.Fatalf("NormalizeOpenAICompatibleBaseURL() = %q", got)
	}
}

func TestOpenAICompatibleCanServeConfiguredModelsOnly(t *testing.T) {
	d := NewOpenAICompatibleDriver(ErrorPauses{})
	state := json.RawMessage(`{"models":["gpt-5.5","gpt-5"]}`)
	now := time.Now()

	if !d.CanServe(nil, state, "gpt-5.5", now) {
		t.Fatal("CanServe(gpt-5.5) = false, want true")
	}
	if d.CanServe(nil, state, "gpt-4.1", now) {
		t.Fatal("CanServe(gpt-4.1) = true, want false")
	}
}

func TestOpenAICompatibleInterpretClassifiesCommonStatuses(t *testing.T) {
	d := NewOpenAICompatibleDriver(ErrorPauses{
		Pause401Refresh: time.Second,
		Pause429:        2 * time.Second,
		Pause529:        3 * time.Second,
	})

	tests := []struct {
		status int
		want   EffectKind
	}{
		{http.StatusOK, EffectSuccess},
		{http.StatusUnauthorized, EffectAuthFail},
		{http.StatusForbidden, EffectBlock},
		{http.StatusTooManyRequests, EffectCooldown},
		{http.StatusInternalServerError, EffectServerError},
		{http.StatusServiceUnavailable, EffectOverload},
		{http.StatusBadRequest, EffectReject},
	}
	for _, tt := range tests {
		effect := d.Interpret(tt.status, nil, []byte(`{"error":{"type":"x","message":"bad"}}`), "gpt-5.5", nil)
		if effect.Kind != tt.want {
			t.Fatalf("Interpret(%d).Kind = %v, want %v", tt.status, effect.Kind, tt.want)
		}
	}
}

func TestOpenAICompatibleParseJSONUsage(t *testing.T) {
	d := NewOpenAICompatibleDriver(ErrorPauses{})
	usage := d.ParseJSONUsage([]byte(`{"usage":{"input_tokens":12,"output_tokens":7,"input_tokens_details":{"cached_tokens":3}}}`))
	if usage == nil {
		t.Fatal("ParseJSONUsage() = nil")
	}
	if usage.InputTokens != 12 || usage.OutputTokens != 7 || usage.CacheReadTokens != 3 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestOpenAICompatibleStreamResponseCapturesCompletedUsage(t *testing.T) {
	d := NewOpenAICompatibleDriver(ErrorPauses{})
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":4,\"output_tokens\":2}}}\n\n",
		)),
	}
	w := &captureResponseWriter{header: make(http.Header)}

	completed, usage := d.StreamResponse(context.Background(), w, resp)
	if !completed {
		t.Fatal("completed = false, want true")
	}
	if usage == nil || usage.InputTokens != 4 || usage.OutputTokens != 2 {
		t.Fatalf("usage = %+v", usage)
	}
	if !strings.Contains(w.body.String(), "response.output_text.delta") {
		t.Fatalf("stream body not forwarded: %q", w.body.String())
	}
}
