package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/store"
)

// Ban signal patterns in 403 response bodies.
var banSignalPattern = regexp.MustCompile(`(?i)(organization has been disabled|account has been disabled|Too many active sessions|only authorized for use with claude code)`)

// SessionBinding holds the account ID bound to a session UUID.
type SessionBinding struct {
	AccountID  string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// UpstreamResult encapsulates the outcome of a single upstream request.
type UpstreamResult struct {
	AccountID  string
	StatusCode int
	Headers    http.Header
	ErrBody    []byte
	Model      string
	IsOpus     bool
}

// ErrorPauses holds configurable error pause durations.
type ErrorPauses struct {
	Pause401        time.Duration
	Pause401Refresh time.Duration // short cooldown for background token refresh on 401
	Pause403        time.Duration
	Pause429        time.Duration
	Pause529        time.Duration
}

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
// excludeIDs are accounts to skip (already failed this request).
func (p *Pool) Pick(provider domain.Provider, excludeIDs []string, isOpus bool, boundAccountID string) (*domain.Account, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

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

	// 2. Pool selection — filter, sort, pick best
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
			pri = AutoPriority(acct)
		}
		candidates = append(candidates, scored{acct: acct, priority: pri})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available accounts")
	}

	// Sort: priority DESC, then lastUsedAt ASC (round-robin)
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
	slog.Debug("account selected", "accountId", selected.ID, "email", selected.Email,
		"priority", candidates[0].priority, "mode", selected.PriorityMode)
	return &copy, nil
}

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

