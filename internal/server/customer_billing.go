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
	"github.com/yansircc/llm-broker/internal/payments"
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
		Provider  string  `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AmountUSD <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "amount_usd required")
		return
	}
	if req.Type == "" {
		req.Type = "alipay"
	}
	integration, provider, providerCfg, err := s.paymentProviderConfig(r.Context(), req.Provider, nil)
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "payment_unavailable", err.Error())
		return
	}
	now := time.Now().UTC()
	creditMicros := int64(math.Round(req.AmountUSD * 1_000_000))
	rate := s.cnyToUSDRateMicros(r)
	amountFen := (creditMicros*100 + rate - 1) / rate
	outTradeNo := fmt.Sprintf("%s_%d", compactID(cc.User.ID), now.UnixMilli())
	order := &domain.PaymentOrder{
		ID:                   uuid.NewString(),
		OutTradeNo:           outTradeNo,
		UserID:               cc.User.ID,
		Gateway:              provider.Name(),
		IntegrationID:        integration.ID,
		Status:               "pending",
		ProductName:          "LLM relay credit",
		AmountCNYFen:         amountFen,
		CreditMicros:         creditMicros,
		ExchangeRateMicros:   rate,
		PaymentType:          req.Type,
		Method:               req.Type,
		SettlementCurrency:   "CNY",
		AmountMinor:          amountFen,
		ProviderMetadataJSON: "{}",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.store.SavePaymentOrder(r.Context(), order); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create order")
		return
	}
	created, err := provider.CreateOrder(r.Context(), providerCfg, payments.CreateOrderRequest{
		Method:      req.Type,
		Name:        order.ProductName,
		AmountMinor: amountFen,
		OutTradeNo:  outTradeNo,
		NotifyURL:   s.publicURL(r, "/api/payments/notify/"+provider.Name()),
		ClientIP:    s.clientIP(r),
		UserID:      cc.User.ID,
	})
	if err != nil {
		order.Status = "failed"
		order.UpdatedAt = time.Now().UTC()
		_ = s.store.SavePaymentOrder(r.Context(), order)
		writeAdminError(w, http.StatusBadGateway, "payment_error", err.Error())
		return
	}
	order.ProviderOrderID = created.ProviderOrderID
	order.ProviderPaymentID = created.ProviderPaymentID
	order.Method = created.Method
	order.PaymentType = created.Method
	order.ZpayTradeNo = created.ProviderPaymentID
	order.QRCode = created.QRCode
	order.QRImage = created.QRImage
	order.CheckoutURL = created.CheckoutURL
	if created.MetadataJSON != "" {
		order.ProviderMetadataJSON = created.MetadataJSON
	}
	order.UpdatedAt = time.Now().UTC()
	_ = s.store.SavePaymentOrder(r.Context(), order)
	writeJSON(w, http.StatusOK, paymentOrderView(order, created.CheckoutURL))
}

func (s *Server) handlePaymentNotify(w http.ResponseWriter, r *http.Request) {
	params, err := paymentParams(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
		return
	}
	order, _ := s.store.GetPaymentOrderByOutTradeNo(r.Context(), params["out_trade_no"])
	integration, provider, providerCfg, err := s.paymentProviderConfig(r.Context(), r.PathValue("provider"), order)
	if err != nil {
		if s.handleLegacyZPayNotify(w, r, params, order) {
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("fail"))
		return
	}
	event, valid, err := provider.VerifyWebhook(r.Context(), providerCfg, r, order)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
		return
	}
	_ = s.store.SavePaymentEvent(r.Context(), &domain.PaymentEvent{
		ID:             uuid.NewString(),
		OrderID:        event.OutTradeNo,
		Gateway:        provider.Name(),
		EventType:      "notify",
		ValidSignature: valid,
		PayloadJSON:    event.PayloadJSON,
		CreatedAt:      time.Now().UTC(),
	})
	s.recordIntegrationEvent(r.Context(), integration, "payment_notify", valid, signatureErrorCode(valid), map[string]any{"out_trade_no": event.OutTradeNo, "status": event.Status})
	if !valid {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("fail"))
		return
	}
	if event.Status != "TRADE_SUCCESS" {
		w.Write([]byte("success"))
		return
	}
	if err := s.fulfillPaidOrder(r.Context(), event.OutTradeNo, event.ProviderPaymentID, event.Method, event.AmountMinor, event.Currency); err != nil {
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
	integration, provider, providerCfg, err := s.paymentProviderConfig(ctx, order.Gateway, order)
	if err != nil {
		if order.IntegrationID == "" && s.zpayClient != nil {
			return s.refreshLegacyZPayOrder(ctx, order, now)
		}
		return nil, err
	}
	event, err := provider.QueryOrder(ctx, providerCfg, order)
	if err != nil {
		order.UpdatedAt = now
		_ = s.store.SavePaymentOrder(ctx, order)
		return nil, err
	}
	_ = s.store.SavePaymentEvent(ctx, &domain.PaymentEvent{
		ID:             uuid.NewString(),
		OrderID:        order.OutTradeNo,
		Gateway:        order.Gateway,
		EventType:      "query",
		ValidSignature: false,
		PayloadJSON:    event.PayloadJSON,
		CreatedAt:      now,
	})
	s.recordIntegrationEvent(ctx, integration, "payment_query", true, "", map[string]any{"out_trade_no": event.OutTradeNo, "status": event.Status})
	if event.Status == "TRADE_SUCCESS" {
		if err := s.fulfillPaidOrder(ctx, order.OutTradeNo, event.ProviderPaymentID, event.Method, event.AmountMinor, event.Currency); err != nil {
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

func (s *Server) refreshLegacyZPayOrder(ctx context.Context, order *domain.PaymentOrder, now time.Time) (*domain.PaymentOrder, error) {
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
		amountFen, err := parseYuanToFen(resp.Money)
		if err != nil {
			return nil, err
		}
		if err := s.fulfillPaidOrder(ctx, order.OutTradeNo, resp.TradeNo, resp.Type, amountFen, "CNY"); err != nil {
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

func (s *Server) paymentProviderConfig(ctx context.Context, requested string, order *domain.PaymentOrder) (*domain.Integration, payments.Provider, payments.Config, error) {
	requested = strings.TrimSpace(strings.ToLower(requested))
	if order != nil && strings.TrimSpace(order.IntegrationID) != "" {
		integration, err := s.store.GetIntegration(ctx, order.IntegrationID)
		if err != nil {
			return nil, nil, payments.Config{}, err
		}
		if integration != nil {
			return s.paymentConfigForIntegration(ctx, integration)
		}
	}
	providerFilter := requested
	if providerFilter == "" && order != nil {
		providerFilter = order.Gateway
	}
	integrations, err := s.enabledIntegrations(ctx, "payment", providerFilter)
	if err != nil {
		return nil, nil, payments.Config{}, err
	}
	for _, integration := range integrations {
		provider := payments.ProviderByName(integration.Provider)
		if provider == nil {
			continue
		}
		cfg, err := s.decryptedIntegrationConfig(ctx, integration)
		if err != nil {
			return nil, nil, payments.Config{}, err
		}
		return integration, provider, cfg, nil
	}
	if providerFilter == "" || providerFilter == "zpay" || providerFilter == "7pay" {
		if s.cfg != nil && s.cfg.ZPayPID != "" && s.cfg.ZPayKey != "" {
			integration := &domain.Integration{ID: "env_zpay", Kind: "payment", Provider: "zpay", DisplayName: "7pay / ZPay", Enabled: true}
			cfg := payments.Config{
				Public:  map[string]string{"pid": s.cfg.ZPayPID, "cid": s.cfg.ZPayCID},
				Secrets: map[string]string{"key": s.cfg.ZPayKey},
			}
			return integration, payments.ZPayProvider{}, cfg, nil
		}
	}
	if providerFilter == "" {
		return nil, nil, payments.Config{}, fmt.Errorf("no payment provider is configured")
	}
	return nil, nil, payments.Config{}, fmt.Errorf("payment provider %q is not configured", providerFilter)
}

func (s *Server) handleLegacyZPayNotify(w http.ResponseWriter, r *http.Request, params map[string]string, order *domain.PaymentOrder) bool {
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.ZPayKey) == "" {
		return false
	}
	valid := zpay.Verify(params, s.cfg.ZPayKey)
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
		return true
	}
	if params["trade_status"] != "TRADE_SUCCESS" {
		w.Write([]byte("success"))
		return true
	}
	amountFen, err := parseYuanToFen(params["money"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
		return true
	}
	if order != nil && order.IntegrationID != "" {
		return false
	}
	if err := s.fulfillPaidOrder(r.Context(), params["out_trade_no"], params["trade_no"], params["type"], amountFen, "CNY"); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
		return true
	}
	w.Write([]byte("success"))
	return true
}

func (s *Server) paymentConfigForIntegration(ctx context.Context, integration *domain.Integration) (*domain.Integration, payments.Provider, payments.Config, error) {
	provider := payments.ProviderByName(integration.Provider)
	if provider == nil {
		return nil, nil, payments.Config{}, fmt.Errorf("payment provider %q is not supported", integration.Provider)
	}
	cfg, err := s.decryptedIntegrationConfig(ctx, integration)
	if err != nil {
		return nil, nil, payments.Config{}, err
	}
	return integration, provider, cfg, nil
}

func signatureErrorCode(valid bool) string {
	if valid {
		return ""
	}
	return "invalid_signature"
}

func (s *Server) fulfillPaidOrder(ctx context.Context, outTradeNo, providerPaymentID, paymentType string, amountMinor int64, currency string) error {
	order, err := s.store.GetPaymentOrderByOutTradeNo(ctx, outTradeNo)
	if err != nil || order == nil {
		return fmt.Errorf("order not found")
	}
	expectedCurrency := order.SettlementCurrency
	if expectedCurrency == "" {
		expectedCurrency = "CNY"
	}
	expectedAmount := order.AmountMinor
	if expectedAmount == 0 {
		expectedAmount = order.AmountCNYFen
	}
	if !strings.EqualFold(strings.TrimSpace(currency), expectedCurrency) || amountMinor != expectedAmount {
		return fmt.Errorf("amount mismatch")
	}
	if _, err := s.billingService().FulfillPaymentOrder(ctx, order, providerPaymentID, paymentType, time.Now().UTC()); err != nil {
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
	if checkoutURL == "" {
		checkoutURL = order.CheckoutURL
	}
	amountCNY := float64(order.AmountCNYFen) / 100
	if strings.EqualFold(order.SettlementCurrency, "CNY") && order.AmountMinor > 0 {
		amountCNY = float64(order.AmountMinor) / 100
	}
	return map[string]any{
		"id":           order.OutTradeNo,
		"out_trade_no": order.OutTradeNo,
		"provider":     order.Gateway,
		"method":       firstNonEmpty(order.Method, order.PaymentType),
		"status":       order.Status,
		"amount_usd":   microsToUSD(order.CreditMicros),
		"amount_cny":   amountCNY,
		"currency":     firstNonEmpty(order.SettlementCurrency, "CNY"),
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

func parseYuanToFen(raw string) (int64, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
