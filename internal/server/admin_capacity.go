package server

import (
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

type CapacityResponse struct {
	Summary        CapacitySummary   `json:"summary"`
	Accounts       []CapacityAccount `json:"accounts"`
	ActiveRequests []map[string]any  `json:"active_requests"`
	Connections    map[string]int    `json:"connections"`
}

type CapacitySummary struct {
	Accounts        int `json:"accounts"`
	ActiveAccounts  int `json:"active_accounts"`
	AvailableNative int `json:"available_native"`
	AvailableCompat int `json:"available_compat"`
	CoolingAccounts int `json:"cooling_accounts"`
	Requests1h      int `json:"requests_1h"`
	Failures1h      int `json:"failures_1h"`
	ActiveRequests  int `json:"active_requests"`
}

type CapacityAccount struct {
	ID              string                      `json:"id"`
	Email           string                      `json:"email"`
	Provider        string                      `json:"provider"`
	Status          string                      `json:"status"`
	Weight          int                         `json:"weight"`
	AvailableNative bool                        `json:"available_native"`
	AvailableCompat bool                        `json:"available_compat"`
	CooldownUntil   *time.Time                  `json:"cooldown_until,omitempty"`
	Requests1h      int                         `json:"requests_1h"`
	Failures1h      int                         `json:"failures_1h"`
	Windows         []UtilizationWindowResponse `json:"windows"`
}

func (s *Server) handleAdminCapacity(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	since := now.Add(-time.Hour)
	accounts := s.pool.List()
	availability := s.pool.SurfaceAvailabilityMap()
	activeRequests := s.snapshotActiveRequests()

	_, requests1h, err := s.store.QueryRequestLogs(r.Context(), domain.RequestLogQuery{Since: &since, Limit: 1})
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load request count")
		return
	}
	_, failures1h, err := s.store.QueryRequestLogs(r.Context(), domain.RequestLogQuery{Since: &since, FailuresOnly: true, Limit: 1})
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load failure count")
		return
	}

	resp := CapacityResponse{
		Summary: CapacitySummary{
			Accounts:       len(accounts),
			Requests1h:     requests1h,
			Failures1h:     failures1h,
			ActiveRequests: len(activeRequests),
		},
		Accounts:       make([]CapacityAccount, 0, len(accounts)),
		ActiveRequests: activeRequests,
		Connections:    s.snapshotConnStates(),
	}
	for _, acct := range accounts {
		avail := availability[acct.ID]
		if acct.Status == domain.StatusActive {
			resp.Summary.ActiveAccounts++
		}
		if avail.Native {
			resp.Summary.AvailableNative++
		}
		if avail.Compat {
			resp.Summary.AvailableCompat++
		}
		if acct.CooldownUntil != nil && acct.CooldownUntil.After(now) {
			resp.Summary.CoolingAccounts++
		}
		_, accountRequests, err := s.store.QueryRequestLogs(r.Context(), domain.RequestLogQuery{
			AccountID: acct.ID,
			Since:     &since,
			Limit:     1,
		})
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load account request count")
			return
		}
		_, accountFailures, err := s.store.QueryRequestLogs(r.Context(), domain.RequestLogQuery{
			AccountID:    acct.ID,
			Since:        &since,
			FailuresOnly: true,
			Limit:        1,
		})
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load account failure count")
			return
		}
		proj := s.projectAccount(acct)
		resp.Accounts = append(resp.Accounts, CapacityAccount{
			ID:              acct.ID,
			Email:           acct.Email,
			Provider:        string(acct.Provider),
			Status:          string(acct.Status),
			Weight:          proj.effectiveWeight,
			AvailableNative: avail.Native,
			AvailableCompat: avail.Compat,
			CooldownUntil:   acct.CooldownUntil,
			Requests1h:      accountRequests,
			Failures1h:      accountFailures,
			Windows:         proj.windows,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
