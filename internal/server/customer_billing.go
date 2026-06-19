package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/payments/zpay"
)

const paymentRefreshMinInterval = 5 * time.Second

func (s *Server) handleCustomerBillingSummary(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	balance, _, err := s.billingService().Balance(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load balance")
		return
	}
	summary, err := s.store.SummarizeBillingLedgerByUser(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load billing summary")
		return
	}
	lowBalanceThreshold := s.lowBalanceAlertThresholdMicros(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":                      "prepaid",
		"status":                    cc.User.Status,
		"balance_usd":               microsToUSD(balance),
		"credits_usd":               microsToUSD(summary.CreditMicros),
		"usage_usd":                 microsToUSD(summary.UsageMicros),
		"low_balance":               lowBalanceThreshold > 0 && balance <= lowBalanceThreshold,
		"low_balance_threshold_usd": microsToUSD(lowBalanceThreshold),
	})
}

func (s *Server) handleCustomerBillingLedger(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r, 50)
	entries, total, err := s.store.ListBillingLedgerByUser(r.Context(), cc.User.ID, limit, offset)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load billing ledger")
		return
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, billingLedgerEntryView(entry))
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "total": total})
}

func (s *Server) handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	var req struct {
		AmountUSD float64 `json:"amount_usd"`
		Type      string  `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AmountUSD <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "amount_usd required")
		return
	}
	client := s.zpayClient
	if client == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "payment_unavailable", "zpay is not configured")
		return
	}
	if req.Type == "" {
		req.Type = "alipay"
	}
	now := time.Now().UTC()
	creditMicros := int64(math.Round(req.AmountUSD * 1_000_000))
	rate := s.cnyToUSDRateMicros(r)
	amountFen := (creditMicros*100 + rate - 1) / rate
	outTradeNo := fmt.Sprintf("%s_%d", compactID(cc.User.ID), now.UnixMilli())
	order := &domain.PaymentOrder{
		ID:                 uuid.NewString(),
		OutTradeNo:         outTradeNo,
		UserID:             cc.User.ID,
		Gateway:            "zpay",
		Status:             "pending",
		ProductName:        "LLM relay credit",
		AmountCNYFen:       amountFen,
		CreditMicros:       creditMicros,
		ExchangeRateMicros: rate,
		PaymentType:        req.Type,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.store.SavePaymentOrder(r.Context(), order); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create order")
		return
	}
	resp, err := client.CreateQRCodeOrder(r.Context(), zpay.CreateQRCodeOrderRequest{
		Type:       req.Type,
		Name:       order.ProductName,
		Money:      fenToYuan(amountFen),
		OutTradeNo: outTradeNo,
		NotifyURL:  s.publicURL(r, "/api/payments/notify"),
		ClientIP:   s.clientIP(r),
		Device:     "pc",
		Param:      cc.User.ID,
		CID:        s.zpayCID(),
	})
	if err != nil || resp.Code != 1 {
		order.Status = "failed"
		order.UpdatedAt = time.Now().UTC()
		_ = s.store.SavePaymentOrder(r.Context(), order)
		msg := "failed to create zpay order"
		if resp.Msg != "" {
			msg = resp.Msg
		}
		writeAdminError(w, http.StatusBadGateway, "payment_error", msg)
		return
	}
	order.ZpayTradeNo = resp.TradeNo
	order.QRCode = resp.QRCode
	order.QRImage = resp.Image
	order.UpdatedAt = time.Now().UTC()
	_ = s.store.SavePaymentOrder(r.Context(), order)
	writeJSON(w, http.StatusOK, paymentOrderView(order, resp.PayURL))
}

func (s *Server) handlePaymentNotify(w http.ResponseWriter, r *http.Request) {
	params, err := paymentParams(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
		return
	}
	valid := s.cfg != nil && s.cfg.ZPayKey != "" && zpay.Verify(params, s.cfg.ZPayKey)
	payload, _ := json.Marshal(params)
	_ = s.store.SavePaymentEvent(r.Context(), &domain.PaymentEvent{
		ID:             uuid.NewString(),
		OrderID:        params["out_trade_no"],
		Gateway:        "zpay",
		EventType:      "notify",
		ValidSignature: valid,
		PayloadJSON:    string(payload),
		CreatedAt:      time.Now().UTC(),
	})
	if !valid {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("fail"))
		return
	}
	if params["trade_status"] != "TRADE_SUCCESS" {
		w.Write([]byte("success"))
		return
	}
	if err := s.fulfillPaidOrder(r.Context(), params["out_trade_no"], params["trade_no"], params["type"], params["money"]); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
		return
	}
	w.Write([]byte("success"))
}

func (s *Server) handleCustomerPaymentOrder(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	orderID := r.PathValue("id")
	order, err := s.store.GetPaymentOrderByOutTradeNo(r.Context(), orderID)
	if err != nil || order == nil || order.UserID != cc.User.ID {
		writeAdminError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	writeJSON(w, http.StatusOK, paymentOrderView(order, ""))
}

func (s *Server) handleCustomerRefreshPaymentOrder(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	orderID := r.PathValue("id")
	order, err := s.store.GetPaymentOrderByOutTradeNo(r.Context(), orderID)
	if err != nil || order == nil || order.UserID != cc.User.ID {
		writeAdminError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	refreshed, err := s.refreshPaymentOrder(r.Context(), order)
	if err != nil {
		writeAdminError(w, http.StatusBadGateway, "payment_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, paymentOrderView(refreshed, ""))
}

func (s *Server) handleCustomerPaymentOrders(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	limit, _ := limitOffset(r, 50)
	orders, err := s.store.ListPaymentOrdersByUser(r.Context(), cc.User.ID, limit)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load payment orders")
		return
	}
	out := make([]map[string]any, 0, len(orders))
	for _, order := range orders {
		out = append(out, paymentOrderView(order, ""))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCustomerReferrals(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	stats, err := s.store.ReferralStatsByInviter(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load referrals")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":          cc.User.ReferralCode,
		"url":           s.publicURL(r, "/app/register?ref="+cc.User.ReferralCode),
		"signups":       stats.Signups,
		"paid_invitees": stats.PaidInvitees,
		"credits_usd":   microsToUSD(stats.CreditMicros),
	})
}

func (s *Server) handleCustomerUsage(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	limit, offset := limitOffset(r, 50)
	since, until := usageRange(r)
	logs, total, err := s.store.QueryRequestLogs(r.Context(), domain.RequestLogQuery{
		UserID:   cc.User.ID,
		APIKeyID: strings.TrimSpace(r.URL.Query().Get("key_id")),
		Model:    strings.TrimSpace(r.URL.Query().Get("model")),
		Since:    since,
		Until:    until,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load usage")
		return
	}
	periods, err := s.store.QueryUsagePeriods(r.Context(), cc.User.ID, time.Local)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load usage periods")
		return
	}
	modelUsage, err := s.store.QueryModelUsage(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load model usage")
		return
	}
	keys, err := s.store.ListAPIKeysByUser(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load api keys")
		return
	}
	keyNames := make(map[string]string, len(keys))
	for _, key := range keys {
		keyNames[key.ID] = key.Name
	}
	out := make([]map[string]any, 0, len(logs))
	for _, log := range logs {
		view := requestLogView(log)
		view["api_key_name"] = keyNames[log.APIKeyID]
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"logs":        out,
		"total":       total,
		"periods":     periods,
		"model_usage": modelUsage,
	})
}

func (s *Server) refreshPaymentOrder(ctx context.Context, order *domain.PaymentOrder) (*domain.PaymentOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("order not found")
	}
	if order.Status == "paid" {
		return order, nil
	}
	now := time.Now().UTC()
	if order.UpdatedAt.After(now.Add(-paymentRefreshMinInterval)) {
		return order, nil
	}
	if s.zpayClient == nil {
		return nil, fmt.Errorf("zpay is not configured")
	}
	resp, err := s.zpayClient.QueryOrder(ctx, zpay.QueryOrderRequest{OutTradeNo: order.OutTradeNo})
	if err != nil {
		order.UpdatedAt = now
		_ = s.store.SavePaymentOrder(ctx, order)
		return nil, err
	}
	payload, _ := json.Marshal(resp)
	_ = s.store.SavePaymentEvent(ctx, &domain.PaymentEvent{
		ID:             uuid.NewString(),
		OrderID:        order.OutTradeNo,
		Gateway:        order.Gateway,
		EventType:      "query",
		ValidSignature: false,
		PayloadJSON:    string(payload),
		CreatedAt:      now,
	})
	if resp.Code == 1 && resp.Status == 1 {
		if err := s.fulfillPaidOrder(ctx, order.OutTradeNo, resp.TradeNo, resp.Type, resp.Money); err != nil {
			return nil, err
		}
		updated, err := s.store.GetPaymentOrderByOutTradeNo(ctx, order.OutTradeNo)
		if err != nil {
			return nil, err
		}
		if updated != nil {
			return updated, nil
		}
	}
	order.UpdatedAt = now
	_ = s.store.SavePaymentOrder(ctx, order)
	return order, nil
}

func (s *Server) fulfillPaidOrder(ctx context.Context, outTradeNo, zpayTradeNo, paymentType, money string) error {
	order, err := s.store.GetPaymentOrderByOutTradeNo(ctx, outTradeNo)
	if err != nil || order == nil {
		return fmt.Errorf("order not found")
	}
	paidFen, err := parseYuanToFen(money)
	if err != nil || paidFen != order.AmountCNYFen {
		return fmt.Errorf("amount mismatch")
	}
	if _, err := s.billingService().FulfillPaymentOrder(ctx, order, zpayTradeNo, paymentType, time.Now().UTC()); err != nil {
		return err
	}
	_, err = s.billingService().FulfillReferralInviterAfterPayment(ctx, order.UserID)
	return err
}

func paymentParams(r *http.Request) (map[string]string, error) {
	values := r.URL.Query()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		values = r.Form
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	if out["sign"] == "" || out["out_trade_no"] == "" {
		return nil, fmt.Errorf("missing required params")
	}
	return out, nil
}

func (s *Server) cnyToUSDRateMicros(r *http.Request) int64 {
	raw, err := s.store.GetBillingSetting(r.Context(), "cny_to_usd_rate_micros")
	if err != nil || raw == "" {
		return 1_000_000
	}
	rate, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || rate <= 0 {
		return 1_000_000
	}
	return rate
}

func (s *Server) lowBalanceAlertThresholdMicros(ctx context.Context) int64 {
	raw, err := s.store.GetBillingSetting(ctx, "low_balance_alert_threshold_micros")
	if err != nil || raw == "" {
		return 5_000_000
	}
	threshold, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || threshold < 0 {
		return 5_000_000
	}
	return threshold
}

func paymentOrderView(order *domain.PaymentOrder, checkoutURL string) map[string]any {
	return map[string]any{
		"id":           order.OutTradeNo,
		"out_trade_no": order.OutTradeNo,
		"status":       order.Status,
		"amount_usd":   microsToUSD(order.CreditMicros),
		"amount_cny":   float64(order.AmountCNYFen) / 100,
		"checkout_url": checkoutURL,
		"qrcode":       order.QRCode,
		"qr_image":     order.QRImage,
		"created_at":   order.CreatedAt,
		"paid_at":      order.PaidAt,
	}
}

func billingLedgerEntryView(entry *domain.BillingLedgerEntry) map[string]any {
	return map[string]any{
		"seq":         entry.Seq,
		"id":          entry.ID,
		"amount_usd":  microsToUSD(entry.AmountMicros),
		"kind":        entry.Kind,
		"source_type": entry.SourceType,
		"source_id":   entry.SourceID,
		"description": entry.Description,
		"created_at":  entry.CreatedAt,
	}
}

func requestLogView(log *domain.RequestLog) map[string]any {
	return map[string]any{
		"id":                  log.ID,
		"request_id":          log.RequestID,
		"api_key_id":          log.APIKeyID,
		"model":               log.Model,
		"surface":             log.Surface,
		"status":              log.Status,
		"input_tokens":        log.InputTokens,
		"output_tokens":       log.OutputTokens,
		"cache_read_tokens":   log.CacheReadTokens,
		"cache_create_tokens": log.CacheCreateTokens,
		"cost_usd":            log.CostUSD,
		"duration_ms":         log.DurationMs,
		"created_at":          log.CreatedAt,
	}
}

func limitOffset(r *http.Request, defaultLimit int) (int, int) {
	limit := defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 200 {
		limit = 200
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func usageRange(r *http.Request) (*time.Time, *time.Time) {
	now := time.Now().UTC()
	until := now
	switch strings.TrimSpace(strings.ToLower(r.URL.Query().Get("range"))) {
	case "today":
		local := now.In(time.Local)
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).UTC()
		return &start, &until
	case "30d":
		since := now.Add(-30 * 24 * time.Hour)
		return &since, &until
	case "7d", "":
		since := now.Add(-7 * 24 * time.Hour)
		return &since, &until
	default:
		return nil, nil
	}
}

func microsToUSD(micros int64) float64 {
	return float64(micros) / 1_000_000
}

func usdToMicros(usd float64) int64 {
	return int64(math.Round(usd * 1_000_000))
}

func fenToYuan(fen int64) string {
	return fmt.Sprintf("%.2f", float64(fen)/100)
}

func parseYuanToFen(raw string) (int64, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
}

func compactID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() >= 10 {
			break
		}
	}
	if b.Len() == 0 {
		return "u"
	}
	return b.String()
}

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	if s.trustsProxyIP(host) {
		for _, h := range []string{"X-Forwarded-For", "X-Real-IP"} {
			v := r.Header.Get(h)
			if v == "" {
				continue
			}
			forwarded := strings.TrimSpace(strings.Split(v, ",")[0])
			if forwarded != "" {
				return forwarded
			}
		}
	}
	if host != "" {
		return host
	}
	return "127.0.0.1"
}

func (s *Server) trustsProxyIP(host string) bool {
	if s == nil || s.cfg == nil || strings.TrimSpace(host) == "" {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	for _, raw := range s.cfg.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) zpayCID() string {
	if s.cfg == nil {
		return ""
	}
	return s.cfg.ZPayCID
}
