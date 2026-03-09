package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// runRateLimitRefresh periodically probes accounts whose reset window has expired.
func (s *Server) runRateLimitRefresh(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshStaleAccounts(ctx)
		}
	}
}

func (s *Server) refreshStaleAccounts(ctx context.Context) {
	accounts := s.pool.List()
	now := time.Now()
	var wg sync.WaitGroup
	for _, acct := range accounts {
		if acct.Status != domain.StatusActive {
			continue
		}

		drv, ok := s.drivers[acct.Provider]
		if !ok {
			continue
		}

		if !drv.IsStale(json.RawMessage(acct.ProviderStateJSON), now) {
			continue
		}

		wg.Add(1)
		go func(a *domain.Account) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			result, err := s.probeAccount(probeCtx, a)
			if err != nil {
				slog.Warn("probe failed", "account", a.Email, "error", err)
				return
			}
			slog.Debug("probe refreshed", "account", a.Email, "observed", result.Observe)
		}(acct)
	}
	wg.Wait()
}
