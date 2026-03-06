package server

import (
	"context"
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
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/oauth"
	"github.com/yansir/cc-relayer/internal/pool"
	"github.com/yansir/cc-relayer/internal/relay"
	"github.com/yansir/cc-relayer/internal/store"
	"github.com/yansir/cc-relayer/internal/transport"
	"github.com/yansir/cc-relayer/internal/ui"
)

// Server is the main HTTP server.
type Server struct {
	cfg          *config.Config
	store        store.Store
	pool         *pool.Pool
	tokens       *oauth.TokenManager
	authMw       *auth.Middleware
	relay        *relay.Relay
	transportMgr *transport.Manager
	bus          *events.Bus
	httpServer   *http.Server
	version      string
	startTime    time.Time
}

func New(
	cfg *config.Config,
	s store.Store,
	p *pool.Pool,
	tm *oauth.TokenManager,
	r *relay.Relay,
	transportMgr *transport.Manager,
	authMw *auth.Middleware,
	bus *events.Bus,
	version string,
) *Server {
	srv := &Server{
		cfg:          cfg,
		store:        s,
		pool:         p,
		tokens:       tm,
		authMw:       authMw,
		relay:        r,
		transportMgr: transportMgr,
		bus:          bus,
		version:      version,
		startTime:    time.Now(),
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	srv.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:        requestLogger(mux),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   cfg.RequestTimeout + 30*time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return srv
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	auth := s.authMw.Authenticate

	// Root redirect
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/dashboard", http.StatusFound)
	})

	// Relay endpoints
	mux.Handle("POST /v1/messages", auth(http.HandlerFunc(s.relay.Handle)))
	mux.Handle("POST /v1/messages/count_tokens", auth(http.HandlerFunc(s.relay.HandleCountTokens)))
	mux.Handle("POST /openai/responses", auth(http.HandlerFunc(s.relay.HandleCodex)))

	// Telemetry sink
	mux.HandleFunc("POST /api/event_logging/batch", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	})

	// Admin: accounts
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
		slog.Warn("ui dist not found, /ui/ disabled", "error", err)
		return
	}
	indexHTML, _ := fs.ReadFile(distFS, "index.html")
	fileServer := http.StripPrefix("/ui/", http.FileServer(http.FS(distFS)))
	mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/ui/")
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
	})
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Background goroutines
	go s.pool.RunCleanup(ctx, 5*time.Minute)
	go s.transportMgr.RunCleanup(ctx)
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
