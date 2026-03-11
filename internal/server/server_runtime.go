package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/neterr"
)

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	go s.pool.RunCleanup(ctx, 5*time.Minute)
	go s.transportPool.RunCleanup(ctx)
	go s.runLogPurge(ctx)
	go s.runRateLimitRefresh(ctx)

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

func (s *Server) probeAccount(ctx context.Context, acct *domain.Account) (driver.ProbeResult, error) {
	accessToken, err := s.tokens.EnsureValidToken(ctx, acct.ID)
	if err != nil {
		return driver.ProbeResult{}, fmt.Errorf("token unavailable: %w", err)
	}

	drv, ok := s.adminDrivers[acct.Provider]
	if !ok {
		return driver.ProbeResult{}, fmt.Errorf("unknown provider")
	}

	result, err := drv.Probe(ctx, acct, accessToken, s.transportPool.ClientForAccount(acct))
	if err != nil && !result.Observe && acct.CellID != "" && s.cfg.CellErrorPause > 0 && neterr.IsTransport(err) {
		s.pool.CooldownCell(acct.CellID, time.Now().Add(s.cfg.CellErrorPause), fmt.Sprintf("probe transport error on account %s: %v", acct.Email, err))
	}
	if result.Observe {
		s.pool.Observe(acct.ID, result.Effect)
	}
	if result.ClearCooldown {
		s.pool.ClearCooldown(acct.ID)
	}
	return result, err
}
