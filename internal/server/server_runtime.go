package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/neterr"
	"github.com/yansircc/llm-broker/internal/requestid"
)

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	go s.transportPool.RunCleanup(ctx)
	go s.runBackgroundJobs(ctx)

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
		active := s.snapshotActiveRequests()
		sort.Slice(active, func(i, j int) bool {
			return active[i]["started"].(string) < active[j]["started"].(string)
		})
		slog.Info("shutdown start", "activeRequests", active, "connStates", s.snapshotConnStates())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown timeout", "error", err, "activeRequests", s.snapshotActiveRequests(), "connStates", s.snapshotConnStates())
			return err
		}
		slog.Info("shutdown complete", "connStates", s.snapshotConnStates())
		return nil
	}
}

func (s *Server) runBackgroundJobs(ctx context.Context) {
	switch s.cfg.BackgroundJobsMode {
	case "off":
		slog.Info("background jobs disabled")
		return
	case "leader":
		s.runLeaderBackgroundJobs(ctx)
	default:
		slog.Info("background jobs mode", "mode", "all")
		s.runBackgroundJobWorkers(ctx)
	}
}

func (s *Server) runBackgroundJobWorkers(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		s.pool.RunCleanup(ctx, 5*time.Minute)
	}()
	go func() {
		defer wg.Done()
		s.runLogPurge(ctx)
	}()
	go func() {
		defer wg.Done()
		s.runRateLimitRefresh(ctx)
	}()
	wg.Wait()
}

func (s *Server) runLeaderBackgroundJobs(ctx context.Context) {
	lockPath := s.cfg.BackgroundLeaderLockPath
	if lockPath == "" {
		slog.Info("background leader mode without lock path; running jobs locally")
		s.runBackgroundJobWorkers(ctx)
		return
	}

	waitLogged := false
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			slog.Error("background leader lock mkdir failed", "path", lockPath, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			slog.Error("background leader lock open failed", "path", lockPath, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			lockFile.Close()
			if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
				if !waitLogged {
					slog.Info("background leader waiting", "path", lockPath)
					waitLogged = true
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				continue
			}
			slog.Error("background leader lock failed", "path", lockPath, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		slog.Info("background leader acquired", "path", lockPath)
		s.runBackgroundJobWorkers(ctx)
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
		slog.Info("background leader released", "path", lockPath)
		return
	}
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = requestid.Ensure(r, w)
		reqID := s.requestSeq.Add(1)
		s.activeRequests.Store(reqID, activeRequest{
			ID:        reqID,
			Method:    r.Method,
			Path:      r.URL.Path,
			Remote:    r.RemoteAddr,
			StartedAt: time.Now(),
		})
		defer s.activeRequests.Delete(reqID)
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
