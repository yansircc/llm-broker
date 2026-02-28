package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/identity"
	"github.com/yansir/cc-relayer/internal/pool"
)

// handleListAccounts returns all accounts (without tokens).
func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := s.pool.List()

	type accountView struct {
		ID                 string                 `json:"id"`
		Email              string                 `json:"email"`
		Provider           string                 `json:"provider"`
		Status             string                 `json:"status"`
		Priority           int                    `json:"priority"`
		PriorityMode       string                 `json:"priority_mode"`
		Schedulable        bool                   `json:"schedulable"`
		ExtInfo            map[string]interface{} `json:"ext_info,omitempty"`
		LastUsedAt         *time.Time             `json:"last_used_at,omitempty"`
		OverloadedUntil    *time.Time             `json:"overloaded_until,omitempty"`
		FiveHourStatus     string                 `json:"five_hour_status"`
		OpusRateLimitEndAt *time.Time             `json:"opus_rate_limit_end_at,omitempty"`
	}

	views := make([]accountView, 0, len(accounts))
	for _, a := range accounts {
		views = append(views, accountView{
			ID:                 a.ID,
			Email:              a.Email,
			Provider:           string(a.Provider),
			Status:             string(a.Status),
			Priority:           a.Priority,
			PriorityMode:       a.PriorityMode,
			Schedulable:        a.Schedulable,
			ExtInfo:            a.ExtInfo,
			LastUsedAt:         a.LastUsedAt,
			OverloadedUntil:    a.OverloadedUntil,
			FiveHourStatus:     a.FiveHourStatus,
			OpusRateLimitEndAt: a.OpusRateLimitEndAt,
		})
	}
	writeJSON(w, http.StatusOK, views)
}

// handleDeleteAccount removes an account by ID.
func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	if err := s.pool.Delete(id); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to delete account")
		return
	}
	slog.Info("account deleted", "id", id, "email", acct.Email)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}

	var stainless map[string]interface{}
	if hdrs, ok := s.pool.GetStainless(id); ok {
		json.Unmarshal([]byte(hdrs), &stainless)
	}

	sessions := s.pool.ListSessionBindingsForAccount(id)

	var autoScore int
	if acct.PriorityMode == "auto" {
		autoScore = pool.AutoPriority(acct)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":                    acct.ID,
		"email":                 acct.Email,
		"provider":              acct.Provider,
		"status":                acct.Status,
		"priority":              acct.Priority,
		"priority_mode":         acct.PriorityMode,
		"auto_score":            autoScore,
		"schedulable":           acct.Schedulable,
		"error_message":         acct.ErrorMessage,
		"ext_info":              acct.ExtInfo,
		"created_at":            acct.CreatedAt,
		"last_used_at":          acct.LastUsedAt,
		"last_refresh_at":       acct.LastRefreshAt,
		"expires_at":            acct.ExpiresAt,
		"five_hour_status":      acct.FiveHourStatus,
		"overloaded_until":      acct.OverloadedUntil,
		"opus_rate_limit_end_at": acct.OpusRateLimitEndAt,
		"stainless":             stainless,
		"sessions":              sessions,
	})
}

func (s *Server) handleUpdateAccountEmail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || len(req.Email) > 100 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "email must be 1-100 characters")
		return
	}
	if err := s.pool.Update(id, func(a *domain.Account) {
		a.Email = req.Email
	}); err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	slog.Info("account email updated", "id", id, "email", req.Email)
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "email": req.Email})
}

func (s *Server) handleUpdateAccountStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Status != "active" && req.Status != "disabled") {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "status must be 'active' or 'disabled'")
		return
	}
	if err := s.pool.Update(id, func(a *domain.Account) {
		a.Status = domain.Status(req.Status)
		if req.Status == "disabled" {
			a.Schedulable = false
		} else {
			a.Schedulable = true
			a.ErrorMessage = ""
		}
	}); err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	slog.Info("account status updated", "id", id, "status", req.Status)
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": req.Status})
}

