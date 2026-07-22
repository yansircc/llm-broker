package relay

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/store"
)

type relayTestDriver struct {
	provider            domain.Provider
	interpretCalls      []int
	interpretBodies     [][]byte
	interpretFn         func(statusCode int, body []byte) driver.Effect
	buildRequestErr     error
	buildRequestBody    string
	buildRequestURL     string
	buildRequestHeaders http.Header
}

func (d *relayTestDriver) Provider() domain.Provider { return d.provider }

func (d *relayTestDriver) Plan(_ *driver.RelayInput) driver.RelayPlan { return driver.RelayPlan{} }

func (d *relayTestDriver) BuildRequest(ctx context.Context, _ *driver.RelayInput, _ *domain.Account, _ string) (*http.Request, error) {
	if d.buildRequestErr != nil {
		return nil, d.buildRequestErr
	}
	url := d.buildRequestURL
	if url == "" {
		url = "https://upstream.test/v1/messages"
	}
	body := d.buildRequestBody
	if body == "" {
		body = `{}`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, vals := range d.buildRequestHeaders {
		for _, val := range vals {
			req.Header.Add(key, val)
		}
	}
	return req, nil
}

func (d *relayTestDriver) Interpret(statusCode int, _ http.Header, body []byte, _ string, _ json.RawMessage) driver.Effect {
	d.interpretCalls = append(d.interpretCalls, statusCode)
	d.interpretBodies = append(d.interpretBodies, append([]byte(nil), body...))
	if d.interpretFn != nil {
		return d.interpretFn(statusCode, body)
	}
	if statusCode == http.StatusOK {
		return driver.Effect{Kind: driver.EffectSuccess, Scope: driver.EffectScopeBucket}
	}
	return driver.Effect{
		Kind:          driver.EffectCooldown,
		Scope:         driver.EffectScopeBucket,
		CooldownUntil: time.Now().Add(5 * time.Minute),
	}
}

func (d *relayTestDriver) StreamResponse(_ context.Context, _ http.ResponseWriter, _ *http.Response) (bool, *driver.Usage) {
	return false, nil
}

func (d *relayTestDriver) ForwardResponse(_ http.ResponseWriter, _ *http.Response) {}

func (d *relayTestDriver) ParseJSONUsage(_ []byte) *driver.Usage { return nil }

func (d *relayTestDriver) ShouldRetry(statusCode int) bool { return statusCode == 529 }

func (d *relayTestDriver) RetrySameAccount(_ int, _ []byte, _ int) bool { return false }

func (d *relayTestDriver) ParseNonRetriable(_ int, _ []byte) bool { return false }

func (d *relayTestDriver) WriteError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func (d *relayTestDriver) WriteUpstreamError(w http.ResponseWriter, statusCode int, body []byte, _ bool) {
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func (d *relayTestDriver) InterceptRequest(_ http.ResponseWriter, _ map[string]interface{}, _ string) bool {
	return false
}

func (d *relayTestDriver) CalcCost(_ string, _ *driver.Usage) float64 { return 0 }

func (d *relayTestDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.BucketKey != "" {
		return acct.BucketKey
	}
	if acct.Subject != "" {
		return string(d.provider) + ":" + acct.Subject
	}
	return string(d.provider) + ":" + acct.ID
}

func (d *relayTestDriver) AutoPriority(_ json.RawMessage) int { return 50 }

func (d *relayTestDriver) IsStale(_ json.RawMessage, _ time.Time) bool { return false }

func (d *relayTestDriver) ComputeExhaustedCooldown(_ json.RawMessage, _ time.Time) time.Time {
	return time.Time{}
}

func (d *relayTestDriver) CanServe(_ json.RawMessage, _ string, _ time.Time) bool { return true }

func (d *relayTestDriver) AssessCapacity(_ json.RawMessage, _ string, _ time.Time) driver.CapacityAssessment {
	return driver.CapacityAssessment{Eligible: true, Priority: 50}
}

type relayTestTokenProvider struct{}

func (relayTestTokenProvider) EnsureValidToken(_ context.Context, _ string) (string, error) {
	return "tok", nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestRelayRetriesCodexHTTP200CapacityBeforeDownstreamCommit(t *testing.T) {
	mockStore := store.NewMockStore()
	for _, acct := range []*domain.Account{
		{
			ID:        "acct-a",
			Email:     "a@example.com",
			Provider:  domain.ProviderCodex,
			Status:    domain.StatusActive,
			Subject:   "subject-a",
			BucketKey: "codex:subject-a",
			CreatedAt: time.Now().UTC(),
		},
		{
			ID:        "acct-b",
			Email:     "b@example.com",
			Provider:  domain.ProviderCodex,
			Status:    domain.StatusActive,
			Subject:   "subject-b",
			BucketKey: "codex:subject-b",
			CreatedAt: time.Now().UTC(),
		},
	} {
		saveRelayTestAccount(t, mockStore, acct)
	}

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}
	codexDriver := driver.NewCodexDriver(driver.CodexConfig{
		APIURL: "https://upstream.test/openai/responses",
		Pauses: driver.ErrorPauses{Pause429: time.Hour},
	})
	capacityStream := "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"message\":\"Selected model is at capacity. Please try a different model.\"}}\n\n"
	healthyStream := "event: response.created\ndata: {\"type\":\"response.created\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"
	upstreamCalls := 0
	transport := relayTestTransport{client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		upstreamCalls++
		body := capacityStream
		if upstreamCalls == 2 {
			body = healthyStream
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderCodex: codexDriver},
	)
	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "heavy-user", Name: "heavy-user"},
		input: &driver.RelayInput{
			Body:     map[string]interface{}{"model": "gpt-5.5", "stream": true},
			RawBody:  []byte(`{"model":"gpt-5.5","stream":true}`),
			Headers:  make(http.Header),
			Path:     "/openai/responses",
			Model:    "gpt-5.5",
			IsStream: true,
		},
		surface:            domain.SurfaceNative,
		affinityKey:        "codex-session",
		affinityContinuity: driver.AffinityPrefer,
	}
	state := newRelayAttemptState()
	recorder := httptest.NewRecorder()

	if outcome := relaySvc.executeRelayAttempt(context.Background(), recorder, codexDriver, prepared, state, 0); outcome != relayAttemptContinue {
		t.Fatalf("first outcome = %v, want continue", outcome)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("capacity attempt committed downstream bytes: %q", recorder.Body.String())
	}
	if outcome := relaySvc.executeRelayAttempt(context.Background(), recorder, codexDriver, prepared, state, 1); outcome != relayAttemptDone {
		t.Fatalf("second outcome = %v, want done", outcome)
	}
	if upstreamCalls != 2 {
		t.Fatalf("upstream calls = %d, want 2", upstreamCalls)
	}
	if recorder.Body.String() != healthyStream {
		t.Fatalf("downstream body = %q, want only healthy stream %q", recorder.Body.String(), healthyStream)
	}
	if acct := p.Get("acct-a"); acct == nil || acct.CooldownUntil == nil {
		t.Fatalf("first bucket was not cooled down: %#v", acct)
	}
	boundID, ok, err := p.GetSessionBinding(context.Background(), "codex-session")
	if err != nil {
		t.Fatalf("GetSessionBinding: %v", err)
	}
	if !ok || boundID != "acct-b" {
		t.Fatalf("session binding = (%q, %v), want acct-b", boundID, ok)
	}
}

