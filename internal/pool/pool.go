package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/store"
)

// SessionBinding holds the account ID bound to a session UUID.
type SessionBinding struct {
	AccountID  string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// Pool is the central authority for account state.
// All account reads and writes go through Pool. Store.SaveAccount is only
// called from within Pool under the mu lock.
type Pool struct {
	mu       sync.RWMutex
	accounts map[string]*domain.Account
	store    store.Store
	bus      *events.Bus
	pauses   driver.ErrorPauses

	// Ephemeral in-memory state
	sessions      *store.TTLMap[SessionBinding]
	stainless     *store.TTLMap[string] // accountID -> JSON-encoded stainless headers
	oauthSessions *store.TTLMap[string] // state -> redirect URL or other data
	refreshLocks  *store.TTLMap[string] // accountID -> lockID

	onAuthFailure func(accountID string) // called on 401 to trigger background token refresh

	// Driver integration
	drivers map[domain.Provider]driver.Driver
}

// SetOnAuthFailure registers a callback invoked when a 401 is observed.
func (p *Pool) SetOnAuthFailure(fn func(accountID string)) {
	p.onAuthFailure = fn
}

// New creates a Pool, loading all accounts from the store.
func New(s store.Store, bus *events.Bus, pauses driver.ErrorPauses) (*Pool, error) {
	p := &Pool{
		accounts:      make(map[string]*domain.Account),
		store:         s,
		bus:           bus,
		pauses:        pauses,
		sessions:      store.NewTTLMap[SessionBinding](),
		stainless:     store.NewTTLMap[string](),
		oauthSessions: store.NewTTLMap[string](),
		refreshLocks:  store.NewTTLMap[string](),
	}

	accounts, err := s.ListAccounts(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load accounts: %w", err)
	}
	for _, acct := range accounts {
		acct.HydrateRuntime()
		p.accounts[acct.ID] = acct
	}
	slog.Info("pool loaded", "accounts", len(p.accounts))
	return p, nil
}

// ---------------------------------------------------------------------------
// Read operations
// ---------------------------------------------------------------------------

// Get returns a copy of the account (nil if not found).
func (p *Pool) Get(id string) *domain.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[id]
	if !ok {
		return nil
	}
	copy := *acct
	return &copy
}

// List returns copies of all accounts.
func (p *Pool) List() []*domain.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*domain.Account, 0, len(p.accounts))
	for _, acct := range p.accounts {
		copy := *acct
		result = append(result, &copy)
	}
	return result
}

// ---------------------------------------------------------------------------
// Pick (scheduler replacement)
// ---------------------------------------------------------------------------

// Pick selects the best available account for a request.
// boundAccountID is from API key binding or session binding.
func (p *Pool) isAvailable(acct *domain.Account, drv driver.Driver, model string, now time.Time) bool {
	if acct.Status != domain.StatusActive {
		return false
	}
	if acct.CooldownUntil != nil && now.Before(*acct.CooldownUntil) {
		return false
	}
	if !drv.CanServe(json.RawMessage(acct.ProviderStateJSON), model, now) {
		return false
	}
	return true
}

func (p *Pool) matchesProvider(acct *domain.Account, provider domain.Provider) bool {
	return acct.Provider == provider
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// ClearOverload is an explicit admin reset that clears overload state.
// This is one of the two legitimate ways to clear cooldowns (the other being
// RunCleanup expiry). It does NOT violate the monotonic cooldown invariant
// because it represents deliberate admin intent, not automated shortening.
func (p *Pool) ClearCooldown(accountID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok || acct.CooldownUntil == nil {
		return
	}
	acct.CooldownUntil = nil
	p.persistLocked(acct)
	p.bus.Publish(events.Event{
		Type: events.EventRecover, AccountID: acct.ID,
		Message: "admin cleared cooldown",
	})
	slog.Info("admin cleared cooldown", "accountId", acct.ID)
}

// ---------------------------------------------------------------------------
// applyCooldown — monotonic guarantee [invariant 2]
// ---------------------------------------------------------------------------

// applyCooldown sets the cooldown_until timestamp.
// If the existing cooldown is longer, the proposed one is ignored.
func (p *Pool) applyCooldown(acct *domain.Account, proposed time.Time) {
	if acct.CooldownUntil != nil && acct.CooldownUntil.After(proposed) {
		return // existing cooldown is longer, keep it
	}
	until := proposed.UTC()
	acct.CooldownUntil = &until
}

// ---------------------------------------------------------------------------
// Update (admin operations)
// ---------------------------------------------------------------------------

// Update applies a mutation function to an account under the pool lock,
// then persists it. Used by admin operations (priority, status, email, etc.).
func (p *Pool) Update(accountID string, fn func(*domain.Account)) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}
	fn(acct)
	p.persistLocked(acct)
	return nil
}

