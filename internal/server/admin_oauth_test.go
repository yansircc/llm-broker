package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/crypto"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/store"
	"github.com/yansircc/llm-broker/internal/tokens"
	"github.com/yansircc/llm-broker/internal/transport"
)

type exchangeStubDriver struct {
	provider           domain.Provider
	authURL            string
	session            driver.OAuthSession
	result             *driver.ExchangeResult
	lastExchangeClient *http.Client
}

func (d *exchangeStubDriver) Provider() domain.Provider { return d.provider }
func (d *exchangeStubDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	var state struct {
		ProjectID string `json:"project_id"`
	}
	_ = json.Unmarshal([]byte(acct.ProviderStateJSON), &state)
	if acct.Subject != "" && state.ProjectID != "" {
		return string(d.provider) + ":" + acct.Subject + ":" + state.ProjectID
	}
	if acct.Subject != "" {
		return string(d.provider) + ":" + acct.Subject
	}
	return string(d.provider) + ":" + acct.ID
}
func (d *exchangeStubDriver) Info() driver.ProviderInfo {
	return driver.ProviderInfo{Label: string(d.provider), OAuthStateRequired: true}
}
func (d *exchangeStubDriver) Models() []driver.Model                   { return nil }
func (d *exchangeStubDriver) Plan(*driver.RelayInput) driver.RelayPlan { return driver.RelayPlan{} }
func (d *exchangeStubDriver) BuildRequest(context.Context, *driver.RelayInput, *domain.Account, string) (*http.Request, error) {
	return nil, nil
}
func (d *exchangeStubDriver) Interpret(int, http.Header, []byte, string, json.RawMessage) driver.Effect {
	return driver.Effect{}
}
func (d *exchangeStubDriver) StreamResponse(context.Context, http.ResponseWriter, *http.Response) (bool, *driver.Usage) {
	return false, nil
}
func (d *exchangeStubDriver) ForwardResponse(http.ResponseWriter, *http.Response) {}
func (d *exchangeStubDriver) ParseJSONUsage([]byte) *driver.Usage                 { return nil }
func (d *exchangeStubDriver) ShouldRetry(int) bool                                { return false }
func (d *exchangeStubDriver) RetrySameAccount(int, []byte, int) bool              { return false }
func (d *exchangeStubDriver) ParseNonRetriable(int, []byte) bool                  { return false }
func (d *exchangeStubDriver) WriteError(http.ResponseWriter, int, string)         {}
func (d *exchangeStubDriver) WriteUpstreamError(http.ResponseWriter, int, []byte, bool) {
}
func (d *exchangeStubDriver) InterceptRequest(http.ResponseWriter, map[string]interface{}, string) bool {
	return false
}
func (d *exchangeStubDriver) GenerateAuthURL() (string, driver.OAuthSession, error) {
	return d.authURL, d.session, nil
}
func (d *exchangeStubDriver) ExchangeCode(_ context.Context, client *http.Client, _, _, _ string) (*driver.ExchangeResult, error) {
	d.lastExchangeClient = client
	return d.result, nil
}
func (d *exchangeStubDriver) RefreshToken(context.Context, *http.Client, string) (*driver.TokenResponse, error) {
	return nil, nil
}
func (d *exchangeStubDriver) Probe(context.Context, *domain.Account, string, *http.Client) (driver.ProbeResult, error) {
	return driver.ProbeResult{}, nil
}
func (d *exchangeStubDriver) DescribeAccount(*domain.Account) []driver.AccountField { return nil }
func (d *exchangeStubDriver) AutoPriority(json.RawMessage) int                      { return 50 }
func (d *exchangeStubDriver) IsStale(json.RawMessage, time.Time) bool               { return false }
func (d *exchangeStubDriver) ComputeExhaustedCooldown(json.RawMessage, time.Time) time.Time {
	return time.Time{}
}
func (d *exchangeStubDriver) CanServe(json.RawMessage, string, time.Time) bool { return true }
func (d *exchangeStubDriver) CalcCost(string, *driver.Usage) float64           { return 0 }
func (d *exchangeStubDriver) GetUtilization(json.RawMessage) []driver.UtilWindow {
	return nil
}