func (s *Server) handleUpdateAccountPriority(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Mode     string `json:"mode"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if req.Mode == "" {
		req.Mode = "manual"
	}
	if req.Mode != "auto" && req.Mode != "manual" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "mode must be 'auto' or 'manual'")
		return
	}
	priority := req.Priority
	mode := req.Mode
	if err := s.pool.Update(id, func(a *domain.Account) {
		a.PriorityMode = mode
		if mode == "manual" {
			a.Priority = priority
		}
	}); err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	slog.Info("account priority updated", "id", id, "mode", mode, "priority", priority)
	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "mode": mode, "priority": priority})
}

func (s *Server) handleRefreshAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}

	accessToken, err := s.tokens.ForceRefresh(r.Context(), id)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "token refresh failed: "+err.Error())
		return
	}
	slog.Info("account token force refreshed", "id", id)

	// Back-fill org UUID if missing (Claude accounts only)
	if acct.Provider != domain.ProviderCodex && (acct.ExtInfo == nil || acct.ExtInfo["orgUUID"] == nil || acct.ExtInfo["orgUUID"] == "") {
		orgUUID := s.fetchOrgUUIDViaAPI(r.Context(), acct, accessToken)
		if orgUUID != "" {
			_ = s.pool.Update(id, func(a *domain.Account) {
				if a.ExtInfo == nil {
					a.ExtInfo = make(map[string]interface{})
				}
				a.ExtInfo["orgUUID"] = orgUUID
			})
			slog.Info("account org UUID back-filled", "id", id, "orgUUID", orgUUID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": "refreshed"})
}

func (s *Server) handleTestAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}

	accessToken, err := s.tokens.EnsureValidToken(r.Context(), acct.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":    false,
			"error": "token unavailable: " + err.Error(),
		})
		return
	}

	if acct.Provider == domain.ProviderCodex {
		s.testCodexAccount(w, r, acct, accessToken)
		return
	}

	// Claude test request
	testBody := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	testReq, err := http.NewRequestWithContext(r.Context(), "POST", s.cfg.ClaudeAPIURL, strings.NewReader(testBody))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "error": "failed to create request"})
		return
	}
	testReq.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(testReq.Header, accessToken, s.cfg.ClaudeAPIVersion, s.cfg.ClaudeBetaHeader)

	client := s.transportMgr.GetClient(acct)
	start := time.Now()
	resp, err := client.Do(testReq)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "latency_ms": latencyMs, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	s.pool.ObserveSuccess(acct.ID, resp.Header)

	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": false, "latency_ms": latencyMs,
			"error": fmt.Sprintf("upstream returned %d", resp.StatusCode),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "latency_ms": latencyMs})
}

func (s *Server) testCodexAccount(w http.ResponseWriter, r *http.Request, acct *domain.Account, accessToken string) {
	testBody := `{"model":"gpt-5-codex","stream":true,"store":false,"instructions":"Reply with only: ok","input":[{"role":"user","content":"test"}]}`
	testReq, err := http.NewRequestWithContext(r.Context(), "POST", s.cfg.CodexAPIURL, strings.NewReader(testBody))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "error": "failed to create request"})
		return
	}
	testReq.Header.Set("Content-Type", "application/json")
	testReq.Header.Set("Authorization", "Bearer "+accessToken)
	testReq.Header.Set("Host", "chatgpt.com")
	testReq.Header.Set("Accept", "text/event-stream")
	if acct.ExtInfo != nil {
		if accountID, ok := acct.ExtInfo["chatgptAccountId"].(string); ok && accountID != "" {
			testReq.Header.Set("Chatgpt-Account-Id", accountID)
		}
	}

	client := s.transportMgr.GetClient(acct)
	start := time.Now()
	resp, err := client.Do(testReq)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "latency_ms": latencyMs, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	s.pool.ObserveSuccess(acct.ID, resp.Header)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": false, "latency_ms": latencyMs,
			"error": fmt.Sprintf("codex upstream returned %d: %s", resp.StatusCode, truncateStr(string(body), 200)),
		})
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	gotOutput := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: response.output_text.delta") {
			gotOutput = true
			break
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"error":{`) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok": false, "latency_ms": time.Since(start).Milliseconds(),
				"error": "upstream error in stream",
			})
			return
		}
	}

	latencyMs = time.Since(start).Milliseconds()
	if gotOutput {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "latency_ms": latencyMs})
	} else {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "latency_ms": latencyMs, "error": "stream ended without output"})
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *Server) handleUnbindSession(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if uuid == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "uuid is required")
		return
	}
	s.pool.UnbindSession(uuid)
	slog.Info("session unbound", "uuid", uuid)
	writeJSON(w, http.StatusOK, map[string]string{"unbound": uuid})
}
