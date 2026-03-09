package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yansir/cc-relayer/internal/auth"
	"github.com/yansir/cc-relayer/internal/config"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/pool"
	"github.com/yansir/cc-relayer/internal/relay"
	"github.com/yansir/cc-relayer/internal/store"
	"github.com/yansir/cc-relayer/internal/tokens"
	"github.com/yansir/cc-relayer/internal/transport"
	"github.com/yansir/cc-relayer/internal/ui"
)

// Server is the main HTTP server.
type Server struct {
	cfg           *config.Config
	store         store.Store
	pool          *pool.Pool
	tokens        *tokens.Manager
	authMw        *auth.Middleware
	relay         *relay.Relay
	transportPool *transport.Pool
	bus           *events.Bus
	httpServer    *http.Server
	version       string
	startTime     time.Time
	drivers       map[domain.Provider]driver.Driver
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
	drivers map[domain.Provider]driver.Driver,
) *Server {
	srv := &Server{
		cfg:           cfg,
		store:         s,
		pool:          p,
		tokens:        tm,
		authMw:        authMw,
		relay:         r,
		transportPool: transportPool,
		bus:           bus,
		version:       version,
		startTime:     time.Now(),
		drivers:       drivers,
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

func (s *Server) registerRoutes(mux *http.ServeMux) {
	auth := s.authMw.Authenticate

	// Models endpoint (authenticated relay metadata)
	mux.Handle("GET /v1/models", auth(http.HandlerFunc(s.handleListModels)))

	// Relay endpoints (registered explicitly from driver info)
	for _, provider := range sortedDriverProviders(s.drivers) {
		drv := s.drivers[provider]
		for _, path := range drv.Info().RelayPaths {
			mux.Handle("POST "+path, auth(http.HandlerFunc(s.relay.HandleProvider(provider))))
		}
	}

	// Telemetry sink
	mux.HandleFunc("POST /api/event_logging/batch", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	})

	// Admin: accounts
	mux.Handle("GET /admin/providers", auth(http.HandlerFunc(s.handleListProviders)))
	mux.Handle("POST /admin/accounts/generate-auth-url", auth(http.HandlerFunc(s.handleGenerateAuthURL)))
	mux.Handle("POST /admin/accounts/exchange-code", auth(http.HandlerFunc(s.handleExchangeCode)))
	mux.Handle("GET /admin/accounts", auth(http.HandlerFunc(s.handleListAccounts)))
	mux.Handle("GET /admin/accounts/{id}", auth(http.HandlerFunc(s.handleGetAccount)))
	mux.Handle("DELETE /admin/accounts/{id}", auth(http.HandlerFunc(s.handleDeleteAccount)))
	mux.Handle("POST /admin/accounts/{id}/email", auth(http.HandlerFunc(s.handleUpdateAccountEmail)))
	mux.Handle("POST /admin/accounts/{id}/status", auth(http.HandlerFunc(s.handleUpdateAccountStatus)))
	mux.Handle("POST /admin/accounts/{id}/priority", auth(http.HandlerFunc(s.handleUpdateAccountPriority)))
	mux.Handle("POST /admin/accounts/{id}/refresh", auth(http.HandlerFunc(s.handleRefreshAccount)))
	mux.Handle("POST /admin/accounts/{id}/test", auth(http.HandlerFunc(s.handleTestAccount)))

	// Admin: events
	mux.Handle("DELETE /admin/events", auth(http.HandlerFunc(s.handleClearEvents)))

	// Admin: login
	mux.HandleFunc("POST /admin/login", s.handleLogin)

	// Admin: users
	mux.Handle("POST /admin/users", auth(http.HandlerFunc(s.handleCreateUser)))
	mux.Handle("GET /admin/users", auth(http.HandlerFunc(s.handleListUsers)))
	mux.Handle("GET /admin/users/{id}", auth(http.HandlerFunc(s.handleGetUser)))
	mux.Handle("DELETE /admin/users/{id}", auth(http.HandlerFunc(s.handleDeleteUser)))
	mux.Handle("POST /admin/users/{id}/regenerate", auth(http.HandlerFunc(s.handleRegenerateUserToken)))
	mux.Handle("POST /admin/users/{id}/status", auth(http.HandlerFunc(s.handleUpdateUserStatus)))

	// Admin: dashboard & health
	mux.Handle("GET /admin/dashboard", auth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /admin/health", auth(http.HandlerFunc(s.handleHealth)))

	// Admin: session unbinding
	mux.Handle("DELETE /admin/sessions/binding/{uuid}", auth(http.HandlerFunc(s.handleUnbindSession)))

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := s.store.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"error","store":"%s"}`, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// WebUI
	distFS, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		slog.Warn("ui dist not found, root UI disabled", "error", err)
		return
	}
	indexHTML, _ := fs.ReadFile(distFS, "index.html")
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		if isReservedUIPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		serveUI(distFS, indexHTML, fileServer, w, r)
	})
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Background goroutines
	go s.pool.RunCleanup(ctx, 5*time.Minute)
	go s.transportPool.RunCleanup(ctx)
	go s.runLogPurge(ctx)
	go s.runRateLimitRefresh(ctx)

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", s.httpServer.Addr)
		errCh <- s.httpServer.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		slog.Info("shutdown signal received", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func serveUI(distFS fs.FS, indexHTML []byte, fileServer http.Handler, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(indexHTML)
		return
	}
	if strings.HasPrefix(path, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if _, err := fs.Stat(distFS, path); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(indexHTML)
		return
	}
	fileServer.ServeHTTP(w, r)
}

func isReservedUIPath(path string) bool {
	switch {
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return true
	case path == "/api" || strings.HasPrefix(path, "/api/"):
		return true
	case path == "/v1" || strings.HasPrefix(path, "/v1/"):
		return true
	case path == "/openai" || strings.HasPrefix(path, "/openai/"):
		return true
	case path == "/ui" || strings.HasPrefix(path, "/ui/"):
		return true
	case path == "/add-account" || path == "/add-account/":
		return true
	case path == "/health":
		return true
	default:
		return false
	}
}

type modelsResponse struct {
	Object string         `json:"object"`
	Data   []driver.Model `json:"data"`
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data := make([]driver.Model, 0)
	for _, provider := range sortedDriverProviders(s.drivers) {
		data = append(data, s.drivers[provider].Models()...)
	}
	json.NewEncoder(w).Encode(modelsResponse{Object: "list", Data: data})
}

func (s *Server) runLogPurge(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			before := time.Now().Add(-30 * 24 * time.Hour)
			n, err := s.store.PurgeOldLogs(ctx, before)
			if err != nil {
				slog.Error("purge old logs failed", "error", err)
			} else if n > 0 {
				slog.Info("purged old request logs", "count", n)
			}
		}
	}
}

func (s *Server) probeAccount(ctx context.Context, acct *domain.Account) (driver.ProbeResult, error) {
	accessToken, err := s.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		return driver.ProbeResult{}, fmt.Errorf("token unavailable: %w", err)
	}

	drv, ok := s.drivers[acct.Provider]
	if !ok {
		return driver.ProbeResult{}, fmt.Errorf("unknown provider")
	}

	result, err := drv.Probe(ctx, acct, accessToken, s.transportPool.ClientForAccount(acct))
	if result.Observe {
		s.pool.Observe(acct.ID, result.Effect)
	}
	if result.ClearCooldown {
		s.pool.ClearCooldown(acct.ID)
	}
	return result, err
}