// Add creates a new account in the pool and persists it.
func (p *Pool) Add(acct *domain.Account) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct.PersistRuntime()
	if err := p.store.SaveAccount(context.Background(), acct); err != nil {
		return err
	}
	acct.HydrateRuntime()
	p.accounts[acct.ID] = acct
	return nil
}

// Delete removes an account from the pool and the store.
func (p *Pool) Delete(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.store.DeleteAccount(context.Background(), accountID); err != nil {
		return err
	}
	delete(p.accounts, accountID)
	return nil
}

// StoreTokens encrypts and stores new tokens after refresh.
// Caller is responsible for encrypting tokens before passing them.
func (p *Pool) StoreTokens(accountID, accessTokenEnc, refreshTokenEnc string, expiresAt int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}
	acct.AccessTokenEnc = accessTokenEnc
	acct.RefreshTokenEnc = refreshTokenEnc
	acct.ExpiresAt = expiresAt
	now := time.Now().UTC()
	acct.LastRefreshAt = &now
	acct.Status = domain.StatusActive
	acct.ErrorMessage = ""
	acct.CooldownUntil = nil
	p.persistLocked(acct)
	return nil
}

// ---------------------------------------------------------------------------
// Persist (called under mu.Lock)
// ---------------------------------------------------------------------------

func (p *Pool) persistLocked(acct *domain.Account) {
	acct.PersistRuntime()
	if err := p.store.SaveAccount(context.Background(), acct); err != nil {
		slog.Error("pool persist failed", "accountId", acct.ID, "error", err)
	}
}

// ---------------------------------------------------------------------------
// RunCleanup — periodic recovery
// ---------------------------------------------------------------------------

// RunCleanup periodically checks for accounts that should be restored.
func (p *Pool) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// Also run TTL map cleanup
	ttlTicker := time.NewTicker(30 * time.Second)
	defer ttlTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanup()
		case <-ttlTicker.C:
			p.sessions.Cleanup()
			p.stainless.Cleanup()
			p.oauthSessions.Cleanup()
			p.refreshLocks.Cleanup()
		}
	}
}

func (p *Pool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	for _, acct := range p.accounts {
		changed := false

		// General cooldown recovery
		if acct.CooldownUntil != nil && now.After(*acct.CooldownUntil) {
			if acct.Status != domain.StatusBlocked {
				acct.CooldownUntil = nil
				changed = true
				p.bus.Publish(events.Event{
					Type: events.EventRecover, AccountID: acct.ID,
					Message: "cooldown expired",
				})
				slog.Info("account cooldown expired", "accountId", acct.ID)
			}
		}

		// Blocked account recovery (auto-unblock after pause expires)
		if acct.Status == domain.StatusBlocked && acct.CooldownUntil != nil && now.After(*acct.CooldownUntil) {
			acct.Status = domain.StatusActive
			acct.ErrorMessage = ""
			acct.CooldownUntil = nil
			changed = true
			p.bus.Publish(events.Event{
				Type: events.EventRecover, AccountID: acct.ID,
				Message: "blocked account recovered",
			})
			slog.Info("blocked account recovered", "accountId", acct.ID)
		}

		// Enforce exhausted cooldown on active accounts without an explicit cooldown.
		if acct.Status == domain.StatusActive && acct.CooldownUntil == nil {
			if cooldownUntil := p.computeExhaustedCooldown(acct, now.Unix()); cooldownUntil > 0 {
				resetTime := time.Unix(cooldownUntil, 0).UTC()
				p.applyCooldown(acct, resetTime)
				changed = true
				slog.Warn("enforced cooldown on exhausted account", "account", acct.Email, "until", resetTime)
			}
		}

		if changed {
			p.persistLocked(acct)
		}
	}
}

func (p *Pool) computeExhaustedCooldown(acct *domain.Account, now int64) int64 {
	if p.drivers == nil {
		return 0
	}
	drv, ok := p.drivers[acct.Provider]
	if !ok {
		return 0
	}
	t := drv.ComputeExhaustedCooldown(json.RawMessage(acct.ProviderStateJSON), time.Unix(now, 0))
	if !t.IsZero() {
		return t.Unix()
	}
	return 0
}

// ---------------------------------------------------------------------------
// Session bindings (in-memory with TTL)
// ---------------------------------------------------------------------------

// GetSessionBinding returns the bound account ID for a session UUID.
func (p *Pool) GetSessionBinding(sessionUUID string) (string, bool) {
	b, ok := p.sessions.Get(sessionUUID)
	if !ok {
		return "", false
	}
	return b.AccountID, true
}

// SetSessionBinding binds a session UUID to an account with a TTL.
func (p *Pool) SetSessionBinding(sessionUUID, accountID string, ttl time.Duration) {
	now := time.Now()
	p.sessions.Set(sessionUUID, SessionBinding{
		AccountID:  accountID,
		CreatedAt:  now,
		LastUsedAt: now,
	}, ttl)
}

