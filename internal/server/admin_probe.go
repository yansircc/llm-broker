package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/identity"
)

// doClaudeProbe sends a minimal haiku request to refresh rate limit headers.
func (s *Server) doClaudeProbe(ctx context.Context, acct *domain.Account, accessToken string) (*http.Response, error) {
	body := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.ClaudeAPIURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(req.Header, accessToken, s.cfg.ClaudeAPIVersion, s.cfg.ClaudeBetaHeader)
	return s.transportMgr.GetClient(acct).Do(req)
}

// doCodexProbe sends a minimal codex streaming request to refresh rate limit headers.
func (s *Server) doCodexProbe(ctx context.Context, acct *domain.Account, accessToken string) (*http.Response, error) {
	body := `{"model":"gpt-5-codex","stream":true,"store":false,"instructions":"Reply: ok","input":[{"role":"user","content":"t"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.CodexAPIURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Host", "chatgpt.com")
	req.Header.Set("Accept", "text/event-stream")
	if acct.ExtInfo != nil {
		if accountID, ok := acct.ExtInfo["chatgptAccountId"].(string); ok && accountID != "" {
			req.Header.Set("Chatgpt-Account-Id", accountID)
		}
	}
	return s.transportMgr.GetClient(acct).Do(req)
}

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
	now := time.Now().Unix()
	var wg sync.WaitGroup
	for _, acct := range accounts {
		if acct.Status != domain.StatusActive {
			continue
		}

		stale := false
		if acct.Provider == domain.ProviderCodex {
			stale = (acct.CodexPrimaryReset > 0 && acct.CodexPrimaryReset < now) ||
				(acct.CodexSecondaryReset > 0 && acct.CodexSecondaryReset < now) ||
				(acct.CodexPrimaryUtil > 0 && acct.CodexPrimaryReset == 0) ||
				(acct.CodexSecondaryUtil > 0 && acct.CodexSecondaryReset == 0)
		} else {
			stale = (acct.FiveHourReset > 0 && acct.FiveHourReset < now) ||
				(acct.SevenDayReset > 0 && acct.SevenDayReset < now) ||
				(acct.FiveHourUtil > 0 && acct.FiveHourReset == 0) ||
				(acct.SevenDayUtil > 0 && acct.SevenDayReset == 0)
		}
		if !stale {
			continue
		}

		wg.Add(1)
		go func(a *domain.Account) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			accessToken, err := s.tokens.EnsureValidToken(probeCtx, a.ID)
			if err != nil {
				slog.Warn("probe token failed", "account", a.Email, "error", err)
				return
			}

			var resp *http.Response
			if a.Provider == domain.ProviderCodex {
				resp, err = s.doCodexProbe(probeCtx, a, accessToken)
			} else {
				resp, err = s.doClaudeProbe(probeCtx, a, accessToken)
			}
			if err != nil {
				slog.Warn("probe request failed", "account", a.Email, "error", err)
				return
			}

			s.pool.ObserveSuccess(a.ID, resp.Header)
			resp.Body.Close()
			slog.Debug("probe refreshed", "account", a.Email, "status", resp.StatusCode)
		}(acct)
	}
	wg.Wait()
}

// fetchOrgUUIDViaAPI makes a minimal API call and extracts the org UUID.
func (s *Server) fetchOrgUUIDViaAPI(ctx context.Context, acct *domain.Account, accessToken string) string {
	resp, err := s.doClaudeProbe(ctx, acct, accessToken)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	return resp.Header.Get("Anthropic-Organization-Id")
}
