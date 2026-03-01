package pool

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/store"
)

func newTestPool(t *testing.T, accounts ...*domain.Account) *Pool {
	t.Helper()
	ms := store.NewMockStore()
	bus := events.NewBus(100)
	p := &Pool{
		accounts:      make(map[string]*domain.Account),
		store:         ms,
		bus:           bus,
		pauses:        ErrorPauses{Pause401: 30 * time.Minute, Pause401Refresh: 30 * time.Second, Pause403: 10 * time.Minute, Pause429: 60 * time.Second, Pause529: 5 * time.Minute},
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
		ID:          id,
		Email:       email,
		Provider:    domain.ProviderClaude,
		Status:      domain.StatusActive,
		Schedulable: true,
		Priority:    50,
	}
}

// Test 1: Pick never returns unavailable accounts
func TestPick_NeverReturnsUnavailable(t *testing.T) {
	blocked := &domain.Account{ID: "b", Email: "b@x", Provider: domain.ProviderClaude, Status: domain.StatusBlocked, Schedulable: true, Priority: 99}
	unschedulable := &domain.Account{ID: "u", Email: "u@x", Provider: domain.ProviderClaude, Status: domain.StatusActive, Schedulable: false, Priority: 99}
	overloaded := activeAccount("o", "o@x")
	future := time.Now().Add(1 * time.Hour)
	overloaded.OverloadedUntil = &future
	opusLimited := activeAccount("op", "op@x")
	opusEnd := time.Now().Add(1 * time.Hour)
	opusLimited.OpusRateLimitEndAt = &opusEnd
	good := activeAccount("g", "g@x")
	good.Priority = 99 // highest priority to ensure deterministic selection

	p := newTestPool(t, blocked, unschedulable, overloaded, opusLimited, good)

	// Pick should return "good" (highest priority, others filtered)
	acct, err := p.Pick(domain.ProviderClaude, nil, false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "g" {
		t.Fatalf("expected g, got %s", acct.ID)
	}

	// With opus, opusLimited should also be excluded, still get "good"
	acct, err = p.Pick(domain.ProviderClaude, nil, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "g" {
		t.Fatalf("expected g with opus, got %s", acct.ID)
	}

	// Exclude "good" with opus → no accounts available (opusLimited blocked by opus check)
	_, err = p.Pick(domain.ProviderClaude, []string{"g"}, true, "")
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
	if acct.OverloadedUntil == nil || !acct.OverloadedUntil.Equal(long.UTC()) {
		t.Fatal("long cooldown should be set")
	}

	short := time.Now().Add(5 * time.Minute)
	p.applyCooldown(acct, short)
	if !acct.OverloadedUntil.Equal(long.UTC()) {
		t.Fatal("short cooldown should not overwrite long cooldown")
	}

	longer := time.Now().Add(2 * time.Hour)
	p.applyCooldown(acct, longer)
	if !acct.OverloadedUntil.Equal(longer.UTC()) {
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
			p.Observe(UpstreamResult{
				AccountID:  "a",
				StatusCode: 529,
				Model:      "sonnet",
			})
		}(i)
	}
	wg.Wait()

	p.mu.RLock()
	defer p.mu.RUnlock()
	a := p.accounts["a"]
	if a.OverloadedUntil == nil {
		t.Fatal("overloadedUntil should be set after 529s")
	}
	if a.Schedulable {
		t.Fatal("schedulable should be false after 529")
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

	p.Observe(UpstreamResult{
		AccountID:  "a",
		StatusCode: 401,
		Model:      "sonnet",
	})

	// Verify account is NOT set to StatusError (key change from old behavior)
	p.mu.RLock()
	a := p.accounts["a"]
	if a.Status == domain.StatusError {
		t.Fatal("401 should NOT set StatusError anymore")
	}
	if a.Schedulable {
		t.Fatal("schedulable should be false after 401")
	}
	if a.OverloadedUntil == nil {
		t.Fatal("overloadedUntil should be set with Pause401Refresh")
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

// Test 5: Status transitions for various codes
func TestObserve_StatusTransitions(t *testing.T) {
	t.Run("529_cooldown", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe(UpstreamResult{AccountID: "a", StatusCode: 529})
		p.mu.RLock()
		a := p.accounts["a"]
		if a.Schedulable {
			t.Fatal("should be unschedulable after 529")
		}
		if a.OverloadedAt == nil {
			t.Fatal("overloadedAt should be set")
		}
		p.mu.RUnlock()
	})

	t.Run("429_cooldown_and_opus", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		h := http.Header{}
		h.Set("anthropic-ratelimit-unified-reset", time.Now().Add(10*time.Minute).Format(time.RFC3339))
		p.Observe(UpstreamResult{AccountID: "a", StatusCode: 429, Headers: h, IsOpus: true})
		p.mu.RLock()
		a := p.accounts["a"]
		if a.Schedulable {
			t.Fatal("should be unschedulable after 429")
		}
		if a.OpusRateLimitEndAt == nil {
			t.Fatal("OpusRateLimitEndAt should be set for opus 429")
		}
		p.mu.RUnlock()
	})

	t.Run("403_ban_blocked", func(t *testing.T) {
		acct := activeAccount("a", "a@x")
		p := newTestPool(t, acct)
		p.Observe(UpstreamResult{AccountID: "a", StatusCode: 403, ErrBody: []byte("organization has been disabled")})
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
		p.Observe(UpstreamResult{AccountID: "a", StatusCode: 403, ErrBody: []byte("some other error")})
		p.mu.RLock()
		a := p.accounts["a"]
		if a.Status != domain.StatusActive {
			t.Fatalf("non-ban 403 should keep status active, got %s", a.Status)
		}
		if a.Schedulable {
			t.Fatal("should be unschedulable after non-ban 403")
		}
		p.mu.RUnlock()
	})
}

// Test 6: StoreTokens restores account
func TestStoreTokens_RestoresAccount(t *testing.T) {
	acct := activeAccount("a", "a@x")
	acct.Status = domain.StatusError
	acct.Schedulable = false
	future := time.Now().Add(1 * time.Hour)
	acct.OverloadedUntil = &future
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
	if !a.Schedulable {
		t.Fatal("should be schedulable after StoreTokens")
	}
	if a.OverloadedUntil != nil {
		t.Fatal("overloadedUntil should be cleared")
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
	acct, err := p.Pick(domain.ProviderClaude, nil, false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "a3" {
		t.Fatalf("expected a3 (highest priority), got %s", acct.ID)
	}

	// Exclude a3, then a2 should win (same priority, older lastUsedAt)
	acct, err = p.Pick(domain.ProviderClaude, []string{"a3"}, false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acct.ID != "a2" {
		t.Fatalf("expected a2 (older lastUsedAt), got %s", acct.ID)
	}
}
