package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/relay"
	"github.com/yansircc/llm-broker/internal/store"
	"github.com/yansircc/llm-broker/internal/tokens"
	"github.com/yansircc/llm-broker/internal/transport"
)

// Server is the main HTTP server.
type Server struct {
	cfg            *config.Config
	store          store.Store
	pool           *pool.Pool
	tokens         *tokens.Manager
	authMw         *auth.Middleware
	relay          *relay.Relay
	transportPool  *transport.Pool
	bus            *events.Bus
	httpServer     *http.Server
	version        string
	startTime      time.Time
	catalogDrivers map[domain.Provider]driver.Descriptor
	oauthDrivers   map[domain.Provider]driver.OAuthDriver
	adminDrivers   map[domain.Provider]driver.AdminDriver
}

func New(
	cfg *config.Config,
	s store.Store,
	p *pool.Pool,
	tm *tokens.Manager,
	r *relay.Relay,
	transportPool *transport.Pool,
	authMw *auth.Middleware,
	bus *events.Bus,
	version string,
	drivers DriverViews,
) *Server {
	srv := &Server{
		cfg:            cfg,
		store:          s,
		pool:           p,
		tokens:         tm,
		authMw:         authMw,
		relay:          r,
		transportPool:  transportPool,
		bus:            bus,
		version:        version,
		startTime:      time.Now(),
		catalogDrivers: drivers.Catalog,
		oauthDrivers:   drivers.OAuth,
		adminDrivers:   drivers.Admin,
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	srv.httpServer = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           requestLogger(mux),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 30*time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return srv
}
