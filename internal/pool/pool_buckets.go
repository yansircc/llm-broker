package pool

import (
	"context"
	"log/slog"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func defaultBucketKey(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.Subject != "" {
		return string(acct.Provider) + ":" + acct.Subject
	}
	return string(acct.Provider) + ":" + acct.ID
}

func (p *Pool) bucketKeyLocked(acct *domain.Account) string {
	if acct == nil {
		return ""
	}
	if acct.BucketKey != "" {
		return acct.BucketKey
	}
	return defaultBucketKey(acct)
}

func (p *Pool) bucketStateLocked(acct *domain.Account) string {
	bucket := p.bucketLocked(acct)
	if bucket == nil || bucket.StateJSON == "" {
		return "{}"
	}
	return bucket.StateJSON
}

func (p *Pool) bucketCooldownLocked(acct *domain.Account) *time.Time {
	bucket := p.bucketLocked(acct)
	if bucket == nil {
		return nil
	}
	return bucket.CooldownUntil
}

func (p *Pool) derivedBucketKeyLocked(acct *domain.Account, stateJSON string) string {
	if acct == nil || p.drivers == nil {
		return p.bucketKeyLocked(acct)
	}
	drv, ok := p.drivers[acct.Provider]
	if !ok {
		return p.bucketKeyLocked(acct)
	}
	projected := *acct
	projected.BucketKey = p.bucketKeyLocked(acct)
	projected.ProviderStateJSON = stateJSON
	if key := drv.BucketKey(&projected); key != "" {
		return key
	}
	return p.bucketKeyLocked(acct)
}

func (p *Pool) refreshBucketKeyLocked(acct *domain.Account, stateJSON string) {
	if acct == nil {
		return
	}
	oldKey := p.bucketKeyLocked(acct)
	newKey := p.derivedBucketKeyLocked(acct, stateJSON)
	acct.BucketKey = newKey
	if oldKey == newKey {
		return
	}

	if bucket, ok := p.buckets[oldKey]; ok {
		delete(p.buckets, oldKey)
		if _, exists := p.buckets[newKey]; !exists {
			bucket.BucketKey = newKey
			p.buckets[newKey] = bucket
			p.persistBucketLocked(bucket)
		}
		_ = p.store.DeleteQuotaBucket(context.Background(), oldKey)
	}
}

func (p *Pool) bucketAccountsLocked(acct *domain.Account) []*domain.Account {
	key := p.bucketKeyLocked(acct)
	if key == "" {
		return []*domain.Account{acct}
	}
	result := make([]*domain.Account, 0, 1)
	for _, candidate := range p.accounts {
		if candidate.Provider != acct.Provider {
			continue
		}
		if p.bucketKeyLocked(candidate) == key {
			result = append(result, candidate)
		}
	}
	if len(result) == 0 {
		return []*domain.Account{acct}
	}
	return result
}

func (p *Pool) bucketLocked(acct *domain.Account) *domain.QuotaBucket {
	if acct == nil {
		return nil
	}
	return p.buckets[p.bucketKeyLocked(acct)]
}

func (p *Pool) ensureBucketLocked(acct *domain.Account) *domain.QuotaBucket {
	if acct == nil {
		return nil
	}
	key := p.bucketKeyLocked(acct)
	if key == "" {
		return nil
	}
	if bucket, ok := p.buckets[key]; ok {
		return bucket
	}
	bucket := &domain.QuotaBucket{
		BucketKey: key,
		Provider:  acct.Provider,
		StateJSON: "{}",
		UpdatedAt: time.Now().UTC(),
	}
	p.buckets[key] = bucket
	p.persistBucketLocked(bucket)
	return bucket
}

func (p *Pool) projectAccountLocked(acct *domain.Account) *domain.Account {
	copy := *acct
	copy.BucketKey = p.bucketKeyLocked(acct)
	copy.CooldownUntil = nil
	copy.ProviderStateJSON = "{}"
	if bucket := p.bucketLocked(acct); bucket != nil {
		copy.CooldownUntil = bucket.CooldownUntil
		if bucket.StateJSON != "" {
			copy.ProviderStateJSON = bucket.StateJSON
		}
	}
	return &copy
}

func (p *Pool) persistBucketLocked(bucket *domain.QuotaBucket) {
	if bucket == nil {
		return
	}
	if bucket.StateJSON == "" {
		bucket.StateJSON = "{}"
	}
	if bucket.UpdatedAt.IsZero() {
		bucket.UpdatedAt = time.Now().UTC()
	}
	if err := p.store.SaveQuotaBucket(context.Background(), bucket); err != nil {
		slog.Error("pool bucket persist failed", "bucketKey", bucket.BucketKey, "error", err)
	}
}
