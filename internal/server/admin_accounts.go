package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// handleListAccounts returns all accounts (without tokens).
func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := s.pool.List()
	cellCounts := accountCountsByCell(accounts)
	availability := s.pool.SurfaceAvailabilityMap()

	views := make([]AccountListItem, 0, len(accounts))
	for _, a := range accounts {
		proj := s.projectAccount(a)
		avail := availability[a.ID]
		views = append(views, AccountListItem{
			ID:              a.ID,
			Email:           a.Email,
			Provider:        string(a.Provider),
			Status:          string(a.Status),
			Weight:          proj.effectiveWeight,
			WeightMode:      a.PriorityMode,
			LastUsedAt:      a.LastUsedAt,
			CooldownUntil:   a.CooldownUntil,
			CellID:          a.CellID,
			AvailableNative: avail.Native,
			AvailableCompat: avail.Compat,
			Cell:            toCellSummary(a.Cell, cellCounts[a.CellID]),
			Windows:         proj.windows,
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
	hdrs, ok, err := s.pool.GetStainless(r.Context(), id)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load stainless binding")
		return
	}
	if ok {
		json.Unmarshal([]byte(hdrs), &stainless)
	}

	sessions, err := s.pool.ListSessionBindingsForAccount(r.Context(), id)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load session bindings")
		return
	}
	if sessions == nil {
		sessions = []domain.SessionBindingInfo{}
	}

	proj := s.projectAccount(acct)
	cellCounts := accountCountsByCell(s.pool.List())

	writeJSON(w, http.StatusOK, AccountDetailResponse{
		ID:             acct.ID,
		Email:          acct.Email,
		Provider:       acct.Provider,
		Subject:        acct.Subject,
		Status:         acct.Status,
		ProbeLabel:     proj.probeLabel,
		Weight:         acct.Priority,
		WeightMode:     acct.PriorityMode,
		AutoWeight:     proj.autoWeight,
		ErrorMessage:   acct.ErrorMessage,
		ProviderFields: proj.providerFields,
		CreatedAt:      acct.CreatedAt,
		LastUsedAt:     acct.LastUsedAt,
		LastRefreshAt:  acct.LastRefreshAt,
		ExpiresAt:      acct.ExpiresAt,
		CooldownUntil:  acct.CooldownUntil,
		CellID:         acct.CellID,
		Cell:           toCellSummary(acct.Cell, cellCounts[acct.CellID]),
		Windows:        proj.windows,
		Stainless:      stainless,
		Sessions:       sessions,
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
			return
		}
		a.ErrorMessage = ""
	}); err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	if req.Status == "active" {
		s.pool.ClearCooldown(id)
	}
	slog.Info("account status updated", "id", id, "status", req.Status)
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": req.Status})
}

func (s *Server) handleUpdateAccountWeight(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Mode   string `json:"mode"`
		Weight int    `json:"weight"`
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
	weight := req.Weight
	mode := req.Mode
	if err := s.pool.Update(id, func(a *domain.Account) {
		a.PriorityMode = mode
		if mode == "manual" {
			a.Priority = weight
		}
	}); err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	slog.Info("account weight updated", "id", id, "mode", mode, "weight", weight)
	writeJSON(w, http.StatusOK, struct {
		ID     string `json:"id"`
		Mode   string `json:"mode"`
		Weight int    `json:"weight"`
	}{id, mode, weight})
}

func (s *Server) handleBindAccountCell(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	var req struct {
		CellID string `json:"cell_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	req.CellID = strings.TrimSpace(req.CellID)
	if req.CellID != "" {
		cell := s.pool.GetCell(req.CellID)
		if cell == nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "cell not found")
			return
		}
		if req.CellID != acct.CellID {
			if reason := accountCellBindError(cell, time.Now().UTC()); reason != "" {
				writeAdminError(w, http.StatusBadRequest, "invalid_request", reason)
				return
			}
			if cell.Proxy == nil || cell.Proxy.Type != "socks5" {
				if accountOwnsCell(s.pool.List(), acct.ID, req.CellID, acct.Provider) {
					writeAdminError(w, http.StatusBadRequest, "invalid_request", "cell is already bound to another account of the same provider")
					return
				}
			}
		}
	}
	if err := s.pool.Update(id, func(a *domain.Account) {
		a.CellID = req.CellID
	}); err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	slog.Info("account cell updated", "id", id, "cellId", req.CellID)
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "cell_id": req.CellID})
}

func accountCellBindError(cell *domain.EgressCell, now time.Time) string {
	if cell == nil {
		return "cell not found"
	}
	status := cell.Status
	if status == "" {
		status = domain.EgressCellActive
	}
	if status != domain.EgressCellActive {
		return "cell is not active"
	}
	if cell.CooldownUntil != nil && now.Before(*cell.CooldownUntil) {
		return "cell is cooling down"
	}
	if cell.Proxy == nil || strings.TrimSpace(cell.Proxy.Host) == "" || cell.Proxy.Port <= 0 {
		return "cell has no usable proxy"
	}
	return ""
}

func accountOwnsCell(accounts []*domain.Account, accountID, cellID string, provider domain.Provider) bool {
	for _, acct := range accounts {
		if acct == nil || acct.ID == accountID {
			continue
		}
		if acct.CellID == cellID && acct.Provider == provider {
			return true
		}
	}
	return false
}

func (s *Server) handleRefreshAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}

	if _, err := s.tokens.ForceRefresh(r.Context(), id); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "token refresh failed: "+err.Error())
		return
	}
	slog.Info("account token force refreshed", "id", id)

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": "refreshed"})
}

func (s *Server) handleTestAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}

	wasBlocked := acct.Status == domain.StatusBlocked

	start := time.Now()
	_, err := s.probeAccount(r.Context(), acct)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, TestAccountResult{LatencyMs: latencyMs, Error: err.Error()})
		return
	}

	// If the account was blocked and the probe succeeded, recover all
	// blocked accounts in the same bucket. The ban is bucket-scoped
	// (e.g. entire org disabled), so a successful probe proves the
	// whole bucket is usable again.
	if wasBlocked {
		n := s.pool.RecoverBucket(id)
		slog.Info("blocked bucket recovered via successful probe", "id", id, "recovered", n)
	}

	writeJSON(w, http.StatusOK, TestAccountResult{OK: true, LatencyMs: latencyMs})
}

func (s *Server) handleUnbindSession(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if uuid == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "uuid is required")
		return
	}
	if err := s.pool.UnbindSession(r.Context(), uuid); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to unbind session")
		return
	}
	slog.Info("session unbound", "uuid", uuid)
	writeJSON(w, http.StatusOK, map[string]string{"unbound": uuid})
}
