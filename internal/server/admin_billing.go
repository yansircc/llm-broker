package server

import "net/http"

func (s *Server) handleAdminBillingSummary(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	orders, err := s.store.SummarizePaymentOrders(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to summarize orders")
		return
	}
	active := 0
	for _, user := range users {
		if user.Status == "active" {
			active++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":               len(users),
		"active_users":        active,
		"open_orders":         orders.PendingOrders,
		"paid_orders":         orders.PaidOrders,
		"revenue_usd":         microsToUSD(orders.PaidCreditMicros),
		"revenue_cny":         float64(orders.PaidAmountCNYFen) / 100,
		"credits_usd":         microsToUSD(orders.PaidCreditMicros),
		"pending_credits_usd": microsToUSD(orders.PendingCreditMicros),
	})
}

func (s *Server) handleAdminBillingOrders(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	userEmails := make(map[string]string, len(users))
	for _, user := range users {
		userEmails[user.ID] = user.Email
	}
	limit, _ := limitOffset(r, 200)
	orders, err := s.store.ListPaymentOrders(r.Context(), limit)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list orders")
		return
	}
	var out []map[string]any
	for _, order := range orders {
		view := paymentOrderView(order, "")
		view["user_id"] = order.UserID
		view["user_email"] = userEmails[order.UserID]
		view["provider"] = order.Gateway
		out = append(out, view)
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminRefreshPaymentOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("id")
	order, err := s.store.GetPaymentOrderByOutTradeNo(r.Context(), orderID)
	if err != nil || order == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	refreshed, err := s.refreshPaymentOrder(r.Context(), order)
	if err != nil {
		writeAdminError(w, http.StatusBadGateway, "payment_query_failed", err.Error())
		return
	}
	view := paymentOrderView(refreshed, "")
	if user, err := s.store.GetUser(r.Context(), refreshed.UserID); err == nil && user != nil {
		view["user_id"] = user.ID
		view["user_email"] = user.Email
	}
	view["provider"] = refreshed.Gateway
	writeJSON(w, http.StatusOK, view)
}
