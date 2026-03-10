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
)

type exchangeStubDriver struct {
	provider domain.Provider
	result   *driver.ExchangeResult
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
	return "", driver.OAuthSession{}, nil
}
func (d *exchangeStubDriver) ExchangeCode(context.Context, string, string, string) (*driver.ExchangeResult, error) {
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

func TestHandleExchangeCodePreservesExistingRefreshTokenOnRebind(t *testing.T) {
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p, err := pool.New(ms, bus)
	if err != nil {
		t.Fatalf("pool.New() error = %v", err)
	}

	c := crypto.New("test-encryption-key")
	tm := tokens.NewManager(p, c, nil, time.Minute, nil)
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
		RefreshTokenEnc: oldRefreshEnc,
		AccessTokenEnc:  oldAccessEnc,
		CreatedAt:       time.Now().UTC(),
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
		cfg:          &config.Config{},
		store:        ms,
		pool:         p,
		tokens:       tm,
		bus:          bus,
		oauthDrivers: map[domain.Provider]driver.OAuthDriver{domain.ProviderGemini: stub},
	}

	body := []byte(`{"provider":"gemini","code":"auth-code","code_verifier":"verifier","state":"state"}`)
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
