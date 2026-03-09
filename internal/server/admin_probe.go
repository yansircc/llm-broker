package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
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
		go func(a *domain.Account, drv driver.Driver) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			accessToken, err := s.tokens.EnsureValidToken(probeCtx, a.ID)
			if err != nil {
				slog.Warn("probe token failed", "account", a.Email, "error", err)
				return
			}

			probeReq, err := drv.BuildProbeRequest(probeCtx, a, accessToken)
			if err != nil {
				slog.Warn("probe build request failed", "account", a.Email, "error", err)
				return
			}

			resp, err := s.transportMgr.GetClient(a).Do(probeReq)
			if err != nil {
				slog.Warn("probe request failed", "account", a.Email, "error", err)
				return
			}

			if resp.StatusCode == 403 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				effect := drv.Interpret(403, resp.Header, body, "")
				if effect.Kind == driver.EffectBlock {
					// Only observe bans from probe — ignore non-ban 403 (often transient)
					s.pool.Observe(a.ID, effect)
					slog.Warn("probe detected ban signal", "account", a.Email)
				} else {
					slog.Debug("probe got non-ban 403, ignoring", "account", a.Email)
				}
			} else {
				// Capture rate-limit headers on success or other statuses
				effect := drv.Interpret(http.StatusOK, resp.Header, nil, "")
				s.pool.Observe(a.ID, effect)
				resp.Body.Close()
			}
			slog.Debug("probe refreshed", "account", a.Email, "status", resp.StatusCode)
		}(acct, drv)
	}
	wg.Wait()
}

// fetchOrgUUIDViaAPI makes a minimal API call and extracts the org UUID.
func (s *Server) fetchOrgUUIDViaAPI(ctx context.Context, acct *domain.Account, accessToken string) string {
	drv, ok := s.drivers[acct.Provider]
	if !ok {
		return ""
	}
	probeReq, err := drv.BuildProbeRequest(ctx, acct, accessToken)
	if err != nil {
		return ""
	}
	resp, err := s.transportMgr.GetClient(acct).Do(probeReq)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	return resp.Header.Get("Anthropic-Organization-Id")
}
