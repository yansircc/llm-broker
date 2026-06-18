package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/relay"
	"github.com/yansircc/llm-broker/internal/store"
)

func TestResolveCompatModelCodexOnly(t *testing.T) {
	for _, raw := range []string{"gpt-5", "openai/gpt-5", "codex/gpt-5"} {
		provider, model, requested, err := resolveCompatModel(raw)
		if err != nil {
			t.Fatalf("resolveCompatModel(%q): %v", raw, err)
		}
		if provider != domain.ProviderCodex || model != "gpt-5" || requested != "gpt-5" {
			t.Fatalf("resolveCompatModel(%q) = %s/%s/%s", raw, provider, model, requested)
		}
	}
	for _, raw := range []string{"claude/claude-sonnet-4-6", "gemini/gemini-2.5-flash"} {
		if _, _, _, err := resolveCompatModel(raw); err == nil {
			t.Fatalf("resolveCompatModel(%q) succeeded, want rejection", raw)
		}
	}
}

func TestCompatOpenAIChatToCodexResponsesRequest(t *testing.T) {
	maxTokens := 123
	req := &compatOpenAIChatRequest{
		Model: "openai/gpt-5",
		Messages: []compatMessage{
			{Role: "system", Content: json.RawMessage(`"be terse"`)},
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		MaxCompletionTokens: &maxTokens,
	}
	got, err := compatOpenAIChatToCodexResponsesRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got["model"] != "gpt-5" || got["max_output_tokens"] != maxTokens {
		t.Fatalf("unexpected request: %#v", got)
	}
	input := got["input"].([]map[string]string)
	if len(input) != 2 || input[0]["role"] != "system" || input[1]["content"] != "hello" {
		t.Fatalf("input = %#v", input)
	}
}

func TestCompatCodexResponsesToOpenAIChatResponse(t *testing.T) {
	body := []byte(`{
		"id":"resp_1",
		"model":"gpt-5",
		"output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],
		"usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13}
	}`)
	got, err := compatCodexResponsesToOpenAIChatResponse(body, "gpt-5")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "resp_1" || got.Choices[0].Message.Content != "hello" {
		t.Fatalf("unexpected response: %#v", got)
	}
	if got.Usage == nil || got.Usage.TotalTokens != 13 {
		t.Fatalf("usage = %#v", got.Usage)
	}
}

func TestHandleCompatListModelsCodexOnly(t *testing.T) {
	srv := &Server{
		catalogDrivers: map[domain.Provider]driver.Descriptor{
			domain.ProviderCodex: driver.NewCodexDriver(driver.CodexConfig{}),
		},
	}
	w := httptest.NewRecorder()
	srv.handleCompatListModels(w, httptest.NewRequest(http.MethodGet, "/compat/v1/models", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "claude/") || strings.Contains(w.Body.String(), "gemini/") {
		t.Fatalf("legacy provider leaked in compat models: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "codex/") {
		t.Fatalf("codex model prefix missing: %s", w.Body.String())
	}
}

func TestHandleCompatOpenAIChatCompletionsCodexLoop(t *testing.T) {
	srv := newCodexCompatTestServer(t, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://codex.example/responses" {
			t.Fatalf("upstream URL = %s", req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"input"`) || strings.Contains(string(body), "claude") {
			t.Fatalf("unexpected upstream body: %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1","model":"gpt-5","output_text":"hello","usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13}}`)),
		}, nil
	})})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-5",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), auth.KeyInfoKey, &auth.KeyInfo{
		ID:             "user-1",
		CustomerID:     "user-1",
		APIKeyID:       "key-1",
		Name:           "test",
		AllowedSurface: domain.SurfaceAll,
	}))
	w := httptest.NewRecorder()
	srv.handleCompatOpenAIChatCompletions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"object":"chat.completion"`) || !strings.Contains(w.Body.String(), `"hello"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func newCodexCompatTestServer(t *testing.T, upstreamClient *http.Client) *Server {
	t.Helper()
	ms := store.NewMockStore()
	bus := events.NewBus(16)
	acct := &domain.Account{
		ID:        "acct-codex-1",
		Email:     "codex@example.com",
		Provider:  domain.ProviderCodex,
		Status:    domain.StatusActive,
		Priority:  1,
		Subject:   "subject-codex-1",
		BucketKey: "codex:subject-codex-1",
		CellID:    "cell-codex-compat-1",
		CreatedAt: time.Now().UTC(),
	}
	if err := ms.SaveAccount(context.Background(), acct); err != nil {
		t.Fatal(err)
	}
	if err := ms.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: acct.BucketKey,
		Provider:  acct.Provider,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.SaveEgressCell(context.Background(), &domain.EgressCell{
		ID:        "cell-codex-compat-1",
		Name:      "Codex Compat 01",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11081},
		Labels:    map[string]string{"lane": "compat"},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatal(err)
	}
	codexDrv := driver.NewCodexDriver(driver.CodexConfig{APIURL: "https://codex.example/responses"})
	srv := &Server{
		cfg:           &config.Config{MaxRequestBodyMB: 60},
		store:         ms,
		pool:          p,
		bus:           bus,
		compatLimiter: newCompatRateLimiter(0, 0),
		relay: relay.New(
			p,
			staticTokenProvider{},
			ms,
			relay.Config{MaxRequestBodyMB: 60, MaxRetryAccounts: 0},
			staticTransportProvider{client: upstreamClient},
			bus,
			map[domain.Provider]driver.ExecutionDriver{domain.ProviderCodex: codexDrv},
		),
		catalogDrivers: map[domain.Provider]driver.Descriptor{domain.ProviderCodex: codexDrv},
	}
	t.Cleanup(srv.relay.WaitForLogFlush)
	return srv
}

type staticTokenProvider struct{}

func (staticTokenProvider) EnsureValidToken(context.Context, string) (string, error) {
	return "test-token", nil
}

type staticTransportProvider struct {
	client *http.Client
}

func (p staticTransportProvider) ClientForAccount(*domain.Account) *http.Client {
	return p.client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
