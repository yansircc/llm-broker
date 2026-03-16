package pool

import (
	"log/slog"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/events"
)

type overloadState struct {
	since             time.Time
	consecutiveProbes int
}

// OverloadBucketInfo describes an overloaded bucket ready for probing.
type OverloadBucketInfo struct {
	BucketKey string
	Provider  domain.Provider
}

// markOverloadLocked sets overload state for a bucket if not already set.
// Caller must hold p.mu.
func (p *Pool) markOverloadLocked(bucketKey string) {
	if _, ok := p.overloadBackoff[bucketKey]; ok {
		return
	}
	p.overloadBackoff[bucketKey] = &overloadState{
		since: time.Now(),
	}
}

// clearOverloadLocked removes overload state for a bucket.
// Caller must hold p.mu.
func (p *Pool) clearOverloadLocked(bucketKey string) {
	delete(p.overloadBackoff, bucketKey)
}

// isOverloadBucketLocked checks if a bucket is in overload state.
// Caller must hold p.mu.
func (p *Pool) isOverloadBucketLocked(bucketKey string) bool {
	_, ok := p.overloadBackoff[bucketKey]
	return ok
}

// OverloadedBucketsReady returns overloaded buckets whose cooldown has expired,
// meaning they are ready for a probe attempt.
func (p *Pool) OverloadedBucketsReady() []OverloadBucketInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	var ready []OverloadBucketInfo
	for bucketKey := range p.overloadBackoff {
		bucket, ok := p.buckets[bucketKey]
		if !ok {
			continue
		}
		if bucket.CooldownUntil != nil && now.Before(*bucket.CooldownUntil) {
			continue
		}
		ready = append(ready, OverloadBucketInfo{
			BucketKey: bucketKey,
			Provider:  bucket.Provider,
		})
	}
	return ready
}

// ClearOverload clears overload state, cooldown, and emits a recovery event.
func (p *Pool) ClearOverload(bucketKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.clearOverloadLocked(bucketKey)
	bucket, ok := p.buckets[bucketKey]
	if !ok {
		return
	}
	if bucket.CooldownUntil != nil {
		bucket.CooldownUntil = nil
		bucket.UpdatedAt = time.Now().UTC()
		p.persistBucketLocked(bucket)
	}
	p.bus.Publish(events.Event{
		Type:      events.EventRecover,
		BucketKey: bucketKey,
		Message:   "overload probe: recovered",
	})
	slog.Info("overload probe: recovered", "bucketKey", bucketKey)
}

// ExtendOverloadCooldown applies exponential backoff on a still-overloaded bucket.
// Formula: baseDuration * 2^(consecutiveProbes+1), capped at 30min.
func (p *Pool) ExtendOverloadCooldown(bucketKey string, baseDuration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	state, ok := p.overloadBackoff[bucketKey]
	if !ok {
		return
	}

	exponent := state.consecutiveProbes + 1
	duration := baseDuration
	for i := 0; i < exponent; i++ {
		duration *= 2
		if duration > 30*time.Minute {
			duration = 30 * time.Minute
			break
		}
	}
	state.consecutiveProbes++

	bucket, ok := p.buckets[bucketKey]
	if !ok {
		return
	}
	until := time.Now().Add(duration).UTC()
	bucket.CooldownUntil = &until
	bucket.UpdatedAt = time.Now().UTC()
	p.persistBucketLocked(bucket)

	p.bus.Publish(events.Event{
		Type:          events.EventOverload,
		BucketKey:     bucketKey,
		CooldownUntil: &until,
		Message:       "overload probe: still overloaded, extending cooldown",
	})
	slog.Warn("overload probe: still overloaded, extending cooldown",
		"bucketKey", bucketKey, "until", until, "probeAttempt", state.consecutiveProbes)
}

// AnyAccountInBucket returns one active account from the given bucket for probing.
func (p *Pool) AnyAccountInBucket(bucketKey string) *domain.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, acct := range p.accounts {
		if p.bucketKeyLocked(acct) == bucketKey && acct.Status == domain.StatusActive {
			return p.projectAccountLocked(acct)
		}
	}
	return nil
}

// IsProviderOverloaded returns true if ALL active buckets for a provider are
// currently in overload state.
func (p *Pool) IsProviderOverloaded(provider domain.Provider) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hasActiveBucket := false
	for _, bucket := range p.buckets {
		if bucket.Provider != provider {
			continue
		}
		// Only consider buckets that have at least one active account.
		hasMember := false
		for _, acct := range p.accounts {
			if p.bucketKeyLocked(acct) == bucket.BucketKey && acct.Status == domain.StatusActive {
				hasMember = true
				break
			}
		}
		if !hasMember {
			continue
		}
		hasActiveBucket = true
		if !p.isOverloadBucketLocked(bucket.BucketKey) {
			return false
		}
	}
	return hasActiveBucket
}
