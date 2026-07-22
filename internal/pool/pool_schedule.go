package pool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

func (p *Pool) structurallyAvailableLocked(acct *domain.Account, now time.Time) bool {
	if acct.Status != domain.StatusActive {
		return false
	}
	if acct.CellID != "" && !p.cellAvailableLocked(p.cellForAccountLocked(acct), now) {
		return false
	}
	if cooldownUntil := p.bucketCooldownLocked(acct); cooldownUntil != nil && now.Before(*cooldownUntil) {
		return false
	}
	return true
}

func (p *Pool) isAvailable(acct *domain.Account, drv driver.SchedulerDriver, model string, now time.Time) bool {
	return p.structurallyAvailableLocked(acct, now) &&
		drv.AssessCapacity(json.RawMessage(p.bucketStateLocked(acct)), model, now).Eligible
}

type bucketCandidate struct {
	key      string
	accts    []*domain.Account
	priority int
	class    string
}

type routeLoadKey struct {
	provider domain.Provider
	bucket   string
	class    string
}

type routeLoad struct {
	inflight     int
	lastAssigned uint64
}

type affinityClaim struct {
	done chan struct{}
}

var (
	ErrAffinityOwnerMissing     = errors.New("required affinity owner missing")
	ErrAffinityOwnerUnavailable = errors.New("required affinity owner unavailable")
)

type AttemptLease struct {
	pool        *Pool
	loadKey     routeLoadKey
	account     *domain.Account
	affinityKey string
	claim       *affinityClaim

	mu       sync.Mutex
	accepted bool
	terminal bool
}

type RouteRequest struct {
	Exclusions    []Exclusion
	Model         string
	Surface       domain.Surface
	HardAccountID string
	AffinityKey   string
	Continuity    driver.AffinityContinuity
}