// RenewSessionBinding updates the TTL and last-used time.
func (p *Pool) RenewSessionBinding(sessionUUID string, ttl time.Duration) {
	p.sessions.Update(sessionUUID, func(b *SessionBinding) {
		b.LastUsedAt = time.Now()
	}, ttl)
}

// ListSessionBindingsForAccount returns all session bindings for a given account.
func (p *Pool) ListSessionBindingsForAccount(accountID string) []domain.SessionBindingInfo {
	entries := p.sessions.Entries()
	var result []domain.SessionBindingInfo
	for _, e := range entries {
		if e.Value.AccountID == accountID {
			result = append(result, domain.SessionBindingInfo{
				SessionUUID: e.Key,
				AccountID:   e.Value.AccountID,
				CreatedAt:   e.Value.CreatedAt.Format(time.RFC3339),
				LastUsedAt:  e.Value.LastUsedAt.Format(time.RFC3339),
				ExpiresAt:   e.ExpiresAt,
			})
		}
	}
	return result
}

// UnbindSession removes a session binding.
func (p *Pool) UnbindSession(sessionUUID string) {
	p.sessions.Delete(sessionUUID)
}

// ---------------------------------------------------------------------------
// Stainless headers (in-memory, per-account)
// ---------------------------------------------------------------------------

// GetStainless returns the stored stainless headers JSON for an account.
func (p *Pool) GetStainless(accountID string) (string, bool) {
	return p.stainless.Get(accountID)
}

// SetStainlessNX sets stainless headers only if not already present.
// Returns true if the value was set (first writer wins).
func (p *Pool) SetStainlessNX(accountID, headersJSON string) bool {
	if _, ok := p.stainless.Get(accountID); ok {
		return false
	}
	p.stainless.Set(accountID, headersJSON, 24*time.Hour)
	return true
}

// ---------------------------------------------------------------------------
// OAuth sessions (in-memory, for PKCE flow)
// ---------------------------------------------------------------------------

// SetOAuthSession stores a transient OAuth session.
func (p *Pool) SetOAuthSession(state, data string, ttl time.Duration) {
	p.oauthSessions.Set(state, data, ttl)
}

// GetDelOAuthSession atomically retrieves and deletes an OAuth session.
func (p *Pool) GetDelOAuthSession(state string) (string, bool) {
	return p.oauthSessions.GetAndDelete(state)
}

// ---------------------------------------------------------------------------
// Refresh locks (in-memory, per-account mutex)
// ---------------------------------------------------------------------------

// AcquireRefreshLock attempts to acquire a per-account refresh lock.
func (p *Pool) AcquireRefreshLock(accountID, lockID string) bool {
	return p.refreshLocks.SetNX(accountID, lockID, 30*time.Second)
}

// ReleaseRefreshLock releases a per-account refresh lock if still held by lockID.
func (p *Pool) ReleaseRefreshLock(accountID, lockID string) {
	p.refreshLocks.DeleteIf(accountID, func(held string) bool {
		return held == lockID
	})
}

// MarkError sets the account status to error with a message.
func (p *Pool) MarkError(accountID, msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return
	}
	acct.Status = domain.StatusError
	acct.ErrorMessage = msg
	p.persistLocked(acct)
}

// ---------------------------------------------------------------------------
// Driver integration
// ---------------------------------------------------------------------------

// SetDrivers injects provider drivers into the pool.
func (p *Pool) SetDrivers(drivers map[domain.Provider]driver.Driver) {
	p.drivers = drivers
}

// Observe applies a provider-agnostic Effect to an account.
func (p *Pool) Observe(accountID string, effect driver.Effect) {
	p.mu.Lock()
	defer p.mu.Unlock()

	acct, ok := p.accounts[accountID]
	if !ok {
		return
	}

	switch effect.Kind {
	case driver.EffectSuccess:
		now := time.Now().UTC()
		acct.LastUsedAt = &now

	case driver.EffectCooldown:
		p.applyCooldown(acct, effect.CooldownUntil)
		p.bus.Publish(events.Event{
			Type: events.EventRateLimit, AccountID: acct.ID,
			Message: fmt.Sprintf("cooldown until %s", effect.CooldownUntil.Format(time.RFC3339)),
		})

	case driver.EffectOverload:
		p.applyCooldown(acct, effect.CooldownUntil)
		p.bus.Publish(events.Event{
			Type: events.EventOverload, AccountID: acct.ID,
			Message: fmt.Sprintf("overloaded, cooldown until %s", effect.CooldownUntil.Format(time.RFC3339)),
		})

	case driver.EffectBlock:
		acct.Status = domain.StatusBlocked
		acct.ErrorMessage = effect.ErrorMessage
		p.applyCooldown(acct, effect.CooldownUntil)
		p.bus.Publish(events.Event{
			Type: events.EventBan, AccountID: acct.ID,
			Message: effect.ErrorMessage,
		})

	case driver.EffectAuthFail:
		p.applyCooldown(acct, effect.CooldownUntil)
		p.bus.Publish(events.Event{
			Type: events.EventRefresh, AccountID: acct.ID,
			Message: "auth failed, background refresh triggered",
		})
		if p.onAuthFailure != nil {
			go p.onAuthFailure(acct.ID)
		}
	}

	// Update provider state JSON if provided
	if len(effect.UpdatedState) > 0 {
		acct.ProviderStateJSON = string(effect.UpdatedState)
	}

	p.persistLocked(acct)
}

