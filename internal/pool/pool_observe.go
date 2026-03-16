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

	for _, acct := range p.accounts {
		changed := false

		if bucket := p.bucketLocked(acct); bucket != nil {
			if bucket.CooldownUntil != nil && now.After(*bucket.CooldownUntil) {
				if acct.Status != domain.StatusBlocked {
					bucket.CooldownUntil = nil
					bucket.UpdatedAt = now.UTC()
					p.persistBucketLocked(bucket)
					p.bus.Publish(events.Event{
						Type: events.EventRecover, AccountID: acct.ID,
						Message: "cooldown expired",
					})
					slog.Info("account cooldown expired", "accountId", acct.ID)
				}
			}

			if acct.Status == domain.StatusActive && bucket.CooldownUntil == nil {
				if cooldownUntil := p.computeExhaustedCooldown(acct, now.Unix()); cooldownUntil > 0 {
					resetTime := time.Unix(cooldownUntil, 0).UTC()
					p.applyBucketCooldown(bucket, resetTime)
					bucket.UpdatedAt = now.UTC()
					p.persistBucketLocked(bucket)
					slog.Warn("enforced cooldown on exhausted account", "account", acct.Email, "until", resetTime)
				}
			}
		}

		if acct.Status == domain.StatusBlocked {
			cooldownUntil := p.bucketCooldownLocked(acct)
			if cooldownUntil != nil && now.After(*cooldownUntil) {
				acct.Status = domain.StatusActive
				acct.ErrorMessage = ""
				changed = true
				p.bus.Publish(events.Event{
					Type: events.EventRecover, AccountID: acct.ID,
					Message: "blocked account recovered",
				})
				slog.Info("blocked account recovered", "accountId", acct.ID)
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

	switch effect.Kind {
	case driver.EffectSuccess:
		now := time.Now().UTC()
		acct.LastUsedAt = &now
		markPersist(acct)
		if bucket != nil {
			delete(p.serverErrCount, bucket.BucketKey)
		}

	case driver.EffectServerError:
		now := time.Now().UTC()
		acct.LastUsedAt = &now
		markPersist(acct)
		if bucket != nil {
			p.serverErrCount[bucket.BucketKey]++
			if p.serverErrCount[bucket.BucketKey] >= 3 {
				cooldownUntil := time.Now().Add(30 * time.Second)
				p.applyBucketCooldown(bucket, cooldownUntil)
				bucket.UpdatedAt = time.Now().UTC()
				p.persistBucketLocked(bucket)
				delete(p.serverErrCount, bucket.BucketKey)
				p.bus.Publish(events.Event{
					Type: events.EventOverload, AccountID: acct.ID,
					Message: "circuit breaker: 3 consecutive upstream 500s, cooldown 30s",
				})
			}
		}

	case driver.EffectCooldown:
		p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		p.bus.Publish(events.Event{
			Type: events.EventRateLimit, AccountID: acct.ID,
			Message: fmt.Sprintf("cooldown until %s", effect.CooldownUntil.Format(time.RFC3339)),
		})

	case driver.EffectOverload:
		p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		p.bus.Publish(events.Event{
			Type: events.EventOverload, AccountID: acct.ID,
			Message: fmt.Sprintf("overloaded, cooldown until %s", effect.CooldownUntil.Format(time.RFC3339)),
		})

	case driver.EffectBlock:
		acct.Status = domain.StatusBlocked
		acct.ErrorMessage = effect.ErrorMessage
		p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		markPersist(acct)
		p.bus.Publish(events.Event{
			Type: events.EventBan, AccountID: acct.ID,
			Message: effect.ErrorMessage,
		})

	case driver.EffectAuthFail:
		p.applyBucketCooldown(bucket, effect.CooldownUntil)
		if bucket != nil {
			bucket.UpdatedAt = time.Now().UTC()
			p.persistBucketLocked(bucket)
		}
		p.bus.Publish(events.Event{
			Type: events.EventRefresh, AccountID: acct.ID,
			Message: "auth failed, background refresh triggered",
		})
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

	for _, target := range persisted {
		p.persistLocked(target)
	}
}
