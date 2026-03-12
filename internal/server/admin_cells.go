package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *Server) handleListEgressCells(w http.ResponseWriter, r *http.Request) {
	accounts := s.pool.List()
	cells := s.pool.ListCells()
	counts := accountCountsByCell(accounts)

	resp := make([]EgressCellResponse, 0, len(cells))
	for _, cell := range cells {
		item := EgressCellResponse{
			ID:            cell.ID,
			Name:          cell.Name,
			Status:        string(cell.Status),
			Proxy:         cell.Proxy,
			Labels:        cell.Labels,
			CooldownUntil: cell.CooldownUntil,
			StateJSON:     cell.StateJSON,
			CreatedAt:     cell.CreatedAt,
			UpdatedAt:     cell.UpdatedAt,
			Accounts:      make([]EgressCellAccountRef, 0, counts[cell.ID]),
		}
		for _, acct := range accounts {
			if acct.CellID != cell.ID {
				continue
			}
			item.Accounts = append(item.Accounts, EgressCellAccountRef{
				ID:       acct.ID,
				Email:    acct.Email,
				Provider: string(acct.Provider),
				Status:   string(acct.Status),
			})
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpsertEgressCell(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string              `json:"id"`
		Name   string              `json:"name"`
		Status string              `json:"status"`
		Proxy  *domain.ProxyConfig `json:"proxy"`
		Labels map[string]string   `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.Status = strings.TrimSpace(req.Status)
	if req.ID == "" || req.Name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "id and name are required")
		return
	}
	if req.Status == "" {
		req.Status = string(domain.EgressCellActive)
	}
	if req.Status != string(domain.EgressCellActive) && req.Status != string(domain.EgressCellDisabled) && req.Status != string(domain.EgressCellError) {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "status must be active, disabled, or error")
		return
	}
	if req.Proxy != nil && (req.Proxy.Host == "" || req.Proxy.Port <= 0) {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "proxy host and port are required")
		return
	}

	cell := &domain.EgressCell{
		ID:     req.ID,
		Name:   req.Name,
		Status: domain.EgressCellStatus(req.Status),
		Proxy:  req.Proxy,
		Labels: req.Labels,
	}
	if existing := s.pool.GetCell(req.ID); existing != nil {
		cell.CreatedAt = existing.CreatedAt
		cell.CooldownUntil = existing.CooldownUntil
		cell.StateJSON = existing.StateJSON
	}
	if err := s.pool.SaveCell(cell); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to save cell")
		return
	}

	saved := s.pool.GetCell(req.ID)
	writeJSON(w, http.StatusOK, EgressCellResponse{
		ID:            saved.ID,
		Name:          saved.Name,
		Status:        string(saved.Status),
		Proxy:         saved.Proxy,
		Labels:        saved.Labels,
		CooldownUntil: saved.CooldownUntil,
		StateJSON:     saved.StateJSON,
		CreatedAt:     saved.CreatedAt,
		UpdatedAt:     saved.UpdatedAt,
		Accounts:      []EgressCellAccountRef{},
	})
}

func (s *Server) handleClearEgressCellCooldown(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "cell id is required")
		return
	}
	if s.pool.GetCell(id) == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "cell not found")
		return
	}
	if !s.pool.ClearCellCooldown(id) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      id,
			"cleared": false,
		})
		return
	}
	cell := s.pool.GetCell(id)
	writeJSON(w, http.StatusOK, EgressCellResponse{
		ID:            cell.ID,
		Name:          cell.Name,
		Status:        string(cell.Status),
		Proxy:         cell.Proxy,
		Labels:        cell.Labels,
		CooldownUntil: cell.CooldownUntil,
		StateJSON:     cell.StateJSON,
		CreatedAt:     cell.CreatedAt,
		UpdatedAt:     cell.UpdatedAt,
		Accounts:      []EgressCellAccountRef{},
	})
}
