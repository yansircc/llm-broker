package server

import "net/http"

func (s *Server) handleAdminBillingSummary(w http.ResponseWriter, r *http.Request) {
	users, _ := s.store.ListUsers(r.Context())
	active := 0
	for _, user := range users {
		if user.Status == "active" {
			active++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":        len(users),
		"active_users": active,
		"open_orders":  0,
		"revenue_usd":  0,
		"credits_usd":  0,
	})
}

func (s *Server) handleAdminBillingOrders(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	var out []map[string]any
	for _, user := range users {
		orders, _ := s.store.ListPaymentOrdersByUser(r.Context(), user.ID, 20)
		for _, order := range orders {
			view := paymentOrderView(order, "")
			view["user_id"] = user.ID
			view["user_email"] = user.Email
			view["provider"] = order.Gateway
			out = append(out, view)
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, out)
}
