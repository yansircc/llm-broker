package pool

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/store"
)

// mockDriver is a minimal Driver implementation for pool tests.
type mockDriver struct {
	provider domain.Provider
}

func (m *mockDriver) Provider() domain.Provider { return m.provider }
func (m *mockDriver) BucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.BucketKey != "" {
		return acct.BucketKey
	}
	if acct.Subject != "" {
		return string(m.provider) + ":" + acct.Subject
	}
	return string(m.provider) + ":" + acct.ID
}
func (m *mockDriver) Info() driver.ProviderInfo {
	return driver.ProviderInfo{Label: string(m.provider), ProbeLabel: "mock"}
}
func (m *mockDriver) Models() []driver.Model                     { return nil }
func (m *mockDriver) Plan(_ *driver.RelayInput) driver.RelayPlan { return driver.RelayPlan{} }
func (m *mockDriver) AutoPriority(state json.RawMessage) int {
	return 50 // default
}
func (m *mockDriver) BuildRequest(_ context.Context, _ *driver.RelayInput, _ *domain.Account, _ string) (*http.Request, error) {
	return nil, nil
}
func (m *mockDriver) Interpret(_ int, _ http.Header, _ []byte, _ string, _ json.RawMessage) driver.Effect {
	return driver.Effect{}
}
func (m *mockDriver) StreamResponse(_ context.Context, _ http.ResponseWriter, _ *http.Response) (bool, *driver.Usage) {
	return false, nil
}
func (m *mockDriver) ForwardResponse(_ http.ResponseWriter, _ *http.Response) {}
func (m *mockDriver) ParseJSONUsage(_ []byte) *driver.Usage                   { return nil }
func (m *mockDriver) ShouldRetry(_ int) bool                                  { return false }
func (m *mockDriver) RetrySameAccount(_ int, _ []byte, _ int) bool            { return false }
func (m *mockDriver) ParseNonRetriable(_ int, _ []byte) bool                  { return false }
func (m *mockDriver) WriteError(_ http.ResponseWriter, _ int, _ string)       {}
func (m *mockDriver) WriteUpstreamError(_ http.ResponseWriter, _ int, _ []byte, _ bool) {
}
func (m *mockDriver) InterceptRequest(_ http.ResponseWriter, _ map[string]interface{}, _ string) bool {
	return false
}
func (m *mockDriver) GenerateAuthURL() (string, driver.OAuthSession, error) {
	return "", driver.OAuthSession{}, nil
}
func (m *mockDriver) ExchangeCode(_ context.Context, _ *http.Client, _, _, _ string) (*driver.ExchangeResult, error) {
	return nil, nil
}
func (m *mockDriver) RefreshToken(_ context.Context, _ *http.Client, _ string) (*driver.TokenResponse, error) {
	return nil, nil
}
func (m *mockDriver) Probe(_ context.Context, _ *domain.Account, _ string, _ *http.Client) (driver.ProbeResult, error) {
	return driver.ProbeResult{}, nil
}
func (m *mockDriver) DescribeAccount(_ *domain.Account) []driver.AccountField { return nil }
func (m *mockDriver) IsStale(_ json.RawMessage, _ time.Time) bool             { return false }
func (m *mockDriver) ComputeExhaustedCooldown(_ json.RawMessage, _ time.Time) time.Time {
	return time.Time{}
}
func (m *mockDriver) CanServe(state json.RawMessage, model string, _ time.Time) bool {
	if !strings.Contains(model, "opus") {
		return true
	}
	var flags struct {
		DenyOpus bool `json:"deny_opus"`
	}
	if json.Unmarshal(state, &flags) != nil {
		return true
	}
	return !flags.DenyOpus
}
func (m *mockDriver) CalcCost(_ string, _ *driver.Usage) float64           { return 0 }
func (m *mockDriver) GetUtilization(_ json.RawMessage) []driver.UtilWindow { return nil }

var testDriver = &mockDriver{provider: domain.ProviderClaude}

