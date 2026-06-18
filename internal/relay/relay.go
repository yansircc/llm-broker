package relay

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/admission"
	"github.com/yansircc/llm-broker/internal/billing"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/requestlog"
)

type TransportProvider interface {
	ClientForAccount(acct *domain.Account) *http.Client
}

type StoreWriter interface {
	InsertRequestLog(ctx context.Context, log *domain.RequestLog) (int64, error)
}

type Config struct {
	MaxRequestBodyMB   int
	MaxRetryAccounts   int
	SessionBindingTTL  time.Duration
	CellErrorPause     time.Duration
	TraceCompat        bool
	RequestLogBlobDir  string
	RequestLogBlobMode requestlog.BlobMode
}

type Relay struct {
	pool      *pool.Pool
	tokens    TokenProvider
	store     StoreWriter
	cfg       Config
	transport TransportProvider
	bus       *events.Bus
	drivers   map[domain.Provider]driver.ExecutionDriver
	logFlush  sync.WaitGroup
	billing   *billing.Service
	admission *admission.Service
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

func (r *Relay) SetCommercialServices(b *billing.Service, a *admission.Service) {
	if r == nil {
		return
	}
	r.billing = b
	r.admission = a
}

func (r *Relay) driverFor(provider domain.Provider) driver.ExecutionDriver {
	return r.drivers[provider]
}

// WaitForLogFlush blocks until every pending request-log insert + on-disk
// write started by logRequestAsync has completed. Tests should defer this
// before t.TempDir cleanup so async file writes don't race the directory's
// removal.
func (r *Relay) WaitForLogFlush() {
	if r == nil {
		return
	}
	r.logFlush.Wait()
}

func (r *Relay) HandleProvider(provider domain.Provider) http.HandlerFunc {
	return r.HandleProviderSurface(provider, domain.SurfaceNative)
}

func (r *Relay) HandleProviderSurface(provider domain.Provider, surface domain.Surface) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		drv := r.driverFor(provider)
		if drv == nil {
			http.Error(w, "unknown provider", http.StatusNotFound)
			return
		}
		r.handleWithDriver(w, req, drv, surface)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
