package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	writeJSON(w, http.StatusOK, DashboardResponse{
		Health:   s.buildHealthInfo(ctx),
		Usage:    s.dashboardUsage(ctx, parseTZParam(r)),
		Accounts: s.dashboardAccounts(),
		Users:    s.dashboardUsers(ctx),
		Events:   s.dashboardEvents(20),
	})
}

func (s *Server) dashboardUsage(ctx context.Context, loc *time.Location) []domain.UsagePeriod {
	usage, err := s.store.QueryUsagePeriods(ctx, "", loc)
	if err != nil {
		slog.Warn("dashboard: query usage periods failed", "error", err)
	}
	if usage == nil {
		return []domain.UsagePeriod{}
	}
	return usage
}

func (s *Server) dashboardAccounts() []DashboardAccount {
	accounts := s.pool.List()
	views := make([]DashboardAccount, 0, len(accounts))
	for _, acct := range accounts {
		proj := s.projectAccount(acct)
		views = append(views, DashboardAccount{
			ID:            acct.ID,
			Email:         acct.Email,
			Provider:      string(acct.Provider),
			Status:        string(acct.Status),
			PriorityMode:  acct.PriorityMode,
			Priority:      proj.effectivePriority,
			CooldownUntil: acct.CooldownUntil,
			LastUsedAt:    acct.LastUsedAt,
			CellID:        acct.CellID,
			Windows:       proj.windows,
		})
	}
	return views
}

func (s *Server) dashboardUsers(ctx context.Context) []DashboardUser {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		slog.Warn("dashboard: list users failed", "error", err)
	}
	userCosts, err := s.store.QueryUserTotalCosts(ctx)
	if err != nil {
		slog.Warn("dashboard: query user costs failed", "error", err)
	}

	views := make([]DashboardUser, 0, len(users))
	for _, user := range users {
		views = append(views, DashboardUser{
			ID:                user.ID,
			Name:              user.Name,
			Status:            user.Status,
			AllowedSurface:    user.AllowedSurface,
			BoundAccountID:    user.BoundAccountID,
			BoundAccountEmail: s.boundAccountEmail(user.BoundAccountID),
			LastActiveAt:      user.LastActiveAt,
			TotalCost:         userCosts[user.ID],
		})
	}
	return views
}

func (s *Server) dashboardEvents(limit int) []DashboardEvent {
	recentEvents := s.bus.Recent(limit)
	views := make([]DashboardEvent, 0, len(recentEvents))
	for i := len(recentEvents) - 1; i >= 0; i-- {
		event := recentEvents[i]
		views = append(views, DashboardEvent{
			Type:      string(event.Type),
			AccountID: event.AccountID,
			Message:   event.Message,
			Timestamp: event.Timestamp.Format(time.RFC3339),
		})
	}
	return views
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.buildHealthInfo(r.Context()))
}

func (s *Server) buildHealthInfo(ctx context.Context) HealthInfo {
	sqliteStatus := "ok"
	if err := s.store.Ping(ctx); err != nil {
		sqliteStatus = err.Error()
	}
	return HealthInfo{
		SQLite:  sqliteStatus,
		Uptime:  formatUptime(time.Since(s.startTime)),
		Version: s.version,
	}
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
}

func (s *Server) handleClearEvents(w http.ResponseWriter, _ *http.Request) {
	s.bus.Clear()
	w.WriteHeader(http.StatusNoContent)
}
