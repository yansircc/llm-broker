package pool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/store"
)

type SessionBinding struct {
	AccountID  string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

type Exclusion struct {
	AccountID string
	BucketKey string
}

func ExcludeAccount(accountID string) Exclusion {
	return Exclusion{AccountID: accountID}
}

func ExcludeBucket(bucketKey string) Exclusion {
	return Exclusion{BucketKey: bucketKey}
}

type Pool struct {
	mu       sync.RWMutex
	accounts map[string]*domain.Account
	buckets  map[string]*domain.QuotaBucket
	store    store.Store
	bus      *events.Bus

	sessions      *store.TTLMap[SessionBinding]
	stainless     *store.TTLMap[string]
	oauthSessions *store.TTLMap[string]
	refreshLocks  *store.TTLMap[string]

	onAuthFailure func(accountID string)
	drivers       map[domain.Provider]driver.SchedulerDriver
}

func (p *Pool) SetOnAuthFailure(fn func(accountID string)) {
	p.onAuthFailure = fn
}

func New(s store.Store, bus *events.Bus) (*Pool, error) {
	p := &Pool{
		accounts:      make(map[string]*domain.Account),
		buckets:       make(map[string]*domain.QuotaBucket),
		store:         s,
		bus:           bus,
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

	buckets, err := s.ListQuotaBuckets(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load quota buckets: %w", err)
	}
	for _, bucket := range buckets {
		if bucket.StateJSON == "" {
			bucket.StateJSON = "{}"
		}
		p.buckets[bucket.BucketKey] = bucket
	}
	for _, acct := range p.accounts {
		p.ensureBucketLocked(acct)
	}

	slog.Info("pool loaded", "accounts", len(p.accounts))
	return p, nil
}

func (p *Pool) Get(id string) *domain.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[id]
	if !ok {
		return nil
	}
	return p.projectAccountLocked(acct)
}

func (p *Pool) List() []*domain.Account {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*domain.Account, 0, len(p.accounts))
	for _, acct := range p.accounts {
		result = append(result, p.projectAccountLocked(acct))
	}
	return result
}
