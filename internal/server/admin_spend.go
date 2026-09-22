package server

import (
	"net/http"
	"sort"
)

// spendCosts carries one row's money across the four fixed rolling windows.
type spendCosts struct {
	D1  float64 `json:"d1"`
	D3  float64 `json:"d3"`
	D7  float64 `json:"d7"`
	D30 float64 `json:"d30"`
}

func (c *spendCosts) add(other spendCosts) {
	c.D1 += other.D1
	c.D3 += other.D3
	c.D7 += other.D7
	c.D30 += other.D30
}

type spendModelRow struct {
	Model string     `json:"model"`
	Cost  spendCosts `json:"cost"`
}

type spendUserGroup struct {
	UserID   string          `json:"user_id"`
	UserName string          `json:"user_name"`
	Models   []spendModelRow `json:"models"`
	Subtotal spendCosts      `json:"subtotal"`
}

type spendResponse struct {
	Users []spendUserGroup `json:"users"`
	Total spendCosts       `json:"total"`
}

// handleSpend answers "which user burned how much on which model" across the
// 1d/3d/7d/30d rolling windows.
//
// Rows whose user_id no longer resolves to a live user are dropped. That
// single rule covers both admin-token traffic (auth assigns the synthetic id
// "admin", which is not a users row) and deleted users, so the totals are
// explicitly "across the users shown here", not "everything that ever ran".
func (s *Server) handleSpend(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.store.QueryUserModelCosts(ctx)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to query spend")
		return
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	names := make(map[string]string, len(users))
	for _, user := range users {
		names[user.ID] = user.Name
	}

	groups := make(map[string]*spendUserGroup)
	resp := spendResponse{Users: []spendUserGroup{}}
	for _, row := range rows {
		name, known := names[row.UserID]
		if !known {
			continue
		}
		group, ok := groups[row.UserID]
		if !ok {
			group = &spendUserGroup{UserID: row.UserID, UserName: name, Models: []spendModelRow{}}
			groups[row.UserID] = group
		}
		cost := spendCosts{D1: row.Cost1d, D3: row.Cost3d, D7: row.Cost7d, D30: row.Cost30d}
		group.Models = append(group.Models, spendModelRow{Model: row.Model, Cost: cost})
		group.Subtotal.add(cost)
		resp.Total.add(cost)
	}

	for _, group := range groups {
		sort.Slice(group.Models, func(i, j int) bool {
			if group.Models[i].Cost.D30 != group.Models[j].Cost.D30 {
				return group.Models[i].Cost.D30 > group.Models[j].Cost.D30
			}
			return group.Models[i].Model < group.Models[j].Model
		})
		resp.Users = append(resp.Users, *group)
	}
	sort.Slice(resp.Users, func(i, j int) bool {
		if resp.Users[i].Subtotal.D30 != resp.Users[j].Subtotal.D30 {
			return resp.Users[i].Subtotal.D30 > resp.Users[j].Subtotal.D30
		}
		return resp.Users[i].UserName < resp.Users[j].UserName
	})

	writeJSON(w, http.StatusOK, resp)
}
