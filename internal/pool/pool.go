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

// ErrorPauses is an alias for driver.ErrorPauses for backward compatibility.
type ErrorPauses = driver.ErrorPauses

// Pool is the central authority for account state.
// All account reads and writes go through Pool. Store.SaveAccount is only
// called from within Pool under the mu lock.
type Pool struct {
	mu       sync.RWMutex
	accounts map[string]*domain.Account
	store    store.Store
	bus      *events.Bus
	pauses   ErrorPauses

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
func New(s store.Store, bus *events.Bus, pauses ErrorPauses) (*Pool, error) {
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
	migrated := p.migrateProviderState()
	slog.Info("pool loaded", "accounts", len(p.accounts), "migrated", migrated)
	return p, nil
}

// migrateProviderState backfills ProviderStateJSON and Subject from old fields
// for pre-migration accounts. Runs once at startup. Returns count of migrated accounts.
func (p *Pool) migrateProviderState() int {
	count := 0
	for _, acct := range p.accounts {
		changed := false

		// Backfill ProviderStateJSON from old rate-limit fields
		if acct.ProviderStateJSON == "" || acct.ProviderStateJSON == "{}" {
			var stateJSON []byte
			if acct.Provider == domain.ProviderCodex {
				stateJSON, _ = json.Marshal(map[string]interface{}{
					"primary_util":    acct.CodexPrimaryUtil,
					"primary_reset":   acct.CodexPrimaryReset,
					"secondary_util":  acct.CodexSecondaryUtil,
					"secondary_reset": acct.CodexSecondaryReset,
				})
			} else {
				stateJSON, _ = json.Marshal(map[string]interface{}{
					"five_hour_status": acct.FiveHourStatus,
					"five_hour_util":   acct.FiveHourUtil,
					"five_hour_reset":  acct.FiveHourReset,
					"seven_day_util":   acct.SevenDayUtil,
					"seven_day_reset":  acct.SevenDayReset,
				})
			}
			acct.ProviderStateJSON = string(stateJSON)
			changed = true
		}

		// Backfill Subject from ExtInfo
		if acct.Subject == "" && acct.ExtInfo != nil {
			switch acct.Provider {
			case domain.ProviderClaude:
				if orgUUID, ok := acct.ExtInfo["orgUUID"].(string); ok && orgUUID != "" {
					acct.Subject = orgUUID
					changed = true
				}
			case domain.ProviderCodex:
				if chatgptID, ok := acct.ExtInfo["chatgptAccountId"].(string); ok && chatgptID != "" {
					acct.Subject = chatgptID
					changed = true
				}
			}
		}

		if changed {
			p.persistLocked(acct)
			count++
			slog.Info("migrated account provider state", "id", acct.ID, "email", acct.Email, "provider", acct.Provider)
		}
	}
	return count
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
func (p *Pool) isAvailable(acct *domain.Account, isOpus bool) bool {
	if acct.Status != domain.StatusActive {
		return false
	}
	if !acct.Schedulable {
		return false
	}
	if acct.OverloadedUntil != nil && time.Now().Before(*acct.OverloadedUntil) {
		return false
	}
	// Opus rate limit check (Claude only)
	if acct.Provider != domain.ProviderCodex && isOpus && acct.OpusRateLimitEndAt != nil {
		if time.Now().Before(*acct.OpusRateLimitEndAt) {
			return false
		}
	}
	return true
}

func (p *Pool) matchesProvider(acct *domain.Account, provider domain.Provider) bool {
	if provider == "" {
		provider = domain.ProviderClaude
	}
	ap := acct.Provider
	if ap == "" {
		ap = domain.ProviderClaude
	}
	return ap == provider
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
func (p *Pool) ClearOverload(accountID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok || acct.OverloadedUntil == nil {
		return
	}
	acct.OverloadedUntil = nil
	acct.OverloadedAt = nil
	acct.Schedulable = true
	p.persistLocked(acct)
	p.bus.Publish(events.Event{
		Type: events.EventRecover, AccountID: acct.ID,
		Message: "admin cleared overload",
	})
	slog.Info("admin cleared overload", "accountId", acct.ID)
}

// ---------------------------------------------------------------------------
// applyCooldown — monotonic guarantee [invariant 2]
// ---------------------------------------------------------------------------

// applyCooldown sets the cooldown (overloadedUntil) and marks schedulable=false.
// If the existing cooldown is longer, the proposed one is ignored.
func (p *Pool) applyCooldown(acct *domain.Account, proposed time.Time) {
	if acct.OverloadedUntil != nil && acct.OverloadedUntil.After(proposed) {
		return // existing cooldown is longer, keep it
	}
	until := proposed.UTC()
	acct.OverloadedUntil = &until
	acct.Schedulable = false
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
	// Clear temporary cooldown markers after a successful refresh
	acct.OverloadedAt = nil
	acct.OverloadedUntil = nil
	acct.Schedulable = true
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

		// Overloaded recovery
		if acct.OverloadedUntil != nil && now.After(*acct.OverloadedUntil) {
			if acct.Status != domain.StatusBlocked {
				acct.Schedulable = true
				acct.OverloadedAt = nil
				acct.OverloadedUntil = nil
				acct.FiveHourStatus = ""
				changed = true
				p.bus.Publish(events.Event{
					Type: events.EventRecover, AccountID: acct.ID,
					Message: "recovered from overload",
				})
				slog.Info("account recovered from overload", "accountId", acct.ID)
			}
		}

		// Opus rate limit recovery
		if acct.OpusRateLimitEndAt != nil && now.After(*acct.OpusRateLimitEndAt) {
			acct.OpusRateLimitEndAt = nil
			changed = true
			slog.Info("account Opus rate limit cleared", "accountId", acct.ID)
		}

		// Blocked account recovery (auto-unblock after pause expires)
		if acct.Status == domain.StatusBlocked && acct.OverloadedUntil != nil && now.After(*acct.OverloadedUntil) {
			acct.Status = domain.StatusActive
			acct.ErrorMessage = ""
			acct.Schedulable = true
			acct.OverloadedUntil = nil
			changed = true
			p.bus.Publish(events.Event{
				Type: events.EventRecover, AccountID: acct.ID,
				Message: "blocked account recovered",
			})
			slog.Info("blocked account recovered", "accountId", acct.ID)
		}

		// Self-heal stale schedulable=false on active accounts
		if acct.Status == domain.StatusActive && !acct.Schedulable {
			blockedByOverload := acct.OverloadedUntil != nil && now.Before(*acct.OverloadedUntil)
			if !blockedByOverload {
				acct.Schedulable = true
				changed = true
				slog.Info("account schedulable flag self-healed", "accountId", acct.ID)
			}
		}

		// Enforce exhausted cooldown on schedulable accounts
		if acct.Schedulable && acct.Status == domain.StatusActive {
			if cooldownUntil := p.computeExhaustedCooldown(acct, now.Unix()); cooldownUntil > 0 {
				resetTime := time.Unix(cooldownUntil, 0).UTC()
				p.applyCooldown(acct, resetTime)
				nowUTC := now.UTC()
				acct.OverloadedAt = &nowUTC
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

// ---------------------------------------------------------------------------
// Token access (for oauth package)
// ---------------------------------------------------------------------------

// GetTokenInfo returns the encrypted tokens and expiry for an account.
func (p *Pool) GetTokenInfo(accountID string) (refreshEnc, accessEnc string, expiresAt int64, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, exists := p.accounts[accountID]
	if !exists {
		return "", "", 0, false
	}
	return acct.RefreshTokenEnc, acct.AccessTokenEnc, acct.ExpiresAt, true
}

// GetProvider returns the provider for an account.
func (p *Pool) GetProvider(accountID string) (domain.Provider, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return "", false
	}
	return acct.Provider, true
}

// GetProxy returns the proxy config for an account.
func (p *Pool) GetProxy(accountID string) *domain.ProxyConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return nil
	}
	return acct.Proxy
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
		now := time.Now().UTC()
		acct.OverloadedAt = &now
		acct.RateLimitedAt = &now
		if effect.IsOpusLimit {
			acct.OpusRateLimitEndAt = &effect.OpusResetAt
		}
		p.bus.Publish(events.Event{
			Type: events.EventRateLimit, AccountID: acct.ID,
			Message: fmt.Sprintf("cooldown until %s", effect.CooldownUntil.Format(time.RFC3339)),
		})

	case driver.EffectOverload:
		p.applyCooldown(acct, effect.CooldownUntil)
		now := time.Now().UTC()
		acct.OverloadedAt = &now
		p.bus.Publish(events.Event{
			Type: events.EventOverload, AccountID: acct.ID,
			Message: fmt.Sprintf("overloaded, cooldown until %s", effect.CooldownUntil.Format(time.RFC3339)),
		})

	case driver.EffectBlock:
		acct.Status = domain.StatusBlocked
		acct.ErrorMessage = effect.ErrorMessage
		acct.Schedulable = false
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

// FindByExtInfoKey returns an account matching provider + ExtInfo[key]==value, or nil.
// Used as fallback dedup for pre-migration accounts that have subject=''.
func (p *Pool) FindByExtInfoKey(provider domain.Provider, key, value string) *domain.Account {
	if value == "" {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, acct := range p.accounts {
		if acct.Provider != provider {
			continue
		}
		if v, ok := acct.ExtInfo[key].(string); ok && v == value {
			copy := *acct
			return &copy
		}
	}
	return nil
}

// Pick selects the best available account using the driver's AutoPriority.
func (p *Pool) Pick(drv driver.Driver, excludeIDs []string, isOpus bool, boundAccountID string) (*domain.Account, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	provider := drv.Provider()

	// 1. Bound account — highest priority
	if boundAccountID != "" {
		acct, ok := p.accounts[boundAccountID]
		if ok && p.isAvailable(acct, isOpus) {
			copy := *acct
			return &copy, nil
		}
		if ok {
			return nil, fmt.Errorf("bound account %s unavailable (status=%s, schedulable=%v)",
				boundAccountID, acct.Status, acct.Schedulable)
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
		if !p.isAvailable(acct, isOpus) {
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
