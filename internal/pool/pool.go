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
	mu             sync.RWMutex
	accounts       map[string]*domain.Account
	cells          map[string]*domain.EgressCell
	buckets        map[string]*domain.QuotaBucket
	serverErrCount map[string]int // consecutive upstream 500s per bucket key
	store          store.Store
	bus            *events.Bus

	onAuthFailure func(accountID string)
	drivers       map[domain.Provider]driver.SchedulerDriver
}

func (p *Pool) SetOnAuthFailure(fn func(accountID string)) {
	p.onAuthFailure = fn
}

func New(s store.Store, bus *events.Bus) (*Pool, error) {
	p := &Pool{
		accounts:       make(map[string]*domain.Account),
		cells:          make(map[string]*domain.EgressCell),
		buckets:        make(map[string]*domain.QuotaBucket),
		serverErrCount: make(map[string]int),
		store:          s,
		bus:            bus,
	}

	if err := p.refreshState(context.Background()); err != nil {
		return nil, fmt.Errorf("load pool state: %w", err)
	}

	slog.Info("pool loaded", "accounts", len(p.accounts))
	return p, nil
}

func (p *Pool) refreshState(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reloadStateLocked(ctx)
}

func (p *Pool) reloadStateLocked(ctx context.Context) error {
	accounts, err := p.store.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	accountMap := make(map[string]*domain.Account, len(accounts))
	for _, acct := range accounts {
		acct.HydrateRuntime()
		accountMap[acct.ID] = acct
	}

	cells, err := p.store.ListEgressCells(ctx)
	if err != nil {
		return fmt.Errorf("list egress cells: %w", err)
	}
	cellMap := make(map[string]*domain.EgressCell, len(cells))
	for _, cell := range cells {
		cell.HydrateRuntime()
		cellMap[cell.ID] = cell
	}

	buckets, err := p.store.ListQuotaBuckets(ctx)
	if err != nil {
		return fmt.Errorf("list quota buckets: %w", err)
	}
	bucketMap := make(map[string]*domain.QuotaBucket, len(buckets))
	for _, bucket := range buckets {
		if bucket.StateJSON == "" {
			bucket.StateJSON = "{}"
		}
		bucketMap[bucket.BucketKey] = bucket
	}

	p.accounts = accountMap
	p.cells = cellMap
	p.buckets = bucketMap
	for _, acct := range p.accounts {
		key := p.bucketKeyLocked(acct)
		if key == "" {
			continue
		}
		if _, ok := p.buckets[key]; ok {
			continue
		}
		p.buckets[key] = &domain.QuotaBucket{
			BucketKey: key,
			Provider:  acct.Provider,
			StateJSON: "{}",
			UpdatedAt: time.Now().UTC(),
		}
	}
	return nil
}

func (p *Pool) Get(id string) *domain.Account {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "get", "error", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	acct, ok := p.accounts[id]
	if !ok {
		return nil
	}
	return p.projectAccountLocked(acct)
}

func (p *Pool) List() []*domain.Account {
	if err := p.refreshState(context.Background()); err != nil {
		slog.Warn("pool refresh failed", "op", "list", "error", err)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*domain.Account, 0, len(p.accounts))
	for _, acct := range p.accounts {
		result = append(result, p.projectAccountLocked(acct))
	}
	return result
}