type relayTestTransport struct {
	client *http.Client
}

func (t relayTestTransport) ClientForAccount(_ *domain.Account) *http.Client { return t.client }

type capturedRecord struct {
	msg   string
	attrs map[string]any
}

type captureHandler struct {
	mu      sync.Mutex
	records []capturedRecord
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	captured := capturedRecord{
		msg:   record.Message,
		attrs: make(map[string]any),
	}
	record.Attrs(func(attr slog.Attr) bool {
		captured.attrs[attr.Key] = valueAny(attr.Value)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, captured)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

func (h *captureHandler) find(msg string) *capturedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.records {
		if h.records[i].msg == msg {
			record := h.records[i]
			return &record
		}
	}
	return nil
}

func valueAny(v slog.Value) any {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindDuration:
		return v.Duration()
	case slog.KindTime:
		return v.Time()
	default:
		return v.Any()
	}
}

func waitRequestLogsCount(t *testing.T, mockStore *store.MockStore, want int) []*domain.RequestLog {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, total, err := mockStore.QueryRequestLogs(context.Background(), domain.RequestLogQuery{})
		if err != nil {
			t.Fatalf("QueryRequestLogs: %v", err)
		}
		if total == want {
			return logs
		}
		time.Sleep(10 * time.Millisecond)
	}

	logs, _, err := mockStore.QueryRequestLogs(context.Background(), domain.RequestLogQuery{})
	if err != nil {
		t.Fatalf("QueryRequestLogs: %v", err)
	}
	t.Fatalf("len(logs) = %d, want %d", len(logs), want)
	return nil
}

