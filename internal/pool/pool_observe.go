package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
)

func (p *Pool) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ttlTicker := time.NewTicker(30 * time.Second)
	defer ttlTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanup()
		case <-ttlTicker.C:
			if _, err := p.store.PurgeExpiredSessionBindings(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
				slog.Warn("purge expired session bindings failed", "error", err)
			}
			if _, err := p.store.PurgeExpiredStainlessBindings(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
				slog.Warn("purge expired stainless bindings failed", "error", err)
			}
			if _, err := p.store.PurgeExpiredOAuthSessions(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
				slog.Warn("purge expired oauth sessions failed", "error", err)
			}
			if _, err := p.store.PurgeExpiredRefreshLocks(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
				slog.Warn("purge expired refresh locks failed", "error", err)
			}
		}
	}
}

func (p *Pool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reloadStateLocked(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "cleanup", "error", err)
	}

	now := time.Now()

	// Phase 1: bucket-scoped decisions. Each bucket is visited exactly once.
	for _, bucket := range p.buckets {
		if bucket.CooldownUntil != nil && now.After(*bucket.CooldownUntil) {
			// Check if any member is blocked — if so, defer clearing to
			// phase 2 so blocked recovery sees the non-nil cooldown.
			hasBlocked := false
			for _, acct := range p.accounts {
				if p.bucketKeyLocked(acct) == bucket.BucketKey && acct.Status == domain.StatusBlocked {
					hasBlocked = true
					break
				}
			}
			if !hasBlocked {
				bucket.CooldownUntil = nil
				bucket.UpdatedAt = now.UTC()
				p.persistBucketLocked(bucket)
				p.bus.Publish(events.Event{
					Type:      events.EventRecover,
					BucketKey: bucket.BucketKey,
					Message:   "cooldown expired",
				})
				slog.Info("bucket cooldown expired", "bucketKey", bucket.BucketKey)
			}
		}
	}

	// Phase 2: recover blocked accounts whose bucket cooldown has expired.
	// This runs after phase 1 so that buckets with blocked members still
	// have CooldownUntil set (phase 1 skipped clearing them).
	for _, acct := range p.accounts {
		if acct.Status == domain.StatusBlocked {
			cooldownUntil := p.bucketCooldownLocked(acct)
			if cooldownUntil != nil && now.After(*cooldownUntil) {
				acct.Status = domain.StatusActive
				acct.ErrorMessage = ""
				p.persistLocked(acct)
				p.bus.Publish(events.Event{
					Type: events.EventRecover, AccountID: acct.ID,
					BucketKey: p.bucketKeyLocked(acct),
					Message:   "blocked account recovered",
				})
				slog.Info("blocked account recovered", "accountId", acct.ID)
			}
		}
	}

	// Phase 3: clear bucket cooldowns that were held for blocked recovery,
	// and enforce exhausted cooldowns on now-available buckets.
	for _, bucket := range p.buckets {
		if bucket.CooldownUntil != nil && now.After(*bucket.CooldownUntil) {
			bucket.CooldownUntil = nil
			bucket.UpdatedAt = now.UTC()
			p.persistBucketLocked(bucket)
			p.bus.Publish(events.Event{
				Type:      events.EventRecover,
				BucketKey: bucket.BucketKey,
				Message:   "cooldown expired",
			})
			slog.Info("bucket cooldown expired", "bucketKey", bucket.BucketKey)
		}

		if bucket.CooldownUntil == nil {
			// Find any active member to compute exhausted cooldown.
			for _, acct := range p.accounts {
				if p.bucketKeyLocked(acct) != bucket.BucketKey || acct.Status != domain.StatusActive {
					continue
				}
				if cooldownUntil := p.computeExhaustedCooldown(acct, now.Unix()); cooldownUntil > 0 {
					resetTime := time.Unix(cooldownUntil, 0).UTC()
					result := p.applyBucketCooldown(bucket, resetTime)
					bucket.UpdatedAt = now.UTC()
					p.persistBucketLocked(bucket)
					if result.Applied {
						p.bus.Publish(events.Event{
							Type:          events.EventRateLimit,
							BucketKey:     bucket.BucketKey,
							CooldownUntil: &result.Actual,
							Message:       "exhausted cooldown enforced",
						})
					}
					slog.Warn("enforced exhausted cooldown", "bucketKey", bucket.BucketKey, "until", resetTime)
				}
				break // one member per bucket is enough for exhaustion check
			}
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
	t := drv.ComputeExhaustedCooldown(json.RawMessage(p.bucketStateLocked(acct)), time.Unix(now, 0))
	if !t.IsZero() {
		return t.Unix()
	}
	return 0
}

func (p *Pool) SetDrivers(drivers map[domain.Provider]driver.SchedulerDriver) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reloadStateLocked(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "set_drivers", "error", err)
	}
	p.drivers = drivers
	for _, acct := range p.accounts {
		prev := acct.BucketKey
		p.refreshBucketKeyLocked(acct, p.bucketStateLocked(acct))
		p.ensureBucketLocked(acct)
		if acct.BucketKey != prev {
			p.persistLocked(acct)
		}
	}
}

