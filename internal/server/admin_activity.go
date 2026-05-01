package server

import (
	"context"
	"net/http"
	"sort"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	writeJSON(w, http.StatusOK, ActivityResponse{
		Health:         s.buildHealthInfo(ctx),
		Accounts:       s.activityAccounts(),
		Users:          s.activityUsers(ctx),
		Events:         s.dashboardEvents(20),
		RecentFailures: s.activityRecentFailures(ctx, 30),
	})
}

func (s *Server) handleActivityUsage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.dashboardUsage(r.Context(), parseTZParam(r)))
}

func (s *Server) activityAccounts() []ActivityAccountRef {
	accounts := s.pool.List()
	views := make([]ActivityAccountRef, 0, len(accounts))
	for _, acct := range accounts {
		views = append(views, ActivityAccountRef{
			ID:    acct.ID,
			Email: acct.Email,
		})
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Email != views[j].Email {
			return views[i].Email < views[j].Email
		}
		return views[i].ID < views[j].ID
	})
	return views
}

func (s *Server) activityUsers(ctx context.Context) []ActivityUserRef {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return []ActivityUserRef{}
	}
	views := make([]ActivityUserRef, 0, len(users))
	for _, user := range users {
		views = append(views, ActivityUserRef{
			ID:   user.ID,
			Name: user.Name,
		})
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Name != views[j].Name {
			return views[i].Name < views[j].Name
		}
		return views[i].ID < views[j].ID
	})
	return views
}

func (s *Server) activityRecentFailures(ctx context.Context, limit int) []*domain.RequestLog {
	logs, _, err := s.store.QueryRequestLogs(ctx, domain.RequestLogQuery{
		FailuresOnly: true,
		Limit:        limit,
	})
	if err != nil || logs == nil {
		return []*domain.RequestLog{}
	}
	return logs
}