func saveRelayTestAccount(t *testing.T, mockStore *store.MockStore, acct *domain.Account) {
	t.Helper()
	if err := mockStore.SaveAccount(context.Background(), acct); err != nil {
		t.Fatalf("SaveAccount(%s): %v", acct.ID, err)
	}
	if err := mockStore.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: acct.BucketKey,
		Provider:  acct.Provider,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket(%s): %v", acct.BucketKey, err)
	}
}

func TestExecuteRelayAttemptLogsRetriableFailure(t *testing.T) {
	mockStore := store.NewMockStore()
	account := &domain.Account{
		ID:        "acct-1",
		Email:     "acct@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-1",
		CreatedAt: time.Now().UTC(),
	}
	if err := mockStore.SaveAccount(context.Background(), account); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	if err := mockStore.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: "claude:subject-1",
		Provider:  domain.ProviderClaude,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket: %v", err)
	}

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{
		provider:         domain.ProviderClaude,
		buildRequestBody: `{"messages":[{"role":"user","content":"hello upstream"}],"stream":false}`,
		buildRequestHeaders: http.Header{
			"Content-Type":     []string{"application/json"},
			"X-Stainless-Test": []string{"1"},
			"Authorization":    []string{"Bearer secret"},
		},
	}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 529,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
					)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	capture := &captureHandler{}
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(oldLogger)

	headers := make(http.Header)
	headers.Set("X-Stainless-Retry-Count", "1")
	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "user-1", Name: "leo"},
		input: &driver.RelayInput{
			Headers: headers,
			RawBody: []byte(`{"messages":[{"role":"user","content":"hello client"}]}`),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		affinityKey: "session-123",
	}

	outcome := relaySvc.executeRelayAttempt(
		context.Background(),
		httptest.NewRecorder(),
		driverStub,
		prepared,
		newRelayAttemptState(),
		0,
	)
	if outcome != relayAttemptContinue {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptContinue)
	}

	var logs []*domain.RequestLog
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var total int
		logs, total, err = mockStore.QueryRequestLogs(context.Background(), domain.RequestLogQuery{})
		if err != nil {
			t.Fatalf("QueryRequestLogs: %v", err)
		}
		if total == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(logs) != 1 {
		t.Fatalf("len(logs) = %d, want 1", len(logs))
	}

	if logs[0].UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", logs[0].UserID)
	}
	if logs[0].AccountID != account.ID {
		t.Fatalf("AccountID = %q, want %q", logs[0].AccountID, account.ID)
	}
	if logs[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("Model = %q", logs[0].Model)
	}
	if logs[0].Status != "upstream_529" {
		t.Fatalf("Status = %q, want upstream_529", logs[0].Status)
	}
	if logs[0].UpstreamStatus != 529 {
		t.Fatalf("UpstreamStatus = %d, want 529", logs[0].UpstreamStatus)
	}

	record := capture.find("retriable upstream error")
	if record == nil {
		t.Fatal("missing retriable upstream error log record")
	}
	if record.attrs["userId"] != "user-1" {
		t.Fatalf("log userId = %#v, want user-1", record.attrs["userId"])
	}
	if record.attrs["userName"] != "leo" {
		t.Fatalf("log userName = %#v, want leo", record.attrs["userName"])
	}
	if record.attrs["path"] != "/v1/messages" {
		t.Fatalf("log path = %#v, want /v1/messages", record.attrs["path"])
	}
	if record.attrs["sessionUUID"] != "session-123" {
		t.Fatalf("log sessionUUID = %#v, want session-123", record.attrs["sessionUUID"])
	}
	if record.attrs["clientRetryCount"] != "1" {
		t.Fatalf("log clientRetryCount = %#v, want 1", record.attrs["clientRetryCount"])
	}
}