// AutoPriority computes the effective priority for an auto-mode account.
// Claude: min(5h_remain%, 7d_remain%); Codex: min(primary_remain%, secondary_remain%).
func AutoPriority(acct *domain.Account) int {
	if acct.Provider == domain.ProviderCodex {
		primaryRemain := 100.0
		if acct.CodexPrimaryUtil > 0 {
			primaryRemain = (1.0 - acct.CodexPrimaryUtil) * 100
		}
		secondaryRemain := 100.0
		if acct.CodexSecondaryUtil > 0 {
			secondaryRemain = (1.0 - acct.CodexSecondaryUtil) * 100
		}
		pri := primaryRemain
		if secondaryRemain < pri {
			pri = secondaryRemain
		}
		return int(pri)
	}

	fiveRemain := 100.0
	if acct.FiveHourUtil > 0 {
		fiveRemain = (1.0 - acct.FiveHourUtil) * 100
	}
	sevenRemain := 100.0
	if acct.SevenDayUtil > 0 {
		sevenRemain = (1.0 - acct.SevenDayUtil) * 100
	}
	pri := fiveRemain
	if sevenRemain < pri {
		pri = sevenRemain
	}
	return int(pri)
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// ---------------------------------------------------------------------------
// Observe (unified state-change entry point) [invariant 1]
// ---------------------------------------------------------------------------

// Observe records the outcome of an upstream request. This is the ONLY write
// path for account health state. All error handling, rate limit capture, and
// cooldown logic flows through here.
func (p *Pool) Observe(r UpstreamResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	acct, ok := p.accounts[r.AccountID]
	if !ok {
		return
	}

	switch r.StatusCode {
	case http.StatusOK:
		// Capture rate limit headers and set lastUsedAt
		now := time.Now().UTC()
		acct.LastUsedAt = &now
		if acct.Provider == domain.ProviderCodex {
			p.captureCodexHeadersLocked(acct, r.Headers)
		} else {
			p.captureClaudeHeadersLocked(acct, r.Headers)
		}

	case 529:
		until := time.Now().Add(p.pauses.Pause529)
		p.applyCooldown(acct, until)
		now := time.Now().UTC()
		acct.OverloadedAt = &now
		p.bus.Publish(events.Event{
			Type: events.EventOverload, AccountID: acct.ID,
			Message: fmt.Sprintf("529 overloaded, cooldown until %s", until.Format(time.RFC3339)),
		})
		slog.Warn("account overloaded (529)", "accountId", acct.ID, "model", r.Model)

	case 429:
		// Capture headers first (may contain utilization data)
		if acct.Provider == domain.ProviderCodex {
			p.captureCodexHeadersLocked(acct, r.Headers)
		} else {
			p.captureClaudeHeadersLocked(acct, r.Headers)
		}

		// Determine cooldown: Retry-After > unified-reset > codex body resets_in_seconds > config default
		until := time.Now().Add(p.pauses.Pause429)
		if retryAfter := parseRetryAfter(r.Headers.Get("Retry-After")); retryAfter > 0 {
			until = time.Now().Add(retryAfter)
		} else if resetStr := r.Headers.Get("anthropic-ratelimit-unified-reset"); resetStr != "" {
			if resetTime, err := time.Parse(time.RFC3339, resetStr); err == nil {
				until = resetTime
			}
		} else if acct.Provider == domain.ProviderCodex && len(r.ErrBody) > 0 {
			if resetsIn := parseCodexResetsIn(r.ErrBody); resetsIn > 0 {
				until = time.Now().Add(resetsIn)
			}
		}
		p.applyCooldown(acct, until)
		now := time.Now().UTC()
		acct.OverloadedAt = &now
		acct.RateLimitedAt = &now

		// Mark Opus-specific rate limit
		if r.IsOpus {
			if resetStr := r.Headers.Get("anthropic-ratelimit-unified-reset"); resetStr != "" {
				if resetTime, err := time.Parse(time.RFC3339, resetStr); err == nil {
					acct.OpusRateLimitEndAt = &resetTime
				}
			}
		}

		p.bus.Publish(events.Event{
			Type: events.EventRateLimit, AccountID: acct.ID,
			Message: fmt.Sprintf("429 rate limited, cooldown until %s", until.Format(time.RFC3339)),
		})
		slog.Warn("account rate limited (429)", "accountId", acct.ID, "model", r.Model, "until", until.UTC())

	case 403:
		bodyStr := string(r.ErrBody)
		if banSignalPattern.MatchString(bodyStr) {
			until := time.Now().Add(p.pauses.Pause401)
			acct.Status = domain.StatusBlocked
			acct.ErrorMessage = fmt.Sprintf("ban signal detected: %s", truncate(bodyStr, 200))
			acct.Schedulable = false
			p.applyCooldown(acct, until)
			p.bus.Publish(events.Event{
				Type: events.EventBan, AccountID: acct.ID,
				Message: acct.ErrorMessage,
			})
			slog.Error("ban signal detected (403)", "accountId", acct.ID, "model", r.Model)
		} else {
			until := time.Now().Add(p.pauses.Pause403)
			p.applyCooldown(acct, until)
			now := time.Now().UTC()
			acct.OverloadedAt = &now
			slog.Warn("account forbidden (403)", "accountId", acct.ID, "model", r.Model)
		}

	case 401:
		until := time.Now().Add(p.pauses.Pause401Refresh)
		p.applyCooldown(acct, until)
		p.bus.Publish(events.Event{
			Type: events.EventRefresh, AccountID: acct.ID,
			Message: "401 auth failed, background refresh triggered",
		})
		slog.Warn("account auth failed (401), triggering refresh",
			"accountId", acct.ID, "model", r.Model)
		if p.onAuthFailure != nil {
			go p.onAuthFailure(acct.ID)
		}
	}

	p.persistLocked(acct)
}

// ObserveSuccess is a shorthand for recording a successful request with rate
// limit header capture (used by admin_probe and relay success path).
func (p *Pool) ObserveSuccess(accountID string, headers http.Header) {
	p.Observe(UpstreamResult{
		AccountID:  accountID,
		StatusCode: http.StatusOK,
		Headers:    headers,
	})
}

// MarkLastUsed updates the lastUsedAt timestamp for an account.
func (p *Pool) MarkLastUsed(accountID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return
	}
	now := time.Now().UTC()
	acct.LastUsedAt = &now
	p.persistLocked(acct)
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
// Rate limit header capture (called under mu.Lock)
// ---------------------------------------------------------------------------

func (p *Pool) captureClaudeHeadersLocked(acct *domain.Account, headers http.Header) {
	if headers == nil {
		return
	}

	// 5-hour status
	if status := headers.Get("anthropic-ratelimit-unified-5h-status"); status != "" {
		acct.FiveHourStatus = status
		if status == "rejected" {
			fiveHourReset := headers.Get("anthropic-ratelimit-unified-5h-reset")
			now := time.Now().UTC()
			acct.Schedulable = false
			acct.OverloadedAt = &now

			resetTime := now.Add(5 * time.Hour)
			if fiveHourReset != "" {
				if secs, err := strconv.ParseInt(fiveHourReset, 10, 64); err == nil && secs > 0 {
					resetTime = time.Unix(secs, 0)
				} else if parsed, err := time.Parse(time.RFC3339, fiveHourReset); err == nil {
					resetTime = parsed
				}
			}
			p.applyCooldown(acct, resetTime)
			p.bus.Publish(events.Event{
				Type: events.EventFiveHStop, AccountID: acct.ID,
				Message: fmt.Sprintf("5h limit rejected, until %s", resetTime.Format(time.RFC3339)),
			})
			slog.Warn("account 5h limit rejected", "accountId", acct.ID, "until", resetTime)
		}
	}

	// Utilization + reset
	if v := headers.Get("anthropic-ratelimit-unified-5h-utilization"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			acct.FiveHourUtil = f
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-5h-reset"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			acct.FiveHourReset = secs
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-7d-utilization"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			acct.SevenDayUtil = f
		}
	}
	if v := headers.Get("anthropic-ratelimit-unified-7d-reset"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			acct.SevenDayReset = secs
		}
	}

	// Proactive cooldown on near-exhaustion
	p.maybeCooldownLocked(acct, acct.FiveHourUtil, acct.FiveHourReset, "5h")
	p.maybeCooldownLocked(acct, acct.SevenDayUtil, acct.SevenDayReset, "7d")
}

