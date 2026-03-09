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
)

// handleListAccounts returns all accounts (without tokens).
func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := s.pool.List()

	views := make([]AccountListItem, 0, len(accounts))
	for _, a := range accounts {
		views = append(views, AccountListItem{
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
	if sessions == nil {
		sessions = []domain.SessionBindingInfo{}
	}

	var autoScore int
	if acct.PriorityMode == "auto" {
		if drv, ok := s.drivers[acct.Provider]; ok {
			autoScore = drv.AutoPriority(json.RawMessage(acct.ProviderStateJSON))
		}
	}

	writeJSON(w, http.StatusOK, AccountDetailResponse{
		ID:                 acct.ID,
		Email:              acct.Email,
		Provider:           acct.Provider,
		Status:             acct.Status,
		Priority:           acct.Priority,
		PriorityMode:       acct.PriorityMode,
		AutoScore:          autoScore,
		Schedulable:        acct.Schedulable,
		ErrorMessage:       acct.ErrorMessage,
		ExtInfo:            acct.ExtInfo,
		CreatedAt:          acct.CreatedAt,
		LastUsedAt:         acct.LastUsedAt,
		LastRefreshAt:      acct.LastRefreshAt,
		ExpiresAt:          acct.ExpiresAt,
		FiveHourStatus:     acct.FiveHourStatus,
		OverloadedUntil:    acct.OverloadedUntil,
		OpusRateLimitEndAt: acct.OpusRateLimitEndAt,
		Stainless:          stainless,
		Sessions:           sessions,
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
			a.OverloadedUntil = nil
			a.OverloadedAt = nil
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
	writeJSON(w, http.StatusOK, struct {
		ID       string `json:"id"`
		Mode     string `json:"mode"`
		Priority int    `json:"priority"`
	}{id, mode, priority})
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

	// Back-fill Subject if empty (pre-migration accounts)
	if acct.Subject == "" {
		switch acct.Provider {
		case domain.ProviderClaude:
			// Claude: fetch orgUUID via API probe
			orgUUID := s.fetchOrgUUIDViaAPI(r.Context(), acct, accessToken)
			if orgUUID != "" {
				_ = s.pool.Update(id, func(a *domain.Account) {
					a.Subject = orgUUID
					if a.ExtInfo == nil {
						a.ExtInfo = make(map[string]interface{})
					}
					a.ExtInfo["orgUUID"] = orgUUID
				})
				slog.Info("account subject back-filled", "id", id, "subject", orgUUID)
			}
		case domain.ProviderCodex:
			// Codex: extract chatgptAccountId from existing ExtInfo
			if acct.ExtInfo != nil {
				if chatgptID, ok := acct.ExtInfo["chatgptAccountId"].(string); ok && chatgptID != "" {
					_ = s.pool.Update(id, func(a *domain.Account) {
						a.Subject = chatgptID
					})
					slog.Info("account subject back-filled", "id", id, "subject", chatgptID)
				}
			}
		}
	} else if acct.Provider != domain.ProviderCodex && (acct.ExtInfo == nil || acct.ExtInfo["orgUUID"] == nil || acct.ExtInfo["orgUUID"] == "") {
		// Back-fill org UUID in ExtInfo if missing (Claude accounts with Subject already set)
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
		writeJSON(w, http.StatusOK, TestAccountResult{Error: "token unavailable: " + err.Error()})
		return
	}

	drv, ok := s.drivers[acct.Provider]
	if !ok {
		writeJSON(w, http.StatusOK, TestAccountResult{Error: "unknown provider"})
		return
	}

	probeReq, err := drv.BuildProbeRequest(r.Context(), acct, accessToken)
	if err != nil {
		writeJSON(w, http.StatusOK, TestAccountResult{Error: "failed to create request"})
		return
	}

	client := s.transportMgr.GetClient(acct)
	start := time.Now()
	resp, err := client.Do(probeReq)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, TestAccountResult{LatencyMs: latencyMs, Error: err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		effect := drv.Interpret(resp.StatusCode, resp.Header, body, "")
		s.pool.Observe(acct.ID, effect)
		writeJSON(w, http.StatusOK, TestAccountResult{LatencyMs: latencyMs, Error: fmt.Sprintf("upstream returned %d", resp.StatusCode)})
		return
	}

	// Success — capture rate-limit headers
	effect := drv.Interpret(http.StatusOK, resp.Header, nil, "")
	s.pool.Observe(acct.ID, effect)

	// For Codex streaming probes, verify we got output before clearing overload
	if acct.Provider == domain.ProviderCodex {
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		gotOutput := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: response.output_text.delta") {
				gotOutput = true
				break
			}
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"error":{`) {
				writeJSON(w, http.StatusOK, TestAccountResult{
					LatencyMs: time.Since(start).Milliseconds(),
					Error:     "upstream error in stream",
				})
				return
			}
		}
		if !gotOutput {
			writeJSON(w, http.StatusOK, TestAccountResult{LatencyMs: time.Since(start).Milliseconds(), Error: "stream ended without output"})
			return
		}
	}

	// All validation passed — clear overload
	s.pool.ClearOverload(acct.ID)

	writeJSON(w, http.StatusOK, TestAccountResult{OK: true, LatencyMs: latencyMs})
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