func TestExecuteRelayAttemptPassesRealStatusToInterpretOnNonRetriableError(t *testing.T) {
	mockStore := store.NewMockStore()
	account := &domain.Account{
		ID:        "acct-500",
		Email:     "acct500@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-500",
		CreatedAt: time.Now().UTC(),
	}
	if err := mockStore.SaveAccount(context.Background(), account); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	if err := mockStore.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: "claude:subject-500",
		Provider:  domain.ProviderClaude,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket: %v", err)
	}

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{provider: domain.ProviderClaude}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":"boom"}`)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "user-500", Name: "leo"},
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
	}
	recorder := httptest.NewRecorder()

	outcome := relaySvc.executeRelayAttempt(
		context.Background(),
		recorder,
		driverStub,
		prepared,
		newRelayAttemptState(),
		0,
	)
	if outcome != relayAttemptDone {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}

	if len(driverStub.interpretCalls) != 1 {
		t.Fatalf("Interpret call count = %d, want 1", len(driverStub.interpretCalls))
	}
	if driverStub.interpretCalls[0] != http.StatusInternalServerError {
		t.Fatalf("Interpret status = %d, want %d", driverStub.interpretCalls[0], http.StatusInternalServerError)
	}

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("response status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	bucket := p.List()[0]
	if bucket.CooldownUntil == nil {
		t.Fatal("expected cooldown after non-retriable 500 interpretation")
	}
}