func TestHandleGenerateAuthURLStoresSelectedCell(t *testing.T) {
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	if err := p.SaveCell(&domain.EgressCell{
		ID:        "cell-fr-linode-02",
		Name:      "FR Linode 02",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11082},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell() error = %v", err)
	}

	stub := &exchangeStubDriver{
		provider: domain.ProviderClaude,
		authURL:  "https://example.com/oauth",
		session: driver.OAuthSession{
			CodeVerifier: "verifier-123",
			State:        "state-123",
		},
	}

	srv := &Server{
		cfg:          &config.Config{},
		store:        ms,
		pool:         p,
		bus:          bus,
		oauthDrivers: map[domain.Provider]driver.OAuthDriver{domain.ProviderClaude: stub},
	}

	req := adminRequest("POST", "/admin/accounts/generate-auth-url")
	req.Body = io.NopCloser(bytes.NewReader([]byte(`{"provider":"claude","cell_id":"cell-fr-linode-02"}`)))
	w := httptest.NewRecorder()

	srv.handleGenerateAuthURL(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		SessionID string `json:"session_id"`
		AuthURL   string `json:"auth_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.AuthURL != "https://example.com/oauth" {
		t.Fatalf("auth_url = %q", resp.AuthURL)
	}

	sessionJSON, ok, err := p.GetDelOAuthSession(context.Background(), resp.SessionID)
	if err != nil {
		t.Fatalf("GetDelOAuthSession() error = %v", err)
	}
	if !ok {
		t.Fatal("expected oauth session to be stored")
	}

	var envelope oauthSessionEnvelope
	if err := json.Unmarshal([]byte(sessionJSON), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(session) error = %v", err)
	}
	if envelope.Provider != "claude" {
		t.Fatalf("provider = %q", envelope.Provider)
	}
	if envelope.CellID != "cell-fr-linode-02" {
		t.Fatalf("cell_id = %q", envelope.CellID)
	}
	if envelope.CodeVerifier != "verifier-123" || envelope.State != "state-123" {
		t.Fatalf("oauth session = %#v", envelope.OAuthSession)
	}
}

func TestHandleGenerateAuthURLAllowsLegacyDirect(t *testing.T) {
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	stub := &exchangeStubDriver{
		provider: domain.ProviderClaude,
		authURL:  "https://example.com/oauth",
		session: driver.OAuthSession{
			CodeVerifier: "verifier-123",
			State:        "state-123",
		},
	}

	srv := &Server{
		cfg:          &config.Config{},
		store:        ms,
		pool:         p,
		bus:          bus,
		oauthDrivers: map[domain.Provider]driver.OAuthDriver{domain.ProviderClaude: stub},
	}

	req := adminRequest("POST", "/admin/accounts/generate-auth-url")
	req.Body = io.NopCloser(bytes.NewReader([]byte(`{"provider":"claude","cell_id":""}`)))
	w := httptest.NewRecorder()

	srv.handleGenerateAuthURL(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	sessionJSON, ok, err := p.GetDelOAuthSession(context.Background(), resp.SessionID)
	if err != nil {
		t.Fatalf("GetDelOAuthSession() error = %v", err)
	}
	if !ok {
		t.Fatal("expected oauth session to be stored")
	}

	var envelope oauthSessionEnvelope
	if err := json.Unmarshal([]byte(sessionJSON), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(session) error = %v", err)
	}
	if envelope.CellID != "" {
		t.Fatalf("cell_id = %q, want empty legacy direct binding", envelope.CellID)
	}
}

func TestHandleExchangeCodeCreatesAccountBoundToCell(t *testing.T) {
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	if err := p.SaveCell(&domain.EgressCell{
		ID:        "cell-fr-linode-03",
		Name:      "FR Linode 03",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11083},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell() error = %v", err)
	}

	c := crypto.New("test-encryption-key")
	tm := tokens.NewManager(p, c, nil, time.Minute, 0, nil)
	tp := transport.NewPool(time.Minute)
	stub := &exchangeStubDriver{
		provider: domain.ProviderClaude,
		result: &driver.ExchangeResult{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    3600,
			Subject:      "sub-123",
			Email:        "bound@example.com",
			Identity:     map[string]string{"sub": "sub-123"},
		},
	}
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: stub,
	})

	srv := &Server{
		cfg:           &config.Config{},
		store:         ms,
		pool:          p,
		tokens:        tm,
		transportPool: tp,
		bus:           bus,
		oauthDrivers:  map[domain.Provider]driver.OAuthDriver{domain.ProviderClaude: stub},
	}

	if err := p.SetOAuthSession(context.Background(), "session-1", `{"provider":"claude","cell_id":"cell-fr-linode-03","code_verifier":"verifier","state":"state"}`, 10*time.Minute); err != nil {
		t.Fatalf("SetOAuthSession() error = %v", err)
	}

	req := adminRequest("POST", "/admin/accounts/exchange-code")
	req.Body = io.NopCloser(bytes.NewReader([]byte(`{"session_id":"session-1","callback_url":"https://example.com/callback?code=auth-code"}`)))
	w := httptest.NewRecorder()

	srv.handleExchangeCode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp exchangeAccountResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	acct := p.Get(resp.ID)
	if acct == nil {
		t.Fatal("account not found after exchange")
	}
	if acct.CellID != "cell-fr-linode-03" {
		t.Fatalf("CellID = %q", acct.CellID)
	}

	expectedTransport := tp.ClientForAccount(&domain.Account{
		CellID: "cell-fr-linode-03",
		Cell:   p.GetCell("cell-fr-linode-03"),
	}).Transport
	if stub.lastExchangeClient == nil {
		t.Fatal("expected exchange client to be passed to driver")
	}
	if stub.lastExchangeClient.Transport != expectedTransport {
		t.Fatalf("exchange transport = %#v, want %#v", stub.lastExchangeClient.Transport, expectedTransport)
	}
}

func TestHandleExchangeCodeAllowsLegacyDirectForNewAccount(t *testing.T) {
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	c := crypto.New("test-encryption-key")
	tm := tokens.NewManager(p, c, nil, time.Minute, 0, nil)
	tp := transport.NewPool(time.Minute)
	stub := &exchangeStubDriver{
		provider: domain.ProviderClaude,
		result: &driver.ExchangeResult{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    3600,
			Subject:      "sub-123",
			Email:        "bound@example.com",
			Identity:     map[string]string{"sub": "sub-123"},
		},
	}
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: stub,
	})

	srv := &Server{
		cfg:           &config.Config{},
		store:         ms,
		pool:          p,
		tokens:        tm,
		transportPool: tp,
		bus:           bus,
		oauthDrivers:  map[domain.Provider]driver.OAuthDriver{domain.ProviderClaude: stub},
	}

	req := adminRequest("POST", "/admin/accounts/exchange-code")
	req.Body = io.NopCloser(bytes.NewReader([]byte(`{"provider":"claude","code":"auth-code","code_verifier":"verifier","state":"state"}`)))
	w := httptest.NewRecorder()

	srv.handleExchangeCode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp exchangeAccountResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	acct := p.Get(resp.ID)
	if acct == nil {
		t.Fatal("account not found after exchange")
	}
	if acct.CellID != "" {
		t.Fatalf("CellID = %q, want empty legacy direct binding", acct.CellID)
	}
	if stub.lastExchangeClient == nil {
		t.Fatal("expected exchange client to be passed to driver")
	}
}

func TestHandleExchangeCodePreservesExistingRefreshTokenOnRebind(t *testing.T) {
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	c := crypto.New("test-encryption-key")
	tm := tokens.NewManager(p, c, nil, time.Minute, 0, nil)
	tp := transport.NewPool(time.Minute)
	oldRefreshEnc, err := tm.EncryptToken("old-refresh")
	if err != nil {
		t.Fatalf("EncryptToken(old refresh) error = %v", err)
	}
	oldAccessEnc, err := tm.EncryptToken("old-access")
	if err != nil {
		t.Fatalf("EncryptToken(old access) error = %v", err)
	}

	existing := &domain.Account{
		ID:              "acct-1",
		Email:           "old@example.com",
		Provider:        domain.ProviderGemini,
		Subject:         "google-sub",
		Status:          domain.StatusActive,
		Priority:        50,
		PriorityMode:    "auto",
		CellID:          "cell-uk-linode-02",
		RefreshTokenEnc: oldRefreshEnc,
		AccessTokenEnc:  oldAccessEnc,
		CreatedAt:       time.Now().UTC(),
	}
	if err := p.SaveCell(&domain.EgressCell{
		ID:        "cell-uk-linode-02",
		Name:      "UK Linode 02(local)",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11082},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell() error = %v", err)
	}
	if err := p.Add(existing); err != nil {
		t.Fatalf("pool.Add() error = %v", err)
	}

	stub := &exchangeStubDriver{
		provider: domain.ProviderGemini,
		result: &driver.ExchangeResult{
			AccessToken:   "new-access",
			RefreshToken:  "",
			ExpiresIn:     3600,
			Subject:       "google-sub",
			Email:         "new@example.com",
			Identity:      map[string]string{"sub": "google-sub"},
			ProviderState: json.RawMessage(`{"project_id":"proj-123"}`),
		},
	}
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderGemini: stub,
	})
	srv := &Server{
		cfg:           &config.Config{},
		store:         ms,
		pool:          p,
		tokens:        tm,
		transportPool: tp,
		bus:           bus,
		oauthDrivers:  map[domain.Provider]driver.OAuthDriver{domain.ProviderGemini: stub},
	}

	body := []byte(`{"provider":"gemini","cell_id":"cell-uk-linode-02","code":"auth-code","code_verifier":"verifier","state":"state"}`)
	req := adminRequest("POST", "/admin/accounts/exchange-code")
	req.Body = io.NopCloser(bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleExchangeCode(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	acct := p.Get(existing.ID)
	if acct == nil {
		t.Fatal("updated account not found")
	}
	if acct.Email != "new@example.com" {
		t.Fatalf("Email = %q", acct.Email)
	}
	if acct.ProviderStateJSON != `{"project_id":"proj-123"}` {
		t.Fatalf("ProviderStateJSON = %q", acct.ProviderStateJSON)
	}

	refreshToken, err := c.Decrypt(acct.RefreshTokenEnc, "salt")
	if err != nil {
		t.Fatalf("Decrypt(refresh) error = %v", err)
	}
	if refreshToken != "old-refresh" {
		t.Fatalf("refresh token = %q, want old-refresh", refreshToken)
	}
}
