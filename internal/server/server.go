package server

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
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
	requestSeq     atomic.Uint64
	activeRequests sync.Map
	connStates     sync.Map
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
		Handler:           srv.requestLogger(mux),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 30*time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ConnState:         srv.recordConnState,
	}

	return srv
}

type activeRequest struct {
	ID        uint64
	Method    string
	Path      string
	Remote    string
	StartedAt time.Time
}

func (s *Server) recordConnState(conn net.Conn, state http.ConnState) {
	if conn == nil {
		return
	}
	switch state {
	case http.StateClosed:
		s.connStates.Delete(conn)
	default:
		s.connStates.Store(conn, state)
	}
}

func (s *Server) snapshotConnStates() map[string]int {
	counts := map[string]int{
		"new":      0,
		"active":   0,
		"idle":     0,
		"hijacked": 0,
		"closed":   0,
	}
	s.connStates.Range(func(_, value any) bool {
		state, ok := value.(http.ConnState)
		if !ok {
			return true
		}
		switch state {
		case http.StateNew:
			counts["new"]++
		case http.StateActive:
			counts["active"]++
		case http.StateIdle:
			counts["idle"]++
		case http.StateHijacked:
			counts["hijacked"]++
		case http.StateClosed:
			counts["closed"]++
		}
		return true
	})
	return counts
}

func (s *Server) snapshotActiveRequests() []map[string]any {
	now := time.Now()
	out := make([]map[string]any, 0)
	s.activeRequests.Range(func(_, value any) bool {
		req, ok := value.(activeRequest)
		if !ok {
			return true
		}
		out = append(out, map[string]any{
			"id":      req.ID,
			"method":  req.Method,
			"path":    req.Path,
			"remote":  req.Remote,
			"age":     now.Sub(req.StartedAt).Round(time.Millisecond).String(),
			"started": req.StartedAt.UTC().Format(time.RFC3339Nano),
		})
		return true
	})
	return out
}
