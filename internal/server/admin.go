package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/yansir/cc-relayer/internal/auth"
	"github.com/yansir/cc-relayer/internal/domain"
)

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	ki := auth.GetKeyInfo(r.Context())
	if ki == nil || !ki.IsAdmin {
		writeAdminError(w, http.StatusForbidden, "forbidden", "admin access required")
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}

	ki, valid := s.authMw.ValidateToken(r.Context(), req.Token)
	if !valid || ki == nil {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "invalid token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "cc_session",
		Value:    req.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	})
	writeJSON(w, http.StatusOK, struct {
		Status  string `json:"status"`
		IsAdmin bool   `json:"is_admin"`
		Name    string `json:"name"`
	}{"ok", ki.IsAdmin, ki.Name})
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	ctx := r.Context()

	// Health
	sqliteStatus := "ok"
	if err := s.store.Ping(ctx); err != nil {
		sqliteStatus = err.Error()
	}
	d := time.Since(s.startTime)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	uptime := fmt.Sprintf("%dd %dh %dm", days, hours, mins)

	// Usage periods
	loc := parseTZParam(r)
	usage, err := s.store.QueryUsagePeriods(ctx, "", loc)
	if err != nil {
		slog.Warn("dashboard: query usage periods failed", "error", err)
	}

	// Accounts
	acctList := s.pool.List()

	acctViews := make([]DashboardAccount, 0, len(acctList))
	for _, a := range acctList {
		pri := a.Priority
		if a.PriorityMode == "auto" {
			if drv, ok := s.drivers[a.Provider]; ok {
				pri = drv.AutoPriority(json.RawMessage(a.ProviderStateJSON))
			}
		}
		av := DashboardAccount{
			ID:              a.ID,
			Email:           a.Email,
			Provider:        string(a.Provider),
			Status:          string(a.Status),
			PriorityMode:    a.PriorityMode,
			Priority:        pri,
			OverloadedUntil: a.OverloadedUntil,
			LastUsedAt:      a.LastUsedAt,
		}
		if drv, ok := s.drivers[a.Provider]; ok {
			primary, secondary := drv.GetUtilization(json.RawMessage(a.ProviderStateJSON))
			if primary != nil {
				av.FiveHourUtil = &primary.Pct
				if primary.Reset > 0 {
					av.FiveHourReset = &primary.Reset
				}
			}
			if secondary != nil {
				av.SevenDayUtil = &secondary.Pct
				if secondary.Reset > 0 {
					av.SevenDayReset = &secondary.Reset
				}
			}
		}
		acctViews = append(acctViews, av)
	}

	// Users with total cost
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		slog.Warn("dashboard: list users failed", "error", err)
	}
	userCosts, err := s.store.QueryUserTotalCosts(ctx)
	if err != nil {
		slog.Warn("dashboard: query user costs failed", "error", err)
	}

	userViews := make([]DashboardUser, 0, len(users))
	for _, u := range users {
		userViews = append(userViews, DashboardUser{
			ID:           u.ID,
			Name:         u.Name,
			Status:       u.Status,
			LastActiveAt: u.LastActiveAt,
			TotalCost:    userCosts[u.ID],
		})
	}

	// Events
	recentEvents := s.bus.Recent(20)
	evViews := make([]DashboardEvent, 0, len(recentEvents))
	for i := len(recentEvents) - 1; i >= 0; i-- {
		e := recentEvents[i]
		evViews = append(evViews, DashboardEvent{
			Type:      string(e.Type),
			AccountID: e.AccountID,
			Message:   e.Message,
			Timestamp: e.Timestamp.Format(time.RFC3339),
		})
	}

	if usage == nil {
		usage = []domain.UsagePeriod{}
	}
	writeJSON(w, http.StatusOK, DashboardResponse{
		Health: HealthInfo{
			SQLite:  sqliteStatus,
			Uptime:  uptime,
			Version: s.version,
		},
		Usage:    usage,
		Accounts: acctViews,
		Users:    userViews,
		Events:   evViews,
	})
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	sqliteStatus := "ok"
	if err := s.store.Ping(r.Context()); err != nil {
		sqliteStatus = err.Error()
	}
	d := time.Since(s.startTime)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	uptime := fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	writeJSON(w, http.StatusOK, HealthInfo{
		SQLite:  sqliteStatus,
		Uptime:  uptime,
		Version: s.version,
	})
}

// parseTZParam extracts the "tz" query parameter (IANA timezone name)
// and returns the corresponding *time.Location. Falls back to UTC.
func parseTZParam(r *http.Request) *time.Location {
	tz := r.URL.Query().Get("tz")
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode failed", "error", err)
	}
}

func writeAdminError(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":"%s","message":"%s"}}`, errType, msg)
}

func (s *Server) handleClearEvents(w http.ResponseWriter, r *http.Request) {
	s.bus.Clear()
	w.WriteHeader(http.StatusNoContent)
}