func newTestPool(t *testing.T, accounts ...*domain.Account) *Pool {
	t.Helper()
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	for _, a := range accounts {
		if err := ms.SaveAccount(context.Background(), a); err != nil {
			t.Fatalf("SaveAccount(%s): %v", a.ID, err)
		}
		key := a.BucketKey
		if key == "" {
			if a.Subject != "" {
				key = string(a.Provider) + ":" + a.Subject
			} else {
				key = string(a.Provider) + ":" + a.ID
			}
		}
		if err := ms.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
			BucketKey:     key,
			Provider:      a.Provider,
			CooldownUntil: a.CooldownUntil,
			StateJSON:     a.ProviderStateJSON,
			UpdatedAt:     time.Now().UTC(),
		}); err != nil {
			t.Fatalf("SaveQuotaBucket(%s): %v", key, err)
		}
	}
	p, err := New(ms, bus)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return p
}

func activeAccount(id, email string) *domain.Account {
	return &domain.Account{
		ID:       id,
		Email:    email,
		Provider: domain.ProviderClaude,
		Subject:  id,
		Status:   domain.StatusActive,
		Priority: 50,
	}
}

// Test 1: Pick never returns unavailable accounts
func TestPick_NeverReturnsUnavailable(t *testing.T) {
	blocked := &domain.Account{ID: "b", Email: "b@x", Provider: domain.ProviderClaude, Subject: "b", Status: domain.StatusBlocked, Priority: 99}
	cooling := activeAccount("c", "c@x")
	overloaded := activeAccount("o", "o@x")
	future := time.Now().Add(1 * time.Hour)
	cooling.CooldownUntil = &future
	overloaded.CooldownUntil = &future
	opusLimited := activeAccount("op", "op@x")
	opusLimited.ProviderStateJSON = `{"deny_opus":true}`
	good := activeAccount("g", "g@x")
	good.Priority = 99 // highest priority to ensure deterministic selection

	p := newTestPool(t, blocked, cooling, overloaded, opusLimited, good)

	// Pick should return "good" (highest priority, others filtered)
	acct, err := p.Pick(testDriver, nil, "claude-haiku", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "g" {
		t.Fatalf("expected g, got %s", acct.ID)
	}

	// With opus, opusLimited should also be excluded, still get "good"
	acct, err = p.Pick(testDriver, nil, "claude-opus-4-6", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "g" {
		t.Fatalf("expected g with opus, got %s", acct.ID)
	}

	// Exclude "good" with opus → no accounts available (opusLimited blocked by opus check)
	_, err = p.Pick(testDriver, []Exclusion{ExcludeAccount("g")}, "claude-opus-4-6", "")
	if err == nil {
		t.Fatal("expected error when all accounts unavailable for opus")
	}
}

func TestPick_SkipsUnavailableCell(t *testing.T) {
	good := activeAccount("good", "good@x")
	good.CellID = "cell-good"
	bad := activeAccount("bad", "bad@x")
	bad.Priority = 99
	bad.CellID = "cell-bad"

	p := newTestPool(t, good, bad)
	if err := p.SaveCell(&domain.EgressCell{
		ID:        "cell-good",
		Name:      "good",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "10.0.0.2", Port: 11081},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell(cell-good): %v", err)
	}
	if err := p.SaveCell(&domain.EgressCell{
		ID:        "cell-bad",
		Name:      "bad",
		Status:    domain.EgressCellDisabled,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "10.0.0.3", Port: 11082},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell(cell-bad): %v", err)
	}

	acct, err := p.Pick(testDriver, nil, "claude-haiku", "")
	if err != nil {
		t.Fatalf("Pick(): %v", err)
	}
	if acct.ID != "good" {
		t.Fatalf("Pick() = %s, want good", acct.ID)
	}
	if acct.Cell == nil || acct.Cell.ID != "cell-good" {
		t.Fatalf("acct.Cell = %#v, want cell-good", acct.Cell)
	}
}

func TestCooldownCellForAccount(t *testing.T) {
	acct := activeAccount("a", "a@x")
	acct.CellID = "cell-a"

	p := newTestPool(t, acct)
	if err := p.SaveCell(&domain.EgressCell{
		ID:        "cell-a",
		Name:      "cell-a",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11080},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell(cell-a): %v", err)
	}

	until := time.Now().Add(2 * time.Minute)
	if !p.CooldownCellForAccount("a", until, "transport error") {
		t.Fatal("CooldownCellForAccount() = false, want true")
	}

	cell := p.GetCell("cell-a")
	if cell == nil || cell.CooldownUntil == nil {
		t.Fatal("cell cooldown should be set")
	}
	if cell.CooldownUntil.UTC().Unix() != until.UTC().Unix() {
		t.Fatalf("cell cooldown = %v, want %v", cell.CooldownUntil, until.UTC())
	}

	if _, err := p.Pick(testDriver, nil, "claude-haiku", ""); err == nil {
		t.Fatal("Pick() should fail while the only cell is cooling down")
	}
}

func TestClearCellCooldown(t *testing.T) {
	acct := activeAccount("a", "a@x")
	acct.CellID = "cell-a"
	until := time.Now().Add(2 * time.Minute).UTC()

	p := newTestPool(t, acct)
	if err := p.SaveCell(&domain.EgressCell{
		ID:            "cell-a",
		Name:          "cell-a",
		Status:        domain.EgressCellActive,
		Proxy:         &domain.ProxyConfig{Type: "socks5", Host: "127.0.0.1", Port: 11080},
		CooldownUntil: &until,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveCell(cell-a): %v", err)
	}

	if !p.ClearCellCooldown("cell-a") {
		t.Fatal("ClearCellCooldown() = false, want true")
	}

	cell := p.GetCell("cell-a")
	if cell == nil {
		t.Fatal("cell should still exist")
	}
	if cell.CooldownUntil != nil {
		t.Fatalf("cell cooldown = %v, want nil", cell.CooldownUntil)
	}
}

func TestPoolReadsFreshStateFromStore(t *testing.T) {
	ms := store.NewMockStore()
	acct := activeAccount("acct-1", "old@example.com")
	acct.CreatedAt = time.Now().UTC()
	if err := ms.SaveAccount(context.Background(), acct); err != nil {
		t.Fatalf("SaveAccount(initial): %v", err)
	}

	bus := events.NewBus(100)
	p, err := New(ms, bus)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	updated := *acct
	updated.Email = "new@example.com"
	updated.Status = domain.StatusDisabled
	if err := ms.SaveAccount(context.Background(), &updated); err != nil {
		t.Fatalf("SaveAccount(updated): %v", err)
	}

	got := p.Get(acct.ID)
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Email != "new@example.com" {
		t.Fatalf("Get().Email = %q, want new@example.com", got.Email)
	}
	if got.Status != domain.StatusDisabled {
		t.Fatalf("Get().Status = %q, want disabled", got.Status)
	}

	if _, err := p.Pick(testDriver, nil, "claude-haiku", ""); err == nil {
		t.Fatal("Pick() = nil error, want no available accounts after external disable")
	}
}

// Test 2: applyBucketCooldown is monotonic
func TestApplyCooldown_Monotonic(t *testing.T) {
	acct := activeAccount("a", "a@x")
	p := newTestPool(t, acct)
	bucket := p.ensureBucketLocked(acct)

	long := time.Now().Add(1 * time.Hour)
	p.applyBucketCooldown(bucket, long)
	if bucket.CooldownUntil == nil || !bucket.CooldownUntil.Equal(long.UTC()) {
		t.Fatal("long cooldown should be set")
	}

	short := time.Now().Add(5 * time.Minute)
	p.applyBucketCooldown(bucket, short)
	if !bucket.CooldownUntil.Equal(long.UTC()) {
		t.Fatal("short cooldown should not overwrite long cooldown")
	}

	longer := time.Now().Add(2 * time.Hour)
	p.applyBucketCooldown(bucket, longer)
	if !bucket.CooldownUntil.Equal(longer.UTC()) {
		t.Fatal("longer cooldown should overwrite existing")
	}
}

// Test 3: Concurrent Observe with cooldown
func TestObserve_ConcurrentCooldown(t *testing.T) {
	acct := activeAccount("a", "a@x")
	p := newTestPool(t, acct)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.Observe("a", driver.Effect{
				Kind:          driver.EffectOverload,
				CooldownUntil: time.Now().Add(5 * time.Minute),
			})
		}(i)
	}
	wg.Wait()

	a := p.Get("a")
	if a == nil || a.CooldownUntil == nil {
		t.Fatal("cooldownUntil should be set after 529s")
	}
}

// Test 4: 401 → background refresh triggered
func TestObserve401_BackgroundRefresh(t *testing.T) {
	acct := activeAccount("a", "a@x")
	p := newTestPool(t, acct)

	called := make(chan string, 1)
	p.SetOnAuthFailure(func(accountID string) {
		called <- accountID
	})

	p.Observe("a", driver.Effect{
		Kind:          driver.EffectAuthFail,
		CooldownUntil: time.Now().Add(30 * time.Second),
	})

	// Verify account is NOT set to StatusError (key change from old behavior)
	a := p.Get("a")
	if a.Status == domain.StatusError {
		t.Fatal("401 should NOT set StatusError anymore")
	}
	if a.CooldownUntil == nil {
		t.Fatal("cooldownUntil should be set with Pause401Refresh")
	}

	// Verify callback was invoked
	select {
	case id := <-called:
		if id != "a" {
			t.Fatalf("expected accountID 'a', got %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onAuthFailure was not called")
	}
}

// Test 5: Status transitions for various effect kinds
func TestObserve_StatusTransitions(t *testing.T) {
	t.Run("529_cooldown", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectOverload,
			CooldownUntil: time.Now().Add(5 * time.Minute),
		})
		a := p.Get("a")
		if a == nil || a.CooldownUntil == nil {
			t.Fatal("cooldownUntil should be set")
		}
	})

	t.Run("429_cooldown", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectCooldown,
			CooldownUntil: time.Now().Add(1 * time.Minute),
		})
		a := p.Get("a")
		if a == nil || a.CooldownUntil == nil {
			t.Fatal("cooldownUntil should be set")
		}
	})

	t.Run("403_ban_blocked", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectBlock,
			CooldownUntil: time.Now().Add(30 * time.Minute),
			ErrorMessage:  "organization has been disabled",
		})
		a := p.Get("a")
		if a.Status != domain.StatusBlocked {
			t.Fatalf("expected blocked, got %s", a.Status)
		}
	})

	t.Run("403_nonban_cooldown", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectCooldown,
			CooldownUntil: time.Now().Add(10 * time.Minute),
		})
		a := p.Get("a")
		if a.Status != domain.StatusActive {
			t.Fatalf("non-ban 403 should keep status active, got %s", a.Status)
		}
		if a.CooldownUntil == nil {
			t.Fatal("cooldownUntil should be set after non-ban 403")
		}
	})
}

