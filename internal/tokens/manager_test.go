package tokens

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/crypto"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

type refreshStubDriver struct {
	resp *driver.TokenResponse
	err  error
}

func (d *refreshStubDriver) Provider() domain.Provider { return domain.ProviderGemini }
func (d *refreshStubDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.BucketKey != "" {
		return acct.BucketKey
	}
	return string(domain.ProviderGemini) + ":" + acct.ID
}
func (d *refreshStubDriver) Info() driver.ProviderInfo                { return driver.ProviderInfo{} }
func (d *refreshStubDriver) Models() []driver.Model                   { return nil }
func (d *refreshStubDriver) Plan(*driver.RelayInput) driver.RelayPlan { return driver.RelayPlan{} }
func (d *refreshStubDriver) BuildRequest(context.Context, *driver.RelayInput, *domain.Account, string) (*http.Request, error) {
	return nil, nil
}
func (d *refreshStubDriver) Interpret(int, http.Header, []byte, string, json.RawMessage) driver.Effect {
	return driver.Effect{}
}
func (d *refreshStubDriver) StreamResponse(context.Context, http.ResponseWriter, *http.Response) (bool, *driver.Usage) {
	return false, nil
}
func (d *refreshStubDriver) ForwardResponse(http.ResponseWriter, *http.Response) {}
func (d *refreshStubDriver) ParseJSONUsage([]byte) *driver.Usage                 { return nil }
func (d *refreshStubDriver) ShouldRetry(int) bool                                { return false }
func (d *refreshStubDriver) RetrySameAccount(int, []byte, int) bool              { return false }
func (d *refreshStubDriver) ParseNonRetriable(int, []byte) bool                  { return false }
func (d *refreshStubDriver) WriteError(http.ResponseWriter, int, string)         {}
func (d *refreshStubDriver) WriteUpstreamError(http.ResponseWriter, int, []byte, bool) {
}
func (d *refreshStubDriver) InterceptRequest(http.ResponseWriter, map[string]interface{}, string) bool {
	return false
}
func (d *refreshStubDriver) GenerateAuthURL() (string, driver.OAuthSession, error) {
	return "", driver.OAuthSession{}, nil
}
func (d *refreshStubDriver) ExchangeCode(context.Context, string, string, string) (*driver.ExchangeResult, error) {
	return nil, nil
}
func (d *refreshStubDriver) RefreshToken(context.Context, *http.Client, string) (*driver.TokenResponse, error) {
	return d.resp, d.err
}
func (d *refreshStubDriver) Probe(context.Context, *domain.Account, string, *http.Client) (driver.ProbeResult, error) {
	return driver.ProbeResult{}, nil
}
func (d *refreshStubDriver) DescribeAccount(*domain.Account) []driver.AccountField { return nil }
func (d *refreshStubDriver) AutoPriority(json.RawMessage) int                      { return 50 }
func (d *refreshStubDriver) IsStale(json.RawMessage, time.Time) bool               { return false }
func (d *refreshStubDriver) ComputeExhaustedCooldown(json.RawMessage, time.Time) time.Time {
	return time.Time{}
}
func (d *refreshStubDriver) CanServe(json.RawMessage, string, time.Time) bool { return true }
func (d *refreshStubDriver) CalcCost(string, *driver.Usage) float64           { return 0 }
func (d *refreshStubDriver) GetUtilization(json.RawMessage) []driver.UtilWindow {
	return nil
}

type refreshStubPool struct {
	acct            *domain.Account
	cooledCellID    string
	cooldownUntil   time.Time
	cooldownMessage string
}

func (p *refreshStubPool) Get(string) *domain.Account { return p.acct }
func (p *refreshStubPool) StoreTokens(_ string, accessTokenEnc, refreshTokenEnc string, expiresAt int64) error {
	p.acct.AccessTokenEnc = accessTokenEnc
	p.acct.RefreshTokenEnc = refreshTokenEnc
	p.acct.ExpiresAt = expiresAt
	return nil
}
func (p *refreshStubPool) MarkError(string, string) {}
func (p *refreshStubPool) AcquireRefreshLock(context.Context, string, string) (bool, error) {
	return true, nil
}
func (p *refreshStubPool) ReleaseRefreshLock(context.Context, string, string) error { return nil }
func (p *refreshStubPool) CooldownCell(cellID string, until time.Time, message string) bool {
	p.cooledCellID = cellID
	p.cooldownUntil = until
	p.cooldownMessage = message
	return true
}