type SurfaceAvailability struct {
	Native bool
	Compat bool
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
	// Hotfix: Claude compat failures on legacy-direct accounts are caused by
	// coupling surface eligibility to cell lane. Keep this provider-specific
	// escape hatch only until surface/account policy is moved out of cell lane
	// and user pins become provider-scoped.
	if acct != nil && acct.Provider == domain.ProviderClaude {
		return surface == domain.SurfaceNative || surface == domain.SurfaceCompat
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

func (p *Pool) capacityAssessmentLocked(acct *domain.Account, drv driver.SchedulerDriver, model string, now time.Time) driver.CapacityAssessment {
	assessment := drv.AssessCapacity(json.RawMessage(p.bucketStateLocked(acct)), model, now)
	if acct.PriorityMode != "auto" {
		assessment.Priority = acct.Priority
	}
	return assessment
}

func (p *Pool) SurfaceAvailabilityMap() map[string]SurfaceAvailability {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "surface_availability_map", "error", err)
		return map[string]SurfaceAvailability{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	result := make(map[string]SurfaceAvailability, len(p.accounts))
	for _, acct := range p.accounts {
		drv, ok := p.drivers[acct.Provider]
		if !ok {
			result[acct.ID] = SurfaceAvailability{}
			continue
		}
		result[acct.ID] = SurfaceAvailability{
			Native: p.allowedOnSurfaceLocked(acct, domain.SurfaceNative) && p.isAvailable(acct, drv, "", now),
			Compat: p.allowedOnSurfaceLocked(acct, domain.SurfaceCompat) && p.isAvailable(acct, drv, "", now),
		}
	}
	return result
}

func (p *Pool) Pick(drv driver.SchedulerDriver, exclusions []Exclusion, model string, boundAccountID string) (*domain.Account, error) {
	return p.PickForSurface(drv, exclusions, model, boundAccountID, domain.SurfaceNative)
}

func (p *Pool) PickForSurface(drv driver.SchedulerDriver, exclusions []Exclusion, model string, boundAccountID string, surface domain.Surface) (*domain.Account, error) {
	acct, lease, err := p.AcquireRoute(context.Background(), drv, RouteRequest{
		Exclusions:    exclusions,
		Model:         model,
		Surface:       surface,
		HardAccountID: boundAccountID,
	})
	if lease != nil {
		lease.Finish()
	}
	return acct, err
}

func (p *Pool) AcquireRoute(ctx context.Context, drv driver.SchedulerDriver, req RouteRequest) (*domain.Account, *AttemptLease, error) {
	for {
		if err := p.refreshState(ctx); err != nil {
			return nil, nil, fmt.Errorf("refresh pool state: %w", err)
		}

		var binding *domain.SessionBinding
		var err error
		if req.HardAccountID == "" && req.AffinityKey != "" {
			binding, err = p.store.GetSessionBinding(ctx, req.AffinityKey)
			if err != nil {
				return nil, nil, fmt.Errorf("load affinity binding: %w", err)
			}
		}

		p.mu.Lock()
		if req.Continuity == driver.AffinityRequire && req.AffinityKey == "" {
			pinned := p.accounts[req.HardAccountID]
			if pinned == nil || pinned.Provider != drv.Provider() {
				p.mu.Unlock()
				return nil, nil, ErrAffinityOwnerMissing
			}
		}
		if binding != nil && binding.Provider != drv.Provider() {
			if req.Continuity == driver.AffinityRequire {
				p.mu.Unlock()
				return nil, nil, ErrAffinityOwnerUnavailable
			}
			binding = nil
		}
		boundID := req.HardAccountID
		if boundID == "" && binding != nil {
			boundID = p.accountIDForBindingLocked(binding)
			if boundID == "" && req.Continuity == driver.AffinityRequire {
				p.mu.Unlock()
				return nil, nil, ErrAffinityOwnerUnavailable
			}
		}

		if boundID != "" {
			acct, lease, acquireErr := p.acquireLocked(drv, req, boundID, nil)
			if acquireErr == nil {
				p.mu.Unlock()
				return acct, lease, nil
			}
			if req.HardAccountID != "" {
				p.mu.Unlock()
				return nil, nil, acquireErr
			}
			if req.Continuity == driver.AffinityRequire {
				p.mu.Unlock()
				return nil, nil, fmt.Errorf("%w: %v", ErrAffinityOwnerUnavailable, acquireErr)
			}
		}

		if req.AffinityKey != "" && binding == nil && req.Continuity == driver.AffinityRequire {
			p.mu.Unlock()
			return nil, nil, ErrAffinityOwnerMissing
		}

		var claim *affinityClaim
		if req.AffinityKey != "" {
			if pending := p.affinityClaims[req.AffinityKey]; pending != nil {
				done := pending.done
				p.mu.Unlock()
				select {
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				case <-done:
					continue
				}
			}
			claim = &affinityClaim{done: make(chan struct{})}
			p.affinityClaims[req.AffinityKey] = claim
		}

		acct, lease, acquireErr := p.acquireLocked(drv, req, "", claim)
		if acquireErr != nil {
			p.releaseAffinityClaimLocked(req.AffinityKey, claim)
			p.mu.Unlock()
			return nil, nil, acquireErr
		}
		p.mu.Unlock()
		return acct, lease, nil
	}
}

func (p *Pool) accountIDForBindingLocked(binding *domain.SessionBinding) string {
	if binding == nil {
		return ""
	}
	for _, acct := range p.accounts {
		if acct.Provider == binding.Provider && acct.Subject == binding.Subject {
			return acct.ID
		}
	}
	return ""
}

func (p *Pool) acquireLocked(drv driver.SchedulerDriver, req RouteRequest, boundAccountID string, claim *affinityClaim) (*domain.Account, *AttemptLease, error) {
	provider := drv.Provider()
	now := time.Now()
	exclusions := req.Exclusions
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
		// Hotfix: bound_account_id is global today. Ignore it for other
		// providers until user pins are migrated to provider-scoped storage.
		if ok && acct.Provider != provider {
			boundAccountID = ""
		} else if ok {
			if _, blocked := excludedAccounts[acct.ID]; blocked {
				return nil, nil, fmt.Errorf("bound account %s excluded", boundAccountID)
			}
			if _, blocked := excludedBuckets[p.bucketKeyLocked(acct)]; blocked {
				return nil, nil, fmt.Errorf("bound bucket %s excluded", p.bucketKeyLocked(acct))
			}
		}
		if boundAccountID != "" && ok && p.allowedOnSurfaceLocked(acct, req.Surface) && p.structurallyAvailableLocked(acct, now) {
			assessment := p.capacityAssessmentLocked(acct, drv, req.Model, now)
			if assessment.Eligible {
				projected, lease := p.leaseAccountLocked(acct, assessment.Class, req.AffinityKey, claim)
				return projected, lease, nil
			}
		}
		if boundAccountID != "" && ok {
			return nil, nil, fmt.Errorf("bound account %s unavailable (status=%s)", boundAccountID, acct.Status)
		}
		if boundAccountID != "" {
			return nil, nil, fmt.Errorf("bound account %s not found", boundAccountID)
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
		if !p.allowedOnSurfaceLocked(acct, req.Surface) {
			continue
		}
		if !p.structurallyAvailableLocked(acct, now) {
			continue
		}
		assessment := p.capacityAssessmentLocked(acct, drv, req.Model, now)
		if !assessment.Eligible {
			continue
		}
		bucket := buckets[bucketKey]
		pri := assessment.Priority
		if bucket == nil {
			bucket = &bucketCandidate{key: bucketKey, priority: pri, class: assessment.Class}
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
		return nil, nil, fmt.Errorf("no available accounts")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].key < candidates[j].key
	})

	chosen := p.leastInflightCandidateLocked(provider, candidates)
	selected := leastRecentlyUsed(chosen.accts)
	projected, lease := p.leaseAccountLocked(selected, chosen.class, req.AffinityKey, claim)
	load := p.routeLoads[lease.loadKey]
	slog.Debug("account selected (driver)", "accountId", selected.ID, "email", selected.Email,
		"priority", chosen.priority, "weight", bucketPriorityWeight(chosen.priority), "mode", selected.PriorityMode,
		"bucketKey", chosen.key, "capacityClass", chosen.class, "inflight", load.inflight)
	return projected, lease, nil
}

func (p *Pool) leastInflightCandidateLocked(provider domain.Provider, candidates []bucketCandidate) *bucketCandidate {
	best := &candidates[0]
	bestLoad := p.routeLoads[routeLoadKey{provider: provider, bucket: best.key, class: best.class}]
	if bestLoad == nil {
		bestLoad = &routeLoad{}
	}
	bestScore := float64(bestLoad.inflight+1) / bucketPriorityWeight(best.priority)

	for i := 1; i < len(candidates); i++ {
		candidate := &candidates[i]
		key := routeLoadKey{provider: provider, bucket: candidate.key, class: candidate.class}
		load := p.routeLoads[key]
		if load == nil {
			load = &routeLoad{}
		}
		score := float64(load.inflight+1) / bucketPriorityWeight(candidate.priority)
		if score < bestScore || (score == bestScore && (load.lastAssigned < bestLoad.lastAssigned ||
			(load.lastAssigned == bestLoad.lastAssigned && candidate.key < best.key))) {
			best = candidate
			bestLoad = load
			bestScore = score
		}
	}
	return best
}

func (p *Pool) leaseAccountLocked(acct *domain.Account, class, affinityKey string, claim *affinityClaim) (*domain.Account, *AttemptLease) {
	key := routeLoadKey{provider: acct.Provider, bucket: p.bucketKeyLocked(acct), class: class}
	load := p.routeLoads[key]
	if load == nil {
		load = &routeLoad{}
		p.routeLoads[key] = load
	}
	p.routeSeq++
	load.inflight++
	load.lastAssigned = p.routeSeq
	projected := p.projectAccountLocked(acct)
	return projected, &AttemptLease{
		pool:        p,
		loadKey:     key,
		account:     projected,
		affinityKey: affinityKey,
		claim:       claim,
	}
}

func (p *Pool) releaseAffinityClaimLocked(key string, claim *affinityClaim) {
	if key == "" || claim == nil || p.affinityClaims[key] != claim {
		return
	}
	delete(p.affinityClaims, key)
	close(claim.done)
}

func (l *AttemptLease) Accept(ctx context.Context, ttl time.Duration) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.terminal {
		return fmt.Errorf("attempt lease already finished")
	}
	if l.accepted {
		return nil
	}
	if l.affinityKey != "" {
		if err := l.pool.SetSessionBinding(ctx, l.affinityKey, l.account, ttl); err != nil {
			l.pool.mu.Lock()
			l.pool.releaseAffinityClaimLocked(l.affinityKey, l.claim)
			l.pool.mu.Unlock()
			return err
		}
	}
	l.accepted = true
	l.pool.mu.Lock()
	l.pool.releaseAffinityClaimLocked(l.affinityKey, l.claim)
	l.pool.mu.Unlock()
	return nil
}

func (l *AttemptLease) Finish() {
	l.finish()
}

func (l *AttemptLease) Abort() {
	l.finish()
}

func (l *AttemptLease) finish() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.terminal {
		return
	}
	l.terminal = true
	l.pool.mu.Lock()
	defer l.pool.mu.Unlock()
	if load := l.pool.routeLoads[l.loadKey]; load != nil && load.inflight > 0 {
		load.inflight--
	}
	l.pool.releaseAffinityClaimLocked(l.affinityKey, l.claim)
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
