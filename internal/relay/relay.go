package relay

import (
	"context"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
)

type TransportProvider interface {
	ClientForAccount(acct *domain.Account) *http.Client
}

type StoreWriter interface {
	InsertRequestLog(ctx context.Context, log *domain.RequestLog) error
}

type Config struct {
	MaxRequestBodyMB  int
	MaxRetryAccounts  int
	SessionBindingTTL time.Duration
}

type Relay struct {
	pool      *pool.Pool
	tokens    TokenProvider
	store     StoreWriter
	cfg       Config
	transport TransportProvider
	bus       *events.Bus
	drivers   map[domain.Provider]driver.ExecutionDriver
}

type TokenProvider interface {
	EnsureValidToken(ctx context.Context, accountID string) (string, error)
}

func New(
	p *pool.Pool,
	tp TokenProvider,
	sw StoreWriter,
	cfg Config,
	transport TransportProvider,
	bus *events.Bus,
	drivers map[domain.Provider]driver.ExecutionDriver,
) *Relay {
	return &Relay{
		pool:      p,
		tokens:    tp,
		store:     sw,
		cfg:       cfg,
		transport: transport,
		bus:       bus,
		drivers:   drivers,
	}
}

func (r *Relay) driverFor(provider domain.Provider) driver.ExecutionDriver {
	return r.drivers[provider]
}

func (r *Relay) HandleProvider(provider domain.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		drv := r.driverFor(provider)
		if drv == nil {
			http.Error(w, "unknown provider", http.StatusNotFound)
			return
		}
		r.handleWithDriver(w, req, drv)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
