package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
	"golang.org/x/net/proxy"
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
		ID         string              `json:"id"`
		Name       string              `json:"name"`
		Status     string              `json:"status"`
		Proxy      *domain.ProxyConfig `json:"proxy"`
		Labels     map[string]string   `json:"labels"`
		CreateOnly bool                `json:"create_only"`
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
		if req.CreateOnly {
			writeAdminError(w, http.StatusConflict, "conflict", "cell already exists")
			return
		}
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

func (s *Server) handleTestProxy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Proxy *domain.ProxyConfig `json:"proxy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if req.Proxy == nil || req.Proxy.Host == "" || req.Proxy.Port <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "proxy host and port are required")
		return
	}
	result := dialTestProxy(req.Proxy)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleTestEgressCell(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "cell id is required")
		return
	}
	cell := s.pool.GetCell(id)
	if cell == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "cell not found")
		return
	}
	if cell.Proxy == nil || cell.Proxy.Host == "" || cell.Proxy.Port <= 0 {
		writeJSON(w, http.StatusOK, TestAccountResult{OK: false, Error: "cell has no usable proxy"})
		return
	}
	result := dialTestProxy(cell.Proxy)
	writeJSON(w, http.StatusOK, result)
}

func dialTestProxy(pcfg *domain.ProxyConfig) TestAccountResult {
	start := time.Now()
	proxyAddr := fmt.Sprintf("%s:%d", pcfg.Host, pcfg.Port)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch pcfg.Type {
	case "socks5":
		return dialTestSocks5(ctx, proxyAddr, pcfg, start)
	default:
		// HTTP CONNECT: plain TCP dial to proxy, then CONNECT handshake
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
		if err != nil {
			return TestAccountResult{OK: false, Error: fmt.Sprintf("tcp dial: %v", err)}
		}
		conn.Close()
		return TestAccountResult{OK: true, LatencyMs: time.Since(start).Milliseconds()}
	}
}

func dialTestSocks5(ctx context.Context, proxyAddr string, pcfg *domain.ProxyConfig, start time.Time) TestAccountResult {
	const testTarget = "www.gstatic.com:443"

	var auth *proxy.Auth
	if pcfg.Username != "" {
		auth = &proxy.Auth{User: pcfg.Username, Password: pcfg.Password}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return TestAccountResult{OK: false, Error: fmt.Sprintf("socks5 dialer: %v", err)}
	}

	if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
		conn, err := ctxDialer.DialContext(ctx, "tcp", testTarget)
		if err != nil {
			return TestAccountResult{OK: false, Error: fmt.Sprintf("dial: %v", err)}
		}
		conn.Close()
	} else {
		conn, err := dialer.Dial("tcp", testTarget)
		if err != nil {
			return TestAccountResult{OK: false, Error: fmt.Sprintf("dial: %v", err)}
		}
		conn.Close()
	}
	return TestAccountResult{OK: true, LatencyMs: time.Since(start).Milliseconds()}
}