func (p *Pool) captureCodexHeadersLocked(acct *domain.Account, headers http.Header) {
	if headers == nil {
		return
	}

	var primaryResetSecs, secondaryResetSecs int

	if v := headers.Get("x-codex-primary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			acct.CodexPrimaryUtil = pct / 100
		}
	}
	if v := headers.Get("x-codex-primary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			primaryResetSecs = secs
			acct.CodexPrimaryReset = time.Now().Unix() + int64(secs)
		}
	}
	if v := headers.Get("x-codex-secondary-used-percent"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			acct.CodexSecondaryUtil = pct / 100
		}
	}
	if v := headers.Get("x-codex-secondary-reset-after-seconds"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			secondaryResetSecs = secs
			acct.CodexSecondaryReset = time.Now().Unix() + int64(secs)
		}
	}

	// Proactive cooldown: pick the longest reset among exhausted windows
	var cooldownUntil time.Time
	if acct.CodexPrimaryUtil >= 0.99 && primaryResetSecs > 0 {
		t := time.Now().Add(time.Duration(primaryResetSecs) * time.Second)
		if t.After(cooldownUntil) {
			cooldownUntil = t
		}
	}
	if acct.CodexSecondaryUtil >= 0.99 && secondaryResetSecs > 0 {
		t := time.Now().Add(time.Duration(secondaryResetSecs) * time.Second)
		if t.After(cooldownUntil) {
			cooldownUntil = t
		}
	}
	if !cooldownUntil.IsZero() {
		p.applyCooldown(acct, cooldownUntil)
		now := time.Now().UTC()
		acct.OverloadedAt = &now
		slog.Warn("codex account rate limit exhausted", "accountId", acct.ID, "until", cooldownUntil)
	}
}

// maybeCooldownLocked sets cooldown if utilization >= 0.99 and reset is in the future.
func (p *Pool) maybeCooldownLocked(acct *domain.Account, util float64, resetSecs int64, window string) {
	if util < 0.99 || resetSecs <= 0 {
		return
	}
	resetTime := time.Unix(resetSecs, 0)
	if time.Now().After(resetTime) {
		return
	}
	p.applyCooldown(acct, resetTime)
	now := time.Now().UTC()
	acct.OverloadedAt = &now
	slog.Warn("account rate limit exhausted", "accountId", acct.ID, "window", window, "until", resetTime)
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
	var cooldownUntil int64
	if acct.Provider == domain.ProviderCodex {
		if acct.CodexPrimaryUtil >= 0.99 && acct.CodexPrimaryReset > now {
			cooldownUntil = acct.CodexPrimaryReset
		}
		if acct.CodexSecondaryUtil >= 0.99 && acct.CodexSecondaryReset > now && acct.CodexSecondaryReset > cooldownUntil {
			cooldownUntil = acct.CodexSecondaryReset
		}
	} else {
		if acct.FiveHourUtil >= 0.99 && acct.FiveHourReset > now {
			cooldownUntil = acct.FiveHourReset
		}
		if acct.SevenDayUtil >= 0.99 && acct.SevenDayReset > now && acct.SevenDayReset > cooldownUntil {
			cooldownUntil = acct.SevenDayReset
		}
	}
	return cooldownUntil
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
// Helpers
// ---------------------------------------------------------------------------

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(value); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// parseCodexResetsIn extracts resets_in_seconds from a Codex 429 JSON body:
// {"error":{"type":"usage_limit_reached","resets_in_seconds":1125}}
func parseCodexResetsIn(body []byte) time.Duration {
	var envelope struct {
		Error struct {
			ResetsInSeconds int `json:"resets_in_seconds"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.ResetsInSeconds > 0 {
		return time.Duration(envelope.Error.ResetsInSeconds) * time.Second
	}
	return 0
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// BanSignalPattern is exported for use by the relay package.
func IsBanSignal(body string) bool {
	return banSignalPattern.MatchString(body)
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
