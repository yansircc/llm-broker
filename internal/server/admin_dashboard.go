package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	writeJSON(w, http.StatusOK, DashboardResponse{
		Health:         s.buildHealthInfo(ctx),
		Usage:          s.dashboardUsage(ctx, parseTZParam(r)),
		Accounts:       s.dashboardAccounts(),
		Users:          s.dashboardUsers(ctx),
		Events:         s.dashboardEvents(20),
		OutcomeStats:   s.dashboardOutcomeStats(ctx, 24*time.Hour),
		CellRisk:       s.dashboardCellRisk(ctx, 7*24*time.Hour),
		RecentFailures: s.dashboardRecentFailures(ctx, 30),
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
	availability := s.pool.SurfaceAvailabilityMap()
	views := make([]DashboardAccount, 0, len(accounts))
	for _, acct := range accounts {
		proj := s.projectAccount(acct)
		avail := availability[acct.ID]
		views = append(views, DashboardAccount{
			ID:              acct.ID,
			Email:           acct.Email,
			Provider:        string(acct.Provider),
			Status:          string(acct.Status),
			WeightMode:      acct.PriorityMode,
			Weight:          proj.effectiveWeight,
			CooldownUntil:   acct.CooldownUntil,
			LastUsedAt:      acct.LastUsedAt,
			CellID:          acct.CellID,
			AvailableNative: avail.Native,
			AvailableCompat: avail.Compat,
			Windows:         proj.windows,
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
			Type:                 string(event.Type),
			AccountID:            event.AccountID,
			UserID:               event.UserID,
			BucketKey:            event.BucketKey,
			CellID:               event.CellID,
			CooldownUntil:        event.CooldownUntil,
			UpstreamStatus:       event.UpstreamStatus,
			UpstreamErrorType:    event.UpstreamErrorType,
			UpstreamErrorMessage: event.UpstreamErrorMessage,
			Message:              event.Message,
			Timestamp:            event.Timestamp.Format(time.RFC3339),
		})
	}
	return views
}

func (s *Server) dashboardOutcomeStats(ctx context.Context, window time.Duration) []RelayOutcomeStatResponse {
	stats, err := s.store.QueryRelayOutcomeStats(ctx, time.Now().UTC().Add(-window))
	if err != nil {
		slog.Warn("dashboard: query relay outcome stats failed", "error", err)
		return []RelayOutcomeStatResponse{}
	}
	views := make([]RelayOutcomeStatResponse, 0, len(stats))
	for _, stat := range stats {
		views = append(views, RelayOutcomeStatResponse{
			Provider:         stat.Provider,
			Surface:          stat.Surface,
			EffectKind:       stat.EffectKind,
			UpstreamStatus:   stat.UpstreamStatus,
			Requests:         stat.Requests,
			DistinctUsers:    stat.DistinctUsers,
			DistinctAccounts: stat.DistinctAccounts,
			LastSeenAt:       stat.LastSeenAt,
		})
	}
	return views
}

func (s *Server) dashboardRecentFailures(ctx context.Context, limit int) []*domain.RequestLog {
	logs, _, err := s.store.QueryRequestLogs(ctx, domain.RequestLogQuery{
		FailuresOnly: true,
		Limit:        limit,
	})
	if err != nil {
		slog.Warn("dashboard: query recent failures failed", "error", err)
		return []*domain.RequestLog{}
	}
	if logs == nil {
		return []*domain.RequestLog{}
	}
	// Strip heavy fields — dashboard only needs summary columns.
	for _, l := range logs {
		l.ClientBodyExcerpt = ""
		l.ClientHeaders = nil
		l.RequestMeta = nil
		l.UpstreamRequestHeaders = nil
		l.UpstreamRequestMeta = nil
		l.UpstreamRequestBodyExcerpt = ""
		l.UpstreamHeaders = nil
		l.UpstreamResponseMeta = nil
		l.UpstreamResponseBodyExcerpt = ""
	}
	return logs
}

func (s *Server) dashboardCellRisk(ctx context.Context, window time.Duration) []CellRiskResponse {
	stats, err := s.store.QueryCellRiskStats(ctx, time.Now().UTC().Add(-window))
	if err != nil {
		slog.Warn("dashboard: query cell risk stats failed", "error", err)
		return []CellRiskResponse{}
	}

	cells := s.pool.ListCells()
	cellMap := make(map[string]*domain.EgressCell, len(cells))
	for _, cell := range cells {
		cellMap[cell.ID] = cell
	}

	views := make([]CellRiskResponse, 0, len(stats))
	for _, stat := range stats {
		cell := cellMap[stat.CellID]
		views = append(views, CellRiskResponse{
			CellID:           stat.CellID,
			CellName:         dashboardCellName(cell, stat.CellID),
			Provider:         stat.Provider,
			Region:           dashboardCellRegion(cell),
			Transport:        dashboardCellTransport(cell),
			Requests:         stat.Requests,
			Successes:        stat.Successes,
			Status400:        stat.Status400,
			Status403:        stat.Status403,
			Status429:        stat.Status429,
			Blocks:           stat.Blocks,
			TransportErrors:  stat.TransportErrors,
			DistinctUsers:    stat.DistinctUsers,
			DistinctAccounts: stat.DistinctAccounts,
			LastSeenAt:       stat.LastSeenAt,
		})
	}

	sort.Slice(views, func(i, j int) bool {
		leftRisk := views[i].Status400 + views[i].Status403 + views[i].Status429 + views[i].Blocks + views[i].TransportErrors
		rightRisk := views[j].Status400 + views[j].Status403 + views[j].Status429 + views[j].Blocks + views[j].TransportErrors
		if leftRisk != rightRisk {
			return leftRisk > rightRisk
		}
		if views[i].Requests != views[j].Requests {
			return views[i].Requests > views[j].Requests
		}
		if views[i].Provider != views[j].Provider {
			return views[i].Provider < views[j].Provider
		}
		return views[i].CellName < views[j].CellName
	})

	return views
}

func dashboardCellName(cell *domain.EgressCell, cellID string) string {
	if cell != nil {
		if cell.Name != "" {
			return cell.Name
		}
		if cell.ID != "" {
			return cell.ID
		}
	}
	if cellID == "" {
		return "legacy direct"
	}
	return cellID
}

func dashboardCellRegion(cell *domain.EgressCell) string {
	if cell == nil || len(cell.Labels) == 0 {
		return "-"
	}
	if region := stringsJoinNonEmpty(cell.Labels["country"], cell.Labels["city"]); region != "" {
		return region
	}
	if site := cell.Labels["site"]; site != "" {
		return site
	}
	return "-"
}

func dashboardCellTransport(cell *domain.EgressCell) string {
	if cell == nil || len(cell.Labels) == 0 {
		return "legacy-direct"
	}
	if transport := cell.Labels["transport"]; transport != "" {
		return transport
	}
	return "unknown"
}

func stringsJoinNonEmpty(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return joinWithSlash(filtered)
}

func joinWithSlash(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += " / " + part
	}
	return out
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