func TestForceRefreshPreservesExistingRefreshTokenWhenProviderReturnsEmpty(t *testing.T) {
	c := crypto.New("test-encryption-key")
	oldRefreshEnc, err := c.Encrypt("old-refresh", tokenSalt)
	if err != nil {
		t.Fatalf("Encrypt(old refresh) error = %v", err)
	}
	oldAccessEnc, err := c.Encrypt("old-access", tokenSalt)
	if err != nil {
		t.Fatalf("Encrypt(old access) error = %v", err)
	}

	pool := &refreshStubPool{
		acct: &domain.Account{
			ID:              "acct-1",
			Provider:        domain.ProviderGemini,
			AccessTokenEnc:  oldAccessEnc,
			RefreshTokenEnc: oldRefreshEnc,
		},
	}
	drv := &refreshStubDriver{
		resp: &driver.TokenResponse{
			AccessToken: "new-access",
			ExpiresIn:   3600,
		},
	}
	mgr := NewManager(pool, c, nil, time.Minute, 0, map[domain.Provider]driver.RefreshDriver{
		domain.ProviderGemini: drv,
	})

	token, err := mgr.ForceRefresh(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("ForceRefresh() error = %v", err)
	}
	if token != "new-access" {
		t.Fatalf("ForceRefresh() token = %q", token)
	}

	refreshToken, err := c.Decrypt(pool.acct.RefreshTokenEnc, tokenSalt)
	if err != nil {
		t.Fatalf("Decrypt(refresh) error = %v", err)
	}
	if refreshToken != "old-refresh" {
		t.Fatalf("refresh token = %q, want preserved old-refresh", refreshToken)
	}

	accessToken, err := c.Decrypt(pool.acct.AccessTokenEnc, tokenSalt)
	if err != nil {
		t.Fatalf("Decrypt(access) error = %v", err)
	}
	if accessToken != "new-access" {
		t.Fatalf("access token = %q, want new-access", accessToken)
	}
}

func TestForceRefreshCooldownsCellOnTransportError(t *testing.T) {
	c := crypto.New("test-encryption-key")
	refreshEnc, err := c.Encrypt("old-refresh", tokenSalt)
	if err != nil {
		t.Fatalf("Encrypt(old refresh) error = %v", err)
	}

	pool := &refreshStubPool{
		acct: &domain.Account{
			ID:              "acct-1",
			Email:           "mark@example.com",
			Provider:        domain.ProviderGemini,
			RefreshTokenEnc: refreshEnc,
			CellID:          "cell-fr-par-mark",
		},
	}
	drv := &refreshStubDriver{err: errors.New("proxy connect tcp: connection refused")}
	mgr := NewManager(pool, c, nil, time.Minute, time.Minute, map[domain.Provider]driver.RefreshDriver{
		domain.ProviderGemini: drv,
	})

	_, err = mgr.ForceRefresh(context.Background(), "acct-1")
	if err == nil {
		t.Fatal("ForceRefresh() error = nil, want transport error")
	}
	if pool.cooledCellID != "cell-fr-par-mark" {
		t.Fatalf("cooledCellID = %q", pool.cooledCellID)
	}
	if pool.cooldownUntil.IsZero() {
		t.Fatal("cooldownUntil is zero")
	}
	if pool.cooldownUntil.Before(time.Now().Add(30 * time.Second)) {
		t.Fatalf("cooldownUntil = %s, want around now+1m", pool.cooldownUntil.Format(time.RFC3339))
	}
	if pool.cooldownMessage == "" {
		t.Fatal("cooldownMessage is empty")
	}
}
