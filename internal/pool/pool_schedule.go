package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

func (p *Pool) isAvailable(acct *domain.Account, drv driver.SchedulerDriver, model string, now time.Time) bool {
	if acct.Status != domain.StatusActive {
		return false
	}
	if acct.CellID != "" && !p.cellAvailableLocked(p.cellForAccountLocked(acct), now) {
		return false
	}
	if cooldownUntil := p.bucketCooldownLocked(acct); cooldownUntil != nil && now.Before(*cooldownUntil) {
		return false
	}
	if !drv.CanServe(json.RawMessage(p.bucketStateLocked(acct)), model, now) {
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

func (p *Pool) IsAvailableFor(accountID string, drv driver.SchedulerDriver, model string) bool {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "is_available_for", "error", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return false
	}
	return p.isAvailable(acct, drv, model, time.Now())
}

func (p *Pool) Pick(drv driver.SchedulerDriver, exclusions []Exclusion, model string, boundAccountID string) (*domain.Account, error) {
	if err := p.refreshState(context.Background()); err != nil {
		return nil, fmt.Errorf("refresh pool state: %w", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	provider := drv.Provider()
	now := time.Now()
	excludedAccounts := make(map[string]struct{}, len(exclusions))
	excludedBuckets := make(map[string]struct{}, len(exclusions))
	for _, exclusion := range exclusions {
		if exclusion.AccountID != "" {
			excludedAccounts[exclusion.AccountID] = struct{}{}
		}
		if exclusion.BucketKey != "" {
			excludedBuckets[exclusion.BucketKey] = struct{}{}
		}
	}

	if boundAccountID != "" {
		acct, ok := p.accounts[boundAccountID]
		if ok {
			if _, blocked := excludedAccounts[acct.ID]; blocked {
				return nil, fmt.Errorf("bound account %s excluded", boundAccountID)
			}
			if _, blocked := excludedBuckets[p.bucketKeyLocked(acct)]; blocked {
				return nil, fmt.Errorf("bound bucket %s excluded", p.bucketKeyLocked(acct))
			}
		}
		if ok && p.isAvailable(acct, drv, model, now) {
			return p.projectAccountLocked(acct), nil
		}
		if ok {
			return nil, fmt.Errorf("bound account %s unavailable (status=%s)", boundAccountID, acct.Status)
		}
	}

	type bucketCandidate struct {
		key      string
		accts    []*domain.Account
		priority int
	}
	buckets := make(map[string]*bucketCandidate)
	for _, acct := range p.accounts {
		if _, excluded := excludedAccounts[acct.ID]; excluded {
			continue
		}
		if !p.matchesProvider(acct, provider) {
			continue
		}
		bucketKey := p.bucketKeyLocked(acct)
		if _, excluded := excludedBuckets[bucketKey]; excluded {
			continue
		}
		if !p.isAvailable(acct, drv, model, now) {
			continue
		}
		bucket := buckets[bucketKey]
		pri := acct.Priority
		if acct.PriorityMode == "auto" {
			pri = drv.AutoPriority(json.RawMessage(p.bucketStateLocked(acct)))
		}
		if bucket == nil {
			bucket = &bucketCandidate{key: bucketKey, priority: pri}
			buckets[bucketKey] = bucket
		} else if pri > bucket.priority {
			bucket.priority = pri
		}
		bucket.accts = append(bucket.accts, acct)
	}

	candidates := make([]bucketCandidate, 0, len(buckets))
	for _, candidate := range buckets {
		candidates = append(candidates, *candidate)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available accounts")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority > candidates[j].priority
		}
		ti := timeOrZero(leastRecentlyUsed(candidates[i].accts).LastUsedAt)
		tj := timeOrZero(leastRecentlyUsed(candidates[j].accts).LastUsedAt)
		return ti.Before(tj)
	})

	selected := leastRecentlyUsed(candidates[0].accts)
	slog.Debug("account selected (driver)", "accountId", selected.ID, "email", selected.Email,
		"priority", candidates[0].priority, "mode", selected.PriorityMode, "bucketKey", candidates[0].key)
	return p.projectAccountLocked(selected), nil
}

func leastRecentlyUsed(accounts []*domain.Account) *domain.Account {
	if len(accounts) == 0 {
		return nil
	}
	best := accounts[0]
	bestTime := timeOrZero(best.LastUsedAt)
	for _, acct := range accounts[1:] {
		if t := timeOrZero(acct.LastUsedAt); t.Before(bestTime) {
			best = acct
			bestTime = t
		}
	}
	return best
}
