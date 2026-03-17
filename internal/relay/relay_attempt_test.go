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
	buildRequestBody    string
	buildRequestURL     string
	buildRequestHeaders http.Header
}

func (d *relayTestDriver) Provider() domain.Provider { return d.provider }

func (d *relayTestDriver) Plan(_ *driver.RelayInput) driver.RelayPlan { return driver.RelayPlan{} }

func (d *relayTestDriver) BuildRequest(ctx context.Context, _ *driver.RelayInput, _ *domain.Account, _ string) (*http.Request, error) {
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

type relayTestTokenProvider struct{}

func (relayTestTokenProvider) EnsureValidToken(_ context.Context, _ string) (string, error) {
	return "tok", nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

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

	driverStub := &relayTestDriver{provider: domain.ProviderClaude}
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
		Config{MaxRetryAccounts: 1},
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
			Path:    "/v1/messages",
			Model:   "claude-sonnet-4-6",
		},
		sessionUUID: "session-123",
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
		Config{MaxRetryAccounts: 1},
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
		Config{MaxRetryAccounts: 1},
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
}