func TestExecuteRelayAttemptPassesBodyToInterpretOnNonRetriable400(t *testing.T) {
	mockStore := store.NewMockStore()
	account := &domain.Account{
		ID:        "acct-400",
		Email:     "acct400@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-400",
		CreatedAt: time.Now().UTC(),
	}
	if err := mockStore.SaveAccount(context.Background(), account); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	if err := mockStore.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: "claude:subject-400",
		Provider:  domain.ProviderClaude,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket: %v", err)
	}

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{
		provider: domain.ProviderClaude,
		interpretFn: func(statusCode int, body []byte) driver.Effect {
			if statusCode == http.StatusBadRequest && strings.Contains(string(body), "organization has been disabled") {
				return driver.Effect{
					Kind:          driver.EffectBlock,
					Scope:         driver.EffectScopeBucket,
					CooldownUntil: time.Now().Add(time.Minute),
					ErrorMessage:  "disabled org",
				}
			}
			return driver.Effect{Kind: driver.EffectSuccess, Scope: driver.EffectScopeBucket}
		},
	}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"type":"error","error":{"type":"invalid_request_error","message":"This organization has been disabled."}}`,
					)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "user-400", Name: "leo"},
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
	}
	recorder := httptest.NewRecorder()

	outcome := relaySvc.executeRelayAttempt(
		context.Background(),
		recorder,
		driverStub,
		prepared,
		newRelayAttemptState(),
		0,
	)
	if outcome != relayAttemptDone {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}

	if len(driverStub.interpretBodies) != 1 {
		t.Fatalf("Interpret body count = %d, want 1", len(driverStub.interpretBodies))
	}
	if !strings.Contains(string(driverStub.interpretBodies[0]), "organization has been disabled") {
		t.Fatalf("Interpret body = %q, want disabled signal", string(driverStub.interpretBodies[0]))
	}

	acct := p.Get(account.ID)
	if acct == nil {
		t.Fatal("expected account to remain readable")
	}
	if acct.Status != domain.StatusBlocked {
		t.Fatalf("account status = %s, want blocked", acct.Status)
	}
	if acct.ErrorMessage != "disabled org" {
		t.Fatalf("account error = %q, want disabled org", acct.ErrorMessage)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("response status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "organization has been disabled") {
		t.Fatalf("response body = %q, want upstream body", recorder.Body.String())
	}
}

func TestExecuteRelayAttemptReturnsDriverValidationError(t *testing.T) {
	mockStore := store.NewMockStore()
	account := &domain.Account{
		ID:        "acct-invalid-model",
		Email:     "invalid@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-invalid-model",
		CreatedAt: time.Now().UTC(),
	}
	if err := mockStore.SaveAccount(context.Background(), account); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	if err := mockStore.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: "claude:subject-invalid-model",
		Provider:  domain.ProviderClaude,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket: %v", err)
	}

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{
		provider:        domain.ProviderClaude,
		buildRequestErr: driver.NewRequestValidationError(http.StatusBadRequest, `model "gpt-5.4" does not belong to Claude; use the OpenAI/Codex relay instead`),
	}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				t.Fatal("upstream should not be called when driver rejects request locally")
				return nil, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "user-invalid-model", Name: "mike"},
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "gpt-5.4",
		},
	}
	recorder := httptest.NewRecorder()

	outcome := relaySvc.executeRelayAttempt(
		context.Background(),
		recorder,
		driverStub,
		prepared,
		newRelayAttemptState(),
		0,
	)
	if outcome != relayAttemptDone {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("response status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "does not belong to Claude") {
		t.Fatalf("response body = %q, want local validation message", recorder.Body.String())
	}
	if len(driverStub.interpretCalls) != 0 {
		t.Fatalf("Interpret call count = %d, want 0", len(driverStub.interpretCalls))
	}
	logs := waitRequestLogsCount(t, mockStore, 1)
	if logs[0].Status != "validation_400" {
		t.Fatalf("request log status = %q, want validation_400", logs[0].Status)
	}
	if logs[0].EffectKind != "reject" {
		t.Fatalf("request log effect kind = %q, want reject", logs[0].EffectKind)
	}
	if logs[0].UpstreamStatus != http.StatusBadRequest {
		t.Fatalf("request log upstream status = %d, want 400", logs[0].UpstreamStatus)
	}
	if logs[0].UpstreamErrorType != "request_validation_error" {
		t.Fatalf("request log upstream error type = %q, want request_validation_error", logs[0].UpstreamErrorType)
	}
}

func TestRelayStoresAndReusesSessionAffinity(t *testing.T) {
	mockStore := store.NewMockStore()
	accountA := &domain.Account{
		ID:        "acct-stick-a",
		Email:     "stick-a@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-stick-a",
		BucketKey: "claude:subject-stick-a",
		CreatedAt: time.Now().UTC(),
	}
	saveRelayTestAccount(t, mockStore, accountA)

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{provider: domain.ProviderClaude}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id":"msg_1","type":"message","usage":{"input_tokens":1,"output_tokens":1}}`)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	keyInfo := &auth.KeyInfo{ID: "user-stick", Name: "mike"}
	prepared1 := &preparedRelayRequest{
		keyInfo: keyInfo,
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		surface:            domain.SurfaceNative,
		affinityKey:        "affinity-stick",
		affinityContinuity: driver.AffinityPrefer,
	}
	if outcome := relaySvc.executeRelayAttempt(context.Background(), httptest.NewRecorder(), driverStub, prepared1, newRelayAttemptState(), 0); outcome != relayAttemptDone {
		t.Fatalf("first executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}

	accountID, ok, err := p.GetSessionBinding(context.Background(), "affinity-stick")
	if err != nil {
		t.Fatalf("GetSessionBinding: %v", err)
	}
	if !ok || accountID != accountA.ID {
		t.Fatalf("affinity binding = (%q, %v), want %q", accountID, ok, accountA.ID)
	}

	accountB := &domain.Account{
		ID:        "acct-stick-b",
		Email:     "stick-b@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-stick-b",
		BucketKey: "claude:subject-stick-b",
		CreatedAt: time.Now().UTC(),
	}
	saveRelayTestAccount(t, mockStore, accountB)

	prepared2 := &preparedRelayRequest{
		keyInfo: keyInfo,
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		surface:            domain.SurfaceNative,
		affinityKey:        "affinity-stick",
		affinityContinuity: driver.AffinityPrefer,
	}
	if outcome := relaySvc.executeRelayAttempt(context.Background(), httptest.NewRecorder(), driverStub, prepared2, newRelayAttemptState(), 0); outcome != relayAttemptDone {
		t.Fatalf("second executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}

	logs := waitRequestLogsCount(t, mockStore, 2)
	// The second request reuses the stable provider identity despite another
	// eligible account becoming available.
	for _, entry := range logs {
		if entry.AccountID != accountA.ID {
			t.Fatalf("expected all logs to target %q, got %q", accountA.ID, entry.AccountID)
		}
	}
}

func TestRelayRebindsPortableSessionWhenAffinityOwnerUnavailable(t *testing.T) {
	mockStore := store.NewMockStore()
	accountA := &domain.Account{
		ID:        "acct-route-a",
		Email:     "route-a@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-route-a",
		BucketKey: "claude:subject-route-a",
		CreatedAt: time.Now().UTC(),
	}
	accountB := &domain.Account{
		ID:        "acct-route-b",
		Email:     "route-b@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-route-b",
		BucketKey: "claude:subject-route-b",
		CreatedAt: time.Now().UTC(),
	}
	saveRelayTestAccount(t, mockStore, accountA)
	saveRelayTestAccount(t, mockStore, accountB)

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}
	if err := p.SetSessionBinding(context.Background(), "affinity-route", accountA, time.Hour); err != nil {
		t.Fatalf("SetSessionBinding: %v", err)
	}
	p.Observe(accountA.ID, driver.Effect{
		Kind:           driver.EffectCooldown,
		Scope:          driver.EffectScopeBucket,
		CooldownUntil:  time.Now().Add(time.Hour),
		UpstreamStatus: 429,
	})

	driverStub := &relayTestDriver{provider: domain.ProviderClaude}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id":"msg_2","type":"message","usage":{"input_tokens":1,"output_tokens":1}}`)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1, SessionBindingTTL: time.Hour},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	keyInfo := &auth.KeyInfo{ID: "user-route", Name: "mike"}
	prepared := &preparedRelayRequest{
		keyInfo: keyInfo,
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		surface:            domain.SurfaceNative,
		affinityKey:        "affinity-route",
		affinityContinuity: driver.AffinityPrefer,
	}
	if outcome := relaySvc.executeRelayAttempt(context.Background(), httptest.NewRecorder(), driverStub, prepared, newRelayAttemptState(), 0); outcome != relayAttemptDone {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}

	accountID, ok, err := p.GetSessionBinding(context.Background(), "affinity-route")
	if err != nil {
		t.Fatalf("GetSessionBinding: %v", err)
	}
	if !ok || accountID != accountB.ID {
		t.Fatalf("affinity binding = (%q, %v), want rebound %q", accountID, ok, accountB.ID)
	}
	logs := waitRequestLogsCount(t, mockStore, 1)
	if logs[0].AccountID != accountB.ID {
		t.Fatalf("AccountID = %q, want rerouted %q", logs[0].AccountID, accountB.ID)
	}
}

func TestRelayDoesNotBindSessionAffinityOnReject(t *testing.T) {
	mockStore := store.NewMockStore()
	account := &domain.Account{
		ID:        "acct-reject-stick",
		Email:     "reject-stick@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-reject-stick",
		BucketKey: "claude:subject-reject-stick",
		CreatedAt: time.Now().UTC(),
	}
	saveRelayTestAccount(t, mockStore, account)

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{
		provider: domain.ProviderClaude,
		interpretFn: func(statusCode int, _ []byte) driver.Effect {
			if statusCode == http.StatusNotFound {
				return driver.Effect{
					Kind:           driver.EffectReject,
					Scope:          driver.EffectScopeBucket,
					UpstreamStatus: http.StatusNotFound,
				}
			}
			return driver.Effect{Kind: driver.EffectSuccess, Scope: driver.EffectScopeBucket}
		},
	}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"not_found_error","message":"model not found"}}`)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{MaxRetryAccounts: 1},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "user-reject-stick", Name: "mike"},
		input: &driver.RelayInput{
			Headers: make(http.Header),
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		surface:            domain.SurfaceNative,
		affinityKey:        "affinity-reject",
		affinityContinuity: driver.AffinityPrefer,
	}
	recorder := httptest.NewRecorder()
	if outcome := relaySvc.executeRelayAttempt(context.Background(), recorder, driverStub, prepared, newRelayAttemptState(), 0); outcome != relayAttemptDone {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("response status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	accountID, ok, err := p.GetSessionBinding(context.Background(), "affinity-reject")
	if err != nil {
		t.Fatalf("GetSessionBinding: %v", err)
	}
	if ok || accountID != "" {
		t.Fatalf("affinity binding after reject = (%q, %v), want absent", accountID, ok)
	}
}

func TestExecuteRelayAttemptLogsCompatTraceEnvelope(t *testing.T) {
	mockStore := store.NewMockStore()
	account := &domain.Account{
		ID:        "acct-trace",
		Email:     "trace@example.com",
		Provider:  domain.ProviderClaude,
		Status:    domain.StatusActive,
		Subject:   "subject-trace",
		CellID:    "cell-compat",
		CreatedAt: time.Now().UTC(),
	}
	if err := mockStore.SaveAccount(context.Background(), account); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	if err := mockStore.SaveEgressCell(context.Background(), &domain.EgressCell{
		ID:        "cell-compat",
		Name:      "compat",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "10.0.0.3", Port: 11082},
		Labels:    map[string]string{"lane": "compat"},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveEgressCell: %v", err)
	}
	if err := mockStore.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: "claude:subject-trace",
		Provider:  domain.ProviderClaude,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket: %v", err)
	}

	bus := events.NewBus(16)
	p, err := pool.New(mockStore, bus)
	if err != nil {
		t.Fatalf("pool.New: %v", err)
	}

	driverStub := &relayTestDriver{
		provider:         domain.ProviderClaude,
		buildRequestURL:  "https://api.anthropic.com/v1/messages",
		buildRequestBody: `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}`,
		buildRequestHeaders: http.Header{
			"Content-Type":  []string{"application/json"},
			"Authorization": []string{"Bearer secret-upstream"},
			"User-Agent":    []string{"claude-cli/2.2.0"},
		},
		interpretFn: func(statusCode int, _ []byte) driver.Effect {
			if statusCode == http.StatusBadRequest {
				return driver.Effect{
					Kind:                 driver.EffectCooldown,
					Scope:                driver.EffectScopeBucket,
					CooldownUntil:        time.Now().Add(5 * time.Minute),
					UpstreamErrorType:    "invalid_request_error",
					UpstreamErrorMessage: "Error",
				}
			}
			return driver.Effect{Kind: driver.EffectSuccess, Scope: driver.EffectScopeBucket}
		},
	}
	transport := relayTestTransport{
		client: &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				headers := make(http.Header)
				headers.Set("request-id", "req_trace")
				headers.Set("Content-Type", "application/json")
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     headers,
					Body: io.NopCloser(strings.NewReader(
						`{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}`,
					)),
				}, nil
			}),
		},
	}
	relaySvc := New(
		p,
		relayTestTokenProvider{},
		mockStore,
		Config{
			MaxRetryAccounts: 1,
			TraceCompat:      true,
		},
		transport,
		bus,
		map[domain.Provider]driver.ExecutionDriver{domain.ProviderClaude: driverStub},
	)

	capture := &captureHandler{}
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(oldLogger)

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer secret-client")
	headers.Set("X-Broker-Compat-Trace-Id", "compat-42")
	headers.Set("X-Broker-Compat-Client-Meta", `{"requested_model":"claude/claude-sonnet-4-6","message_count":1}`)
	prepared := &preparedRelayRequest{
		keyInfo: &auth.KeyInfo{ID: "user-trace", Name: "trace"},
		surface: domain.SurfaceCompat,
		input: &driver.RelayInput{
			Headers: headers,
			Path:    "/compat/v1/chat/completions",
			Model:   "claude-sonnet-4-6",
			RawBody: []byte(`{"model":"claude/claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}`),
		},
	}

	outcome := relaySvc.executeRelayAttempt(
		context.Background(),
		httptest.NewRecorder(),
		driverStub,
		prepared,
		newRelayAttemptState(),
		0,
	)
	if outcome != relayAttemptDone {
		t.Fatalf("executeRelayAttempt outcome = %v, want %v", outcome, relayAttemptDone)
	}

	reqRecord := capture.find("compat upstream request")
	if reqRecord == nil {
		t.Fatal("missing compat upstream request log record")
	}
	if reqRecord.attrs["traceId"] != "compat-42" {
		t.Fatalf("traceId = %#v, want compat-42", reqRecord.attrs["traceId"])
	}
	if reqRecord.attrs["upstreamBody"] != `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}` {
		t.Fatalf("upstreamBody = %#v", reqRecord.attrs["upstreamBody"])
	}
	headersAny, ok := reqRecord.attrs["upstreamHeaders"].(map[string]string)
	if !ok {
		t.Fatalf("upstreamHeaders type = %T, want map[string]string", reqRecord.attrs["upstreamHeaders"])
	}
	if _, exists := headersAny["Authorization"]; exists {
		t.Fatalf("authorization header should not be logged: %#v", headersAny)
	}

	respRecord := capture.find("compat upstream response")
	if respRecord == nil {
		t.Fatal("missing compat upstream response log record")
	}
	if respRecord.attrs["traceId"] != "compat-42" {
		t.Fatalf("response traceId = %#v, want compat-42", respRecord.attrs["traceId"])
	}
	if respRecord.attrs["responseBody"] != `{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}` {
		t.Fatalf("responseBody = %#v", respRecord.attrs["responseBody"])
	}

	var logs []*domain.RequestLog
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var total int
		logs, total, err = mockStore.QueryRequestLogs(context.Background(), domain.RequestLogQuery{})
		if err != nil {
			t.Fatalf("QueryRequestLogs: %v", err)
		}
		if total == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(logs) != 1 {
		t.Fatalf("len(logs) = %d, want 1", len(logs))
	}
	if logs[0].UpstreamErrorType != "invalid_request_error" {
		t.Fatalf("UpstreamErrorType = %q, want invalid_request_error", logs[0].UpstreamErrorType)
	}
}
