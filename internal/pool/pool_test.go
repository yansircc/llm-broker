package pool

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
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
	good.Priority = 99

	p := newTestPool(t, blocked, cooling, overloaded, opusLimited, good)

	// Non-opus may choose any available account, but never an unavailable one.
	acct, err := p.Pick(testDriver, nil, "claude-haiku", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "g" && acct.ID != "op" {
		t.Fatalf("expected an available account, got %s", acct.ID)
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

func TestPickForSurface_SeparatesNativeAndCompatLanes(t *testing.T) {
	native := activeAccount("native", "native@x")
	native.Priority = 80
	native.CellID = "cell-native"

	compat := activeAccount("compat", "compat@x")
	compat.Priority = 90
	compat.CellID = "cell-compat"

	p := newTestPool(t, native, compat)
	for _, cell := range []*domain.EgressCell{
		{
			ID:        "cell-native",
			Name:      "native",
			Status:    domain.EgressCellActive,
			Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "10.0.0.2", Port: 11081},
			Labels:    map[string]string{"lane": "native"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		{
			ID:        "cell-compat",
			Name:      "compat",
			Status:    domain.EgressCellActive,
			Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "10.0.0.3", Port: 11082},
			Labels:    map[string]string{"lane": "compat"},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	} {
		if err := p.SaveCell(cell); err != nil {
			t.Fatalf("SaveCell(%s): %v", cell.ID, err)
		}
	}

	gotNative, err := p.PickForSurface(testDriver, nil, "claude-haiku", "", domain.SurfaceNative)
	if err != nil {
		t.Fatalf("PickForSurface(native): %v", err)
	}
	if gotNative.ID != "native" {
		t.Fatalf("PickForSurface(native) = %s, want native", gotNative.ID)
	}

	gotCompat, err := p.PickForSurface(testDriver, nil, "claude-haiku", "", domain.SurfaceCompat)
	if err != nil {
		t.Fatalf("PickForSurface(compat): %v", err)
	}
	if gotCompat.ID != "compat" {
		t.Fatalf("PickForSurface(compat) = %s, want compat", gotCompat.ID)
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

func TestPick_SameBucketUsesLeastRecentlyUsed(t *testing.T) {
	now := time.Now()
	recent := activeAccount("recent", "recent@x")
	recent.BucketKey = "shared"
	recent.LastUsedAt = &now

	older := activeAccount("older", "older@x")
	older.BucketKey = "shared"
	past := now.Add(-1 * time.Hour)
	older.LastUsedAt = &past

	p := newTestPool(t, recent, older)

	acct, err := p.Pick(testDriver, nil, "claude-haiku", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "older" {
		t.Fatalf("expected older account from shared bucket, got %s", acct.ID)
	}
}

func TestPickBucketCandidate_UsesSqrtWeightedLottery(t *testing.T) {
	candidates := make([]bucketCandidate, 0, 15)
	for i := 0; i < 14; i++ {
		candidates = append(candidates, bucketCandidate{key: "old", priority: 20})
	}
	candidates = append(candidates, bucketCandidate{key: "new", priority: 100})

	rng := rand.New(rand.NewSource(1))
	const draws = 20000
	newHits := 0
	for i := 0; i < draws; i++ {
		chosen := pickBucketCandidate(candidates, func(totalWeight float64) float64 {
			return rng.Float64() * totalWeight
		})
		if chosen.key == "new" {
			newHits++
		}
	}

	share := float64(newHits) / draws
	want := bucketPriorityWeight(100) / (14*bucketPriorityWeight(20) + bucketPriorityWeight(100))
	if math.Abs(share-want) > 0.02 {
		t.Fatalf("new bucket share = %.4f, want around %.4f", share, want)
	}
	if share >= 0.20 {
		t.Fatalf("new bucket share = %.4f, want well below monopoly", share)
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

// ---------------------------------------------------------------------------
// Circuit breaker (PR #13)
// ---------------------------------------------------------------------------

func TestCircuitBreakerOnServerError(t *testing.T) {
	acct := activeAccount("cb-1", "cb@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	bucketKey := testDriver.BucketKey(acct)

	// First two EffectServerError should NOT trigger cooldown.
	for i := 0; i < 2; i++ {
		p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
		bucket := p.buckets[bucketKey]
		if bucket != nil && bucket.CooldownUntil != nil {
			t.Fatalf("unexpected cooldown after %d server errors", i+1)
		}
	}

	// Third should trigger cooldown.
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
	bucket := p.buckets[bucketKey]
	if bucket == nil || bucket.CooldownUntil == nil {
		t.Fatal("expected cooldown after 3 consecutive server errors")
	}
	if bucket.CooldownUntil.Before(time.Now()) {
		t.Fatal("cooldown should be in the future")
	}
}

func TestCircuitBreakerResetOnSuccess(t *testing.T) {
	acct := activeAccount("cb-2", "cb2@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	bucketKey := testDriver.BucketKey(acct)

	// Two server errors, then a success should reset the counter.
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectSuccess, Scope: driver.EffectScopeBucket})

	if count := p.serverErrCount[bucketKey]; count != 0 {
		t.Fatalf("expected counter reset after success, got %d", count)
	}

	// After reset, need 3 more errors to trigger cooldown.
	for i := 0; i < 2; i++ {
		p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
	}
	bucket := p.buckets[bucketKey]
	if bucket != nil && bucket.CooldownUntil != nil {
		t.Fatal("unexpected cooldown after only 2 server errors post-reset")
	}
}

func TestCircuitBreakerResetOnNon500Effect(t *testing.T) {
	acct := activeAccount("cb-3", "cb3@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	bucketKey := testDriver.BucketKey(acct)

	// Sequence: 500 → 429 → 500 → 500 should NOT trigger circuit breaker
	// because the 429 breaks the consecutive chain.
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectCooldown, Scope: driver.EffectScopeBucket, CooldownUntil: time.Now().Add(-time.Second)})
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})
	p.Observe(acct.ID, driver.Effect{Kind: driver.EffectServerError, Scope: driver.EffectScopeBucket})

	if count := p.serverErrCount[bucketKey]; count != 2 {
		t.Fatalf("expected counter = 2 after 429 reset, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Observability: CooldownResult and structured events
// ---------------------------------------------------------------------------

func TestApplyBucketCooldown_JoinSemantics(t *testing.T) {
	acct := activeAccount("join-1", "join@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})
	bucketKey := testDriver.BucketKey(acct)
	bucket := p.buckets[bucketKey]
	if bucket == nil {
		t.Fatal("bucket not found")
	}

	later := time.Now().Add(10 * time.Minute)
	r1 := p.applyBucketCooldown(bucket, later)
	if !r1.Applied {
		t.Fatal("first cooldown should be applied")
	}
	if r1.Actual.Unix() != later.UTC().Unix() {
		t.Fatalf("actual = %v, want %v", r1.Actual, later.UTC())
	}

	earlier := time.Now().Add(5 * time.Minute)
	r2 := p.applyBucketCooldown(bucket, earlier)
	if r2.Applied {
		t.Fatal("earlier cooldown should be rejected by join")
	}
	if r2.Actual.Unix() != later.UTC().Unix() {
		t.Fatalf("actual after rejection = %v, want original %v", r2.Actual, later.UTC())
	}

	evenLater := time.Now().Add(20 * time.Minute)
	r3 := p.applyBucketCooldown(bucket, evenLater)
	if !r3.Applied {
		t.Fatal("later cooldown should be applied")
	}
	if r3.Actual.Unix() != evenLater.UTC().Unix() {
		t.Fatalf("actual = %v, want %v", r3.Actual, evenLater.UTC())
	}
}

func TestObserve_EventCooldownUsesActualValue(t *testing.T) {
	acct := activeAccount("obs-1", "obs@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	longCooldown := time.Now().Add(30 * time.Minute)
	p.Observe(acct.ID, driver.Effect{
		Kind:          driver.EffectCooldown,
		Scope:         driver.EffectScopeBucket,
		CooldownUntil: longCooldown,
	})

	shortCooldown := time.Now().Add(5 * time.Minute)
	p.Observe(acct.ID, driver.Effect{
		Kind:          driver.EffectCooldown,
		Scope:         driver.EffectScopeBucket,
		CooldownUntil: shortCooldown,
	})

	recent := p.bus.Recent(1)
	if len(recent) == 0 {
		t.Fatal("no events")
	}
	evt := recent[0]
	if evt.Type != events.EventRateLimit {
		t.Fatalf("expected EventRateLimit, got %s", evt.Type)
	}
	if evt.CooldownUntil == nil {
		t.Fatal("event CooldownUntil is nil")
	}
	if evt.CooldownUntil.Unix() != longCooldown.UTC().Unix() {
		t.Fatalf("event CooldownUntil = %v, want original longer %v", *evt.CooldownUntil, longCooldown.UTC())
	}
	if !strings.Contains(evt.Message, longCooldown.UTC().Format(time.RFC3339)) {
		t.Fatalf("event Message should contain actual cooldown time, got %q", evt.Message)
	}
}

func TestObserve_BanEventIncludesCooldownAndBucketKey(t *testing.T) {
	acct := activeAccount("ban-1", "ban@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	until := time.Now().Add(1 * time.Hour)
	p.Observe(acct.ID, driver.Effect{
		Kind:          driver.EffectBlock,
		Scope:         driver.EffectScopeBucket,
		CooldownUntil: until,
		ErrorMessage:  "organization has been disabled",
	})

	recent := p.bus.Recent(1)
	if len(recent) == 0 {
		t.Fatal("no events")
	}
	evt := recent[0]
	if evt.Type != events.EventBan {
		t.Fatalf("expected EventBan, got %s", evt.Type)
	}
	if evt.CooldownUntil == nil {
		t.Fatal("ban event CooldownUntil is nil")
	}
	if evt.CooldownUntil.Unix() != until.UTC().Unix() {
		t.Fatalf("ban CooldownUntil = %v, want %v", *evt.CooldownUntil, until.UTC())
	}
	if !strings.Contains(evt.Message, "cooldown until") {
		t.Fatalf("ban Message missing cooldown info: %q", evt.Message)
	}
	if !strings.Contains(evt.Message, "organization has been disabled") {
		t.Fatalf("ban Message missing error text: %q", evt.Message)
	}
	if evt.BucketKey == "" {
		t.Fatal("ban event BucketKey is empty")
	}
}

// ---------------------------------------------------------------------------
// Edge cases: can an LLM reconstruct what happened from the event stream?
// ---------------------------------------------------------------------------

func TestObserve_BlockWithZeroCooldown(t *testing.T) {
	acct := activeAccount("zc-1", "zc@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	p.Observe(acct.ID, driver.Effect{
		Kind:         driver.EffectBlock,
		Scope:        driver.EffectScopeBucket,
		ErrorMessage: "unexpected block",
	})

	evt := p.bus.Recent(1)[0]
	if evt.Type != events.EventBan {
		t.Fatalf("expected EventBan, got %s", evt.Type)
	}
	if evt.CooldownUntil != nil {
		t.Fatalf("zero CooldownUntil should produce nil, got %v", *evt.CooldownUntil)
	}
	if !strings.Contains(evt.Message, "unexpected block") {
		t.Fatalf("error message lost: %q", evt.Message)
	}
}

func TestObserve_SharedBucket_OneBlocked(t *testing.T) {
	a1 := &domain.Account{
		ID: "sh-1", Email: "a@test.com", Provider: domain.ProviderClaude,
		Subject: "shared-org", Status: domain.StatusActive, Priority: 50,
	}
	a2 := &domain.Account{
		ID: "sh-2", Email: "b@test.com", Provider: domain.ProviderClaude,
		Subject: "shared-org", Status: domain.StatusActive, Priority: 50,
	}
	p := newTestPool(t, a1, a2)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	until := time.Now().Add(1 * time.Hour)
	p.Observe(a1.ID, driver.Effect{
		Kind:          driver.EffectBlock,
		Scope:         driver.EffectScopeBucket,
		CooldownUntil: until,
		ErrorMessage:  "org disabled",
	})

	evt := p.bus.Recent(1)[0]
	if evt.AccountID != a1.ID {
		t.Fatalf("event AccountID = %s, want %s", evt.AccountID, a1.ID)
	}
	if evt.BucketKey == "" {
		t.Fatal("BucketKey missing on shared-bucket ban event")
	}
	bucket := p.buckets[evt.BucketKey]
	if bucket == nil || bucket.CooldownUntil == nil {
		t.Fatal("shared bucket should have cooldown")
	}
}

func TestObserve_FullLifecycle_EventNarrative(t *testing.T) {
	acct := activeAccount("lc-1", "lc@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	cd1 := time.Now().Add(10 * time.Minute)
	p.Observe(acct.ID, driver.Effect{
		Kind: driver.EffectCooldown, Scope: driver.EffectScopeBucket,
		CooldownUntil: cd1,
	})

	cd2 := time.Now().Add(5 * time.Minute)
	p.Observe(acct.ID, driver.Effect{
		Kind: driver.EffectCooldown, Scope: driver.EffectScopeBucket,
		CooldownUntil: cd2,
	})

	cd3 := time.Now().Add(30 * time.Minute)
	p.Observe(acct.ID, driver.Effect{
		Kind: driver.EffectBlock, Scope: driver.EffectScopeBucket,
		CooldownUntil: cd3, ErrorMessage: "banned",
	})

	all := p.bus.Recent(10)
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}
	if all[0].Type != events.EventRateLimit {
		t.Fatalf("event[0] type = %s", all[0].Type)
	}
	if all[0].CooldownUntil.Unix() != cd1.UTC().Unix() {
		t.Fatalf("event[0] cooldown wrong")
	}
	if all[1].Type != events.EventRateLimit {
		t.Fatalf("event[1] type = %s", all[1].Type)
	}
	if all[1].CooldownUntil.Unix() != cd1.UTC().Unix() {
		t.Fatalf("event[1] should show actual (longer) cooldown cd1, not proposed cd2")
	}
	if all[2].Type != events.EventBan {
		t.Fatalf("event[2] type = %s", all[2].Type)
	}
	if all[2].CooldownUntil.Unix() != cd3.UTC().Unix() {
		t.Fatalf("event[2] cooldown wrong")
	}

	bk := all[0].BucketKey
	for i, ev := range all {
		if ev.BucketKey != bk {
			t.Fatalf("event[%d] BucketKey = %q, want %q", i, ev.BucketKey, bk)
		}
	}

	bucket := p.buckets[bk]
	past := time.Now().Add(-1 * time.Second)
	bucket.CooldownUntil = &past
	p.persistBucketLocked(bucket)
	p.cleanup()

	// Blocked accounts must NOT auto-recover. Only bucket cooldown should clear.
	if got := p.accounts[acct.ID]; got.Status != domain.StatusBlocked {
		t.Fatalf("blocked account should stay blocked, got %s", got.Status)
	}

	recent := p.bus.Recent(10)
	var recoveredBucket bool
	for _, ev := range recent {
		if ev.Type == events.EventRecover && ev.AccountID == acct.ID && ev.Message == "blocked account recovered" {
			t.Fatal("blocked account should not auto-recover")
		}
		if ev.Type == events.EventRecover && ev.BucketKey == bk && ev.Message == "cooldown expired" {
			recoveredBucket = true
		}
	}
	if !recoveredBucket {
		t.Fatal("missing 'cooldown expired' event")
	}
}

func TestObserve_UnknownAccount(t *testing.T) {
	p := newTestPool(t)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	p.Observe("nonexistent", driver.Effect{
		Kind: driver.EffectBlock, Scope: driver.EffectScopeBucket,
		CooldownUntil: time.Now().Add(1 * time.Hour),
		ErrorMessage:  "ghost account",
	})

	if len(p.bus.Recent(10)) != 0 {
		t.Fatal("events emitted for unknown account")
	}
}

func TestObserve_SuccessThenFailure_ContextSufficient(t *testing.T) {
	acct := activeAccount("sf-1", "sf@test.com")
	p := newTestPool(t, acct)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	for i := 0; i < 5; i++ {
		p.Observe(acct.ID, driver.Effect{Kind: driver.EffectSuccess, Scope: driver.EffectScopeBucket})
	}
	if len(p.bus.Recent(10)) != 0 {
		t.Fatal("success should not emit events")
	}

	until := time.Now().Add(1 * time.Hour)
	p.Observe(acct.ID, driver.Effect{
		Kind: driver.EffectBlock, Scope: driver.EffectScopeBucket,
		CooldownUntil: until, ErrorMessage: "sudden ban",
	})

	all := p.bus.Recent(10)
	if len(all) != 1 {
		t.Fatalf("expected 1 event, got %d", len(all))
	}
	evt := all[0]
	if evt.AccountID == "" || evt.BucketKey == "" || evt.CooldownUntil == nil || evt.Message == "" {
		t.Fatalf("ban event missing fields for self-contained diagnosis: %+v", evt)
	}
}

func TestObserve_InterleavedEffects_SameBucket(t *testing.T) {
	a1 := &domain.Account{
		ID: "il-1", Email: "x@test.com", Provider: domain.ProviderClaude,
		Subject: "same-org", Status: domain.StatusActive, Priority: 50,
	}
	a2 := &domain.Account{
		ID: "il-2", Email: "y@test.com", Provider: domain.ProviderClaude,
		Subject: "same-org", Status: domain.StatusActive, Priority: 50,
	}
	p := newTestPool(t, a1, a2)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	cd1 := time.Now().Add(10 * time.Minute)
	p.Observe(a1.ID, driver.Effect{
		Kind: driver.EffectCooldown, Scope: driver.EffectScopeBucket,
		CooldownUntil: cd1,
	})

	cd2 := time.Now().Add(3 * time.Minute)
	p.Observe(a2.ID, driver.Effect{
		Kind: driver.EffectAuthFail, Scope: driver.EffectScopeBucket,
		CooldownUntil: cd2,
	})

	all := p.bus.Recent(10)
	if len(all) != 2 {
		t.Fatalf("expected 2 events, got %d", len(all))
	}
	if all[0].AccountID != a1.ID {
		t.Fatalf("event[0] AccountID = %s, want %s", all[0].AccountID, a1.ID)
	}
	if all[1].AccountID != a2.ID {
		t.Fatalf("event[1] AccountID = %s, want %s", all[1].AccountID, a2.ID)
	}
	if all[0].BucketKey != all[1].BucketKey {
		t.Fatalf("shared bucket should have same key: %q vs %q", all[0].BucketKey, all[1].BucketKey)
	}
	if all[0].Type != events.EventRateLimit {
		t.Fatalf("event[0] type = %s", all[0].Type)
	}
	if all[1].Type != events.EventRefresh {
		t.Fatalf("event[1] type = %s", all[1].Type)
	}
	if all[1].CooldownUntil.Unix() != cd1.UTC().Unix() {
		t.Fatalf("event[1] CooldownUntil should be cd1 (join kept longer), got %v", *all[1].CooldownUntil)
	}
}

func TestApplyBucketCooldown_NilBucket(t *testing.T) {
	acct := activeAccount("nil-1", "nil@test.com")
	p := newTestPool(t, acct)

	result := p.applyBucketCooldown(nil, time.Now().Add(10*time.Minute))
	if result.Applied {
		t.Fatal("nil bucket should not apply cooldown")
	}
	if !result.Actual.IsZero() {
		t.Fatalf("nil bucket Actual should be zero, got %v", result.Actual)
	}
}

func TestCleanup_SharedBucket_BlockedStaysBlocked(t *testing.T) {
	active := &domain.Account{
		ID: "sbr-active", Email: "a@test.com", Provider: domain.ProviderClaude,
		Subject: "shared-org", Status: domain.StatusActive, Priority: 50,
	}
	blocked := &domain.Account{
		ID: "sbr-blocked", Email: "b@test.com", Provider: domain.ProviderClaude,
		Subject: "shared-org", Status: domain.StatusBlocked, Priority: 50,
		ErrorMessage: "banned",
	}
	p := newTestPool(t, active, blocked)
	p.SetDrivers(map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderClaude: testDriver,
	})

	bucketKey := testDriver.BucketKey(active)
	bucket := p.buckets[bucketKey]
	if bucket == nil {
		t.Fatal("bucket not found")
	}
	past := time.Now().Add(-1 * time.Second)
	bucket.CooldownUntil = &past
	p.persistBucketLocked(bucket)

	p.cleanup()

	// Blocked account must stay blocked — no auto-recovery.
	if p.accounts["sbr-blocked"].Status != domain.StatusBlocked {
		t.Fatalf("blocked account should stay blocked, got status %s", p.accounts["sbr-blocked"].Status)
	}
	if p.accounts["sbr-blocked"].ErrorMessage != "banned" {
		t.Fatalf("error message should be preserved, got %q", p.accounts["sbr-blocked"].ErrorMessage)
	}

	// Bucket cooldown should still clear so other accounts in the bucket can be used.
	bucket = p.buckets[bucketKey]
	if bucket == nil {
		t.Fatal("bucket gone after cleanup")
	}
	if bucket.CooldownUntil != nil {
		t.Fatalf("bucket cooldown should be nil, got %v", *bucket.CooldownUntil)
	}

	all := p.bus.Recent(10)
	hasExpiry := false
	for _, ev := range all {
		if ev.Type == events.EventRecover && ev.AccountID == "sbr-blocked" {
			t.Fatal("blocked account should not emit recovery event")
		}
		if ev.Type == events.EventRecover && ev.Message == "cooldown expired" {
			hasExpiry = true
		}
	}
	if !hasExpiry {
		t.Fatal("missing 'cooldown expired' event")
	}
}
