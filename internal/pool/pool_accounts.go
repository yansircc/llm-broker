package pool

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/events"
)

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func (p *Pool) ClearCooldown(accountID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return
	}
	bucket := p.ensureBucketLocked(acct)
	if bucket == nil || bucket.CooldownUntil == nil {
		return
	}
	bucket.CooldownUntil = nil
	bucket.UpdatedAt = time.Now().UTC()
	p.persistBucketLocked(bucket)
	p.bus.Publish(events.Event{
		Type: events.EventRecover, AccountID: acct.ID,
		Message: "admin cleared cooldown",
	})
	slog.Info("admin cleared cooldown", "accountId", acct.ID)
}

func (p *Pool) applyBucketCooldown(bucket *domain.QuotaBucket, proposed time.Time) {
	if bucket == nil {
		return
	}
	if bucket.CooldownUntil != nil && bucket.CooldownUntil.After(proposed) {
		return
	}
	until := proposed.UTC()
	bucket.CooldownUntil = &until
}

func (p *Pool) Update(accountID string, fn func(*domain.Account)) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}

	projected := p.projectAccountLocked(acct)
	fn(projected)

	acct.Email = projected.Email
	acct.Provider = projected.Provider
	acct.Status = projected.Status
	acct.Priority = projected.Priority
	acct.PriorityMode = projected.PriorityMode
	acct.ErrorMessage = projected.ErrorMessage
	acct.RefreshTokenEnc = projected.RefreshTokenEnc
	acct.AccessTokenEnc = projected.AccessTokenEnc
	acct.ExpiresAt = projected.ExpiresAt
	acct.CreatedAt = projected.CreatedAt
	acct.LastUsedAt = projected.LastUsedAt
	acct.LastRefreshAt = projected.LastRefreshAt
	acct.Proxy = projected.Proxy
	acct.ProxyJSON = projected.ProxyJSON
	acct.Identity = projected.Identity
	acct.IdentityJSON = projected.IdentityJSON
	acct.Subject = projected.Subject

	stateJSON := projected.ProviderStateJSON
	if stateJSON == "" {
		stateJSON = "{}"
	}
	p.refreshBucketKeyLocked(acct, stateJSON)
	bucket := p.ensureBucketLocked(acct)
	if bucket != nil && (bucket.StateJSON != stateJSON || !sameTime(bucket.CooldownUntil, projected.CooldownUntil)) {
		bucket.StateJSON = stateJSON
		bucket.CooldownUntil = projected.CooldownUntil
		bucket.UpdatedAt = time.Now().UTC()
		p.persistBucketLocked(bucket)
	}
	p.persistLocked(acct)
	return nil
}

func (p *Pool) Add(acct *domain.Account) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	stateJSON := acct.ProviderStateJSON
	if stateJSON == "" {
		stateJSON = "{}"
	}
	p.refreshBucketKeyLocked(acct, stateJSON)
	bucket := p.ensureBucketLocked(acct)
	if bucket != nil {
		bucket.StateJSON = stateJSON
		bucket.CooldownUntil = acct.CooldownUntil
		bucket.UpdatedAt = time.Now().UTC()
		p.persistBucketLocked(bucket)
	}
	acct.PersistRuntime()
	if err := p.store.SaveAccount(context.Background(), acct); err != nil {
		return err
	}
	acct.HydrateRuntime()
	acct.CooldownUntil = nil
	acct.ProviderStateJSON = ""
	p.accounts[acct.ID] = acct
	return nil
}

func (p *Pool) Delete(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	acct, ok := p.accounts[accountID]
	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}
	if err := p.store.DeleteAccount(context.Background(), accountID); err != nil {
		return err
	}
	delete(p.accounts, accountID)
	bucketKey := p.bucketKeyLocked(acct)
	if !p.bucketHasMembersLocked(bucketKey) {
		delete(p.buckets, bucketKey)
		_ = p.store.DeleteQuotaBucket(context.Background(), bucketKey)
	}
	return nil
}

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
	if bucket := p.ensureBucketLocked(acct); bucket != nil && bucket.CooldownUntil != nil {
		bucket.CooldownUntil = nil
		bucket.UpdatedAt = now
		p.persistBucketLocked(bucket)
	}
	p.persistLocked(acct)
	return nil
}

func (p *Pool) persistLocked(acct *domain.Account) {
	acct.PersistRuntime()
	if err := p.store.SaveAccount(context.Background(), acct); err != nil {
		slog.Error("pool persist failed", "accountId", acct.ID, "error", err)
	}
}

func (p *Pool) bucketHasMembersLocked(bucketKey string) bool {
	for _, acct := range p.accounts {
		if p.bucketKeyLocked(acct) == bucketKey {
			return true
		}
	}
	return false
}

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