// FindBySubject returns an account matching provider+subject, or nil.
func (p *Pool) FindBySubject(provider domain.Provider, subject string) *domain.Account {
	if subject == "" {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, acct := range p.accounts {
		if acct.Provider == provider && acct.Subject == subject {
			copy := *acct
			return &copy
		}
	}
	return nil
}

// IsAvailableFor reports whether the account can serve the given model now.
func (p *Pool) IsAvailableFor(accountID string, drv driver.Driver, model string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return false
	}
	return p.isAvailable(acct, drv, model, time.Now())
}

// Pick selects the best available account using the driver's AutoPriority.
func (p *Pool) Pick(drv driver.Driver, excludeIDs []string, model string, boundAccountID string) (*domain.Account, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	provider := drv.Provider()
	now := time.Now()

	// 1. Bound account — highest priority
	if boundAccountID != "" {
		acct, ok := p.accounts[boundAccountID]
		if ok && p.isAvailable(acct, drv, model, now) {
			copy := *acct
			return &copy, nil
		}
		if ok {
			return nil, fmt.Errorf("bound account %s unavailable (status=%s)", boundAccountID, acct.Status)
		}
	}

	// 2. Pool selection
	type scored struct {
		acct     *domain.Account
		priority int
	}
	var candidates []scored
	for _, acct := range p.accounts {
		if slices.Contains(excludeIDs, acct.ID) {
			continue
		}
		if !p.matchesProvider(acct, provider) {
			continue
		}
		if !p.isAvailable(acct, drv, model, now) {
			continue
		}
		pri := acct.Priority
		if acct.PriorityMode == "auto" {
			pri = drv.AutoPriority(json.RawMessage(acct.ProviderStateJSON))
		}
		candidates = append(candidates, scored{acct: acct, priority: pri})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available accounts")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority > candidates[j].priority
		}
		ti := timeOrZero(candidates[i].acct.LastUsedAt)
		tj := timeOrZero(candidates[j].acct.LastUsedAt)
		return ti.Before(tj)
	})

	selected := candidates[0].acct
	copy := *selected
	slog.Debug("account selected (driver)", "accountId", selected.ID, "email", selected.Email,
		"priority", candidates[0].priority, "mode", selected.PriorityMode)
	return &copy, nil
}

// BindStainlessFromRequest captures and replays stainless headers.
// Bound keys are captured once per account; dynamic keys pass through each request.
func (p *Pool) BindStainlessFromRequest(accountID string, reqHeaders http.Header, outHeaders http.Header) {
	stored, ok := p.GetStainless(accountID)

	if ok {
		var headers map[string]string
		if json.Unmarshal([]byte(stored), &headers) == nil {
			for k, v := range headers {
				outHeaders.Set(k, v)
			}
		}
	} else {
		captured := make(map[string]string)
		for _, key := range boundStainlessKeys {
			if v := reqHeaders.Get(key); v != "" {
				captured[key] = v
				outHeaders.Set(key, v)
			}
		}
		if len(captured) > 0 {
			data, _ := json.Marshal(captured)
			if !p.SetStainlessNX(accountID, string(data)) {
				// Another request beat us — re-read and apply
				if reread, ok := p.GetStainless(accountID); ok {
					var headers map[string]string
					if json.Unmarshal([]byte(reread), &headers) == nil {
						for k, v := range headers {
							outHeaders.Set(k, v)
						}
					}
				}
			}
		}
	}

	// Always pass through dynamic headers
	for _, key := range passthroughStainlessKeys {
		if v := reqHeaders.Get(key); v != "" {
			outHeaders.Set(key, v)
		}
	}
}

var boundStainlessKeys = []string{
	"x-stainless-os",
	"x-stainless-arch",
	"x-stainless-runtime",
	"x-stainless-runtime-version",
	"x-stainless-lang",
	"x-stainless-package-version",
}

var passthroughStainlessKeys = []string{
	"x-stainless-retry-count",
	"x-stainless-read-timeout",
}
