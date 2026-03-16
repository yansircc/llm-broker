package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
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

type bucketCandidate struct {
	key      string
	accts    []*domain.Account
	priority int
}

func cellLane(cell *domain.EgressCell) domain.Surface {
	if cell == nil || len(cell.Labels) == 0 {
		return ""
	}
	return domain.NormalizeSurface(cell.Labels["lane"])
}

func (p *Pool) allowedOnSurfaceLocked(acct *domain.Account, surface domain.Surface) bool {
	surface = domain.NormalizeSurface(string(surface))
	if surface == "" || surface == domain.SurfaceAll {
		return true
	}

	cell := p.cellForAccountLocked(acct)
	lane := cellLane(cell)

	switch surface {
	case domain.SurfaceCompat:
		if cell == nil {
			return false
		}
		return lane == domain.SurfaceCompat || lane == domain.SurfaceAll
	case domain.SurfaceNative:
		if cell == nil {
			return true
		}
		return lane == "" || lane == domain.SurfaceNative || lane == domain.SurfaceAll
	default:
		return false
	}
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
	return p.IsAvailableForSurface(accountID, drv, model, domain.SurfaceNative)
}

func (p *Pool) IsAvailableForSurface(accountID string, drv driver.SchedulerDriver, model string, surface domain.Surface) bool {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "is_available_for", "error", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return false
	}
	if !p.allowedOnSurfaceLocked(acct, surface) {
		return false
	}
	return p.isAvailable(acct, drv, model, time.Now())
}

func (p *Pool) Pick(drv driver.SchedulerDriver, exclusions []Exclusion, model string, boundAccountID string) (*domain.Account, error) {
	return p.PickForSurface(drv, exclusions, model, boundAccountID, domain.SurfaceNative)
}

func (p *Pool) PickForSurface(drv driver.SchedulerDriver, exclusions []Exclusion, model string, boundAccountID string, surface domain.Surface) (*domain.Account, error) {
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
		if ok && p.allowedOnSurfaceLocked(acct, surface) && p.isAvailable(acct, drv, model, now) {
			return p.projectAccountLocked(acct), nil
		}
		if ok {
			return nil, fmt.Errorf("bound account %s unavailable (status=%s)", boundAccountID, acct.Status)
		}
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
		if !p.allowedOnSurfaceLocked(acct, surface) {
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
		return candidates[i].key < candidates[j].key
	})

	chosen := pickBucketCandidate(candidates, func(totalWeight float64) float64 {
		return rand.Float64() * totalWeight
	})
	selected := leastRecentlyUsed(chosen.accts)
	slog.Debug("account selected (driver)", "accountId", selected.ID, "email", selected.Email,
		"priority", chosen.priority, "weight", bucketPriorityWeight(chosen.priority), "mode", selected.PriorityMode, "bucketKey", chosen.key)
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

func bucketPriorityWeight(priority int) float64 {
	if priority < 0 {
		priority = 0
	}
	return 1 + math.Sqrt(float64(priority))
}

func pickBucketCandidate(candidates []bucketCandidate, draw func(totalWeight float64) float64) *bucketCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return &candidates[0]
	}

	totalWeight := 0.0
	for _, candidate := range candidates {
		totalWeight += bucketPriorityWeight(candidate.priority)
	}
	if totalWeight <= 0 {
		return &candidates[0]
	}

	offset := draw(totalWeight)
	switch {
	case offset < 0:
		offset = 0
	case offset >= totalWeight:
		offset = math.Nextafter(totalWeight, 0)
	}

	cursor := 0.0
	for i := range candidates {
		cursor += bucketPriorityWeight(candidates[i].priority)
		if offset < cursor {
			return &candidates[i]
		}
	}
	return &candidates[len(candidates)-1]
}
