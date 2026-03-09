package pool

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/store"
)

// mockDriver is a minimal Driver implementation for pool tests.
type mockDriver struct {
	provider domain.Provider
}

func (m *mockDriver) Provider() domain.Provider { return m.provider }
func (m *mockDriver) Info() driver.ProviderInfo {
	return driver.ProviderInfo{Label: string(m.provider), ProbeLabel: "mock"}
}
func (m *mockDriver) Models() []driver.Model { return nil }
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
func (m *mockDriver) ExtractSessionUUID(_ map[string]interface{}) string { return "" }
func (m *mockDriver) GenerateAuthURL() (string, driver.OAuthSession, error) {
	return "", driver.OAuthSession{}, nil
}
func (m *mockDriver) ExchangeCode(_ context.Context, _, _, _ string) (*driver.ExchangeResult, error) {
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
	p := &Pool{
		accounts:      make(map[string]*domain.Account),
		store:         ms,
		bus:           bus,
		pauses:        driver.ErrorPauses{Pause401: 30 * time.Minute, Pause401Refresh: 30 * time.Second, Pause403: 10 * time.Minute, Pause429: 60 * time.Second, Pause529: 5 * time.Minute},
		sessions:      store.NewTTLMap[SessionBinding](),
		stainless:     store.NewTTLMap[string](),
		oauthSessions: store.NewTTLMap[string](),
		refreshLocks:  store.NewTTLMap[string](),
	}
	for _, a := range accounts {
		p.accounts[a.ID] = a
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
	_, err = p.Pick(testDriver, []string{"g"}, "claude-opus-4-6", "")
	if err == nil {
		t.Fatal("expected error when all accounts unavailable for opus")
	}
}

// Test 2: applyCooldown is monotonic
func TestApplyCooldown_Monotonic(t *testing.T) {
	acct := activeAccount("a", "a@x")
	p := newTestPool(t, acct)

	long := time.Now().Add(1 * time.Hour)
	p.applyCooldown(acct, long)
	if acct.CooldownUntil == nil || !acct.CooldownUntil.Equal(long.UTC()) {
		t.Fatal("long cooldown should be set")
	}

	short := time.Now().Add(5 * time.Minute)
	p.applyCooldown(acct, short)
	if !acct.CooldownUntil.Equal(long.UTC()) {
		t.Fatal("short cooldown should not overwrite long cooldown")
	}

	longer := time.Now().Add(2 * time.Hour)
	p.applyCooldown(acct, longer)
	if !acct.CooldownUntil.Equal(longer.UTC()) {
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

	p.mu.RLock()
	defer p.mu.RUnlock()
	a := p.accounts["a"]
	if a.CooldownUntil == nil {
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
	p.mu.RLock()
	a := p.accounts["a"]
	if a.Status == domain.StatusError {
		t.Fatal("401 should NOT set StatusError anymore")
	}
	if a.CooldownUntil == nil {
		t.Fatal("cooldownUntil should be set with Pause401Refresh")
	}
	p.mu.RUnlock()

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
		p.mu.RLock()
		a := p.accounts["a"]
		if a.CooldownUntil == nil {
			t.Fatal("cooldownUntil should be set")
		}
		p.mu.RUnlock()
	})

	t.Run("429_cooldown", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectCooldown,
			CooldownUntil: time.Now().Add(1 * time.Minute),
		})
		p.mu.RLock()
		a := p.accounts["a"]
		if a.CooldownUntil == nil {
			t.Fatal("cooldownUntil should be set")
		}
		p.mu.RUnlock()
	})

	t.Run("403_ban_blocked", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectBlock,
			CooldownUntil: time.Now().Add(30 * time.Minute),
			ErrorMessage:  "organization has been disabled",
		})
		p.mu.RLock()
		a := p.accounts["a"]
		if a.Status != domain.StatusBlocked {
			t.Fatalf("expected blocked, got %s", a.Status)
		}
		p.mu.RUnlock()
	})

	t.Run("403_nonban_cooldown", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe("a", driver.Effect{
			Kind:          driver.EffectCooldown,
			CooldownUntil: time.Now().Add(10 * time.Minute),
		})
		p.mu.RLock()
		a := p.accounts["a"]
		if a.Status != domain.StatusActive {
			t.Fatalf("non-ban 403 should keep status active, got %s", a.Status)
		}
		if a.CooldownUntil == nil {
			t.Fatal("cooldownUntil should be set after non-ban 403")
		}
		p.mu.RUnlock()
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

	p.mu.RLock()
	a := p.accounts["a"]
	if a.Status != domain.StatusActive {
		t.Fatalf("expected active, got %s", a.Status)
	}
	if a.CooldownUntil != nil {
		t.Fatal("cooldownUntil should be cleared")
	}
	if a.ErrorMessage != "" {
		t.Fatal("errorMessage should be cleared")
	}
	p.mu.RUnlock()
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
	acct, err = p.Pick(testDriver, []string{"a3"}, "claude-haiku", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "a2" {
		t.Fatalf("expected a2 (older lastUsedAt), got %s", acct.ID)
	}
}