func (p *Pool) Observe(accountID string, effect driver.Effect) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.reloadStateLocked(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "observe", "accountId", accountID, "error", err)
	}

	acct, ok := p.accounts[accountID]
	if !ok {
		return
	}

	bucket := p.ensureBucketLocked(acct)
	persisted := make(map[string]*domain.Account)
	markPersist := func(a *domain.Account) {
		if a != nil {
			persisted[a.ID] = a
		}
	}

	// Reset circuit breaker counter on any non-500 outcome.
	if effect.Kind != driver.EffectServerError && bucket != nil {
		delete(p.serverErrCount, bucket.BucketKey)
	}

	// Collect event to emit after all state mutations (including UpdatedState
	// which may change bucketKey). BucketKey is set after UpdatedState.
	var pendingEvent *events.Event

	// cooldownPtr returns a non-nil pointer only for non-zero times.
	// Zero CooldownUntil means the driver didn't set one — emit nil
	// (explicit blank) rather than a fake 0001-01-01 sentinel.
	cooldownPtr := func(r CooldownResult) *time.Time {
		if r.Actual.IsZero() {
			return nil
		}
		return &r.Actual
	}

	// cooldownSuffix formats " (cooldown until ...)" or "" for zero.
	cooldownSuffix := func(r CooldownResult) string {
		if r.Actual.IsZero() {
			return ""
		}
		return fmt.Sprintf(" (cooldown until %s)", r.Actual.Format(time.RFC3339))
	}

	switch effect.Kind {
	case driver.EffectSuccess:
		now := time.Now().UTC()
		acct.LastUsedAt = &now
		markPersist(acct)

	case driver.EffectServerError:
		now := time.Now().UTC()
		acct.LastUsedAt = &now
		markPersist(acct)
		if bucket != nil {
			p.serverErrCount[bucket.BucketKey]++
			if p.serverErrCount[bucket.BucketKey] >= 3 {
				cooldownUntil := time.Now().Add(30 * time.Second)
				result := p.applyBucketCooldown(bucket, cooldownUntil)
				bucket.UpdatedAt = time.Now().UTC()
				p.persistBucketLocked(bucket)
				delete(p.serverErrCount, bucket.BucketKey)
				pendingEvent = &events.Event{
					Type:          events.EventOverload,
					AccountID:     acct.ID,
					CooldownUntil: cooldownPtr(result),
					Message:       "circuit breaker: 3 consecutive upstream 500s" + cooldownSuffix(result),
				}
			}
		}

	case driver.EffectCooldown:
		result := p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		pendingEvent = &events.Event{
			Type: events.EventRateLimit, AccountID: acct.ID,
			CooldownUntil: cooldownPtr(result),
			Message:       "cooldown" + cooldownSuffix(result),
		}

	case driver.EffectOverload:
		result := p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		pendingEvent = &events.Event{
			Type: events.EventOverload, AccountID: acct.ID,
			CooldownUntil: cooldownPtr(result),
			Message:       "overloaded" + cooldownSuffix(result),
		}

	case driver.EffectBlock:
		acct.Status = domain.StatusBlocked
		acct.ErrorMessage = effect.ErrorMessage
		result := p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		markPersist(acct)
		pendingEvent = &events.Event{
			Type: events.EventBan, AccountID: acct.ID,
			CooldownUntil: cooldownPtr(result),
			Message:       effect.ErrorMessage + cooldownSuffix(result),
		}

	case driver.EffectAuthFail:
		result := p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		pendingEvent = &events.Event{
			Type: events.EventRefresh, AccountID: acct.ID,
			CooldownUntil: cooldownPtr(result),
			Message:       "auth failed, background refresh triggered",
		}
		if p.onAuthFailure != nil {
			go p.onAuthFailure(acct.ID)
		}
	}

	if len(effect.UpdatedState) > 0 {
		stateJSON := string(effect.UpdatedState)
		members := p.bucketAccountsLocked(acct)
		if bucket != nil {
			bucket.StateJSON = stateJSON
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		for _, member := range members {
			prevKey := member.BucketKey
			p.refreshBucketKeyLocked(member, stateJSON)
			if member.BucketKey != prevKey {
				markPersist(member)
			}
		}
	}

	// Emit event with committed bucketKey (after any UpdatedState migration).
	if pendingEvent != nil {
		pendingEvent.BucketKey = p.bucketKeyLocked(acct)
		p.bus.Publish(*pendingEvent)
	}

	for _, target := range persisted {
		p.persistLocked(target)
	}
}