// Test 6: StoreTokens restores account
func TestStoreTokens_RestoresAccount(t *testing.T) {
	acct := activeAccount("a", "a@x")
	acct.Status = domain.StatusError
	future := time.Now().Add(1 * time.Hour)
	acct.CooldownUntil = &future
	p := newTestPool(t, acct)

	err := p.StoreTokens("a", "enc_access", "enc_refresh", time.Now().Add(1*time.Hour).Unix())
	if err != nil {
		t.Fatalf("StoreTokens failed: %v", err)
	}

	a := p.Get("a")
	if a.Status != domain.StatusActive {
		t.Fatalf("expected active, got %s", a.Status)
	}
	if a.CooldownUntil != nil {
		t.Fatal("cooldownUntil should be cleared")
	}
	if a.ErrorMessage != "" {
		t.Fatal("errorMessage should be cleared")
	}
}

// Test 7: Update persists under lock
func TestUpdate_PersistsUnderLock(t *testing.T) {
	acct := activeAccount("a", "a@x")
	p := newTestPool(t, acct)

	err := p.Update("a", func(a *domain.Account) {
		a.Priority = 99
		a.Email = "new@x"
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	p.mu.RLock()
	a := p.accounts["a"]
	if a.Priority != 99 {
		t.Fatalf("expected priority 99, got %d", a.Priority)
	}
	if a.Email != "new@x" {
		t.Fatalf("expected new@x, got %s", a.Email)
	}
	p.mu.RUnlock()

	// Verify persisted to store
	ms := p.store.(*store.MockStore)
	saved, _ := ms.GetAccount(nil, "a")
	if saved == nil || saved.Priority != 99 {
		t.Fatal("update should have been persisted to store")
	}
}

// Test 8: Pick sorts by priority DESC, lastUsedAt ASC
func TestPick_PrioritySort(t *testing.T) {
	a1 := activeAccount("a1", "a1@x")
	a1.Priority = 80
	now := time.Now()
	a1.LastUsedAt = &now

	a2 := activeAccount("a2", "a2@x")
	a2.Priority = 80
	past := now.Add(-1 * time.Hour)
	a2.LastUsedAt = &past

	a3 := activeAccount("a3", "a3@x")
	a3.Priority = 90

	p := newTestPool(t, a1, a2, a3)

	// a3 should win (highest priority)
	acct, err := p.Pick(testDriver, nil, "claude-haiku", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "a3" {
		t.Fatalf("expected a3 (highest priority), got %s", acct.ID)
	}

	// Exclude a3, then a2 should win (same priority, older lastUsedAt)
	acct, err = p.Pick(testDriver, []Exclusion{ExcludeAccount("a3")}, "claude-haiku", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "a2" {
		t.Fatalf("expected a2 (older lastUsedAt), got %s", acct.ID)
	}
}

func TestPick_ExcludeBucketSkipsSiblingAccounts(t *testing.T) {
	g1 := &domain.Account{
		ID:        "g1",
		Email:     "g1@example.com",
		Provider:  domain.ProviderGemini,
		Subject:   "google-sub",
		BucketKey: "gemini:google-sub:proj-1",
		Status:    domain.StatusActive,
		Priority:  50,
	}
	g2 := &domain.Account{
		ID:        "g2",
		Email:     "g2@example.com",
		Provider:  domain.ProviderGemini,
		Subject:   "google-sub",
		BucketKey: "gemini:google-sub:proj-1",
		Status:    domain.StatusActive,
		Priority:  50,
	}
	p := newTestPool(t, g1, g2)
	geminiDriver := &mockDriver{provider: domain.ProviderGemini}

	_, err := p.Pick(geminiDriver, []Exclusion{ExcludeBucket("gemini:google-sub:proj-1")}, "gemini-2.5-flash", "")
	if err == nil {
		t.Fatal("expected no available accounts when bucket is excluded")
	}
}

func TestPick_ReturnsBucketProjectedAccount(t *testing.T) {
	g1 := &domain.Account{
		ID:                "g1",
		Email:             "g1@example.com",
		Provider:          domain.ProviderGemini,
		Subject:           "google-sub",
		BucketKey:         "gemini:google-sub:proj-1",
		Status:            domain.StatusActive,
		Priority:          50,
		ProviderStateJSON: `{}`,
	}
	p := newTestPool(t, g1)
	if err := p.store.SaveQuotaBucket(context.Background(), &domain.QuotaBucket{
		BucketKey: "gemini:google-sub:proj-1",
		Provider:  domain.ProviderGemini,
		StateJSON: `{"project_id":"proj-1","rpm":1}`,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveQuotaBucket(): %v", err)
	}
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderGemini: &mockDriver{provider: domain.ProviderGemini},
	})

	acct, err := p.Pick(&mockDriver{provider: domain.ProviderGemini}, nil, "gemini-2.5-flash", "")
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if acct.ProviderStateJSON != `{"project_id":"proj-1","rpm":1}` {
		t.Fatalf("Pick() ProviderStateJSON = %q, want bucket projection", acct.ProviderStateJSON)
	}
}

func TestObserve_BucketScopeSyncsCooldownAndState(t *testing.T) {
	g1 := &domain.Account{
		ID:                "g1",
		Email:             "g1@example.com",
		Provider:          domain.ProviderGemini,
		Subject:           "google-sub",
		BucketKey:         "gemini:google-sub:proj-1",
		Status:            domain.StatusActive,
		Priority:          50,
		ProviderStateJSON: `{"project_id":"proj-1"}`,
	}
	g2 := &domain.Account{
		ID:                "g2",
		Email:             "g2@example.com",
		Provider:          domain.ProviderGemini,
		Subject:           "google-sub",
		BucketKey:         "gemini:google-sub:proj-1",
		Status:            domain.StatusActive,
		Priority:          50,
		ProviderStateJSON: `{"project_id":"proj-1"}`,
	}
	p := newTestPool(t, g1, g2)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderGemini: &mockDriver{provider: domain.ProviderGemini},
	})

	until := time.Now().Add(2 * time.Minute)
	p.Observe("g1", driver.Effect{
		Kind:          driver.EffectCooldown,
		Scope:         driver.EffectScopeBucket,
		CooldownUntil: until,
		UpdatedState:  json.RawMessage(`{"project_id":"proj-1","rpm":1}`),
	})

	for _, id := range []string{"g1", "g2"} {
		acct := p.Get(id)
		if acct == nil {
			t.Fatalf("account %s missing", id)
		}
		if acct.CooldownUntil == nil || acct.CooldownUntil.Unix() != until.UTC().Unix() {
			t.Fatalf("account %s cooldown = %v, want %v", id, acct.CooldownUntil, until.UTC())
		}
		if acct.ProviderStateJSON != `{"project_id":"proj-1","rpm":1}` {
			t.Fatalf("account %s ProviderStateJSON = %q", id, acct.ProviderStateJSON)
		}
	}
}
