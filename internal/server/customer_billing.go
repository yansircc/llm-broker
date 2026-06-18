package server

import (
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
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":        "prepaid",
		"status":      cc.User.Status,
		"balance_usd": microsToUSD(balance),
		"credits_usd": microsToUSD(maxInt64Local(balance, 0)),
		"usage_usd":   0,
	})
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
		ClientIP:   clientIP(r),
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
	if err := s.fulfillPaidOrder(r, params["out_trade_no"], params["trade_no"], params["type"], params["money"]); err != nil {
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

func (s *Server) handleCustomerReferrals(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":        cc.User.ReferralCode,
		"url":         s.publicURL(r, "/app/register?ref="+cc.User.ReferralCode),
		"signups":     0,
		"credits_usd": 0,
	})
}

func (s *Server) fulfillPaidOrder(r *http.Request, outTradeNo, zpayTradeNo, paymentType, money string) error {
	order, err := s.store.GetPaymentOrderByOutTradeNo(r.Context(), outTradeNo)
	if err != nil || order == nil {
		return fmt.Errorf("order not found")
	}
	paidFen, err := parseYuanToFen(money)
	if err != nil || paidFen != order.AmountCNYFen {
		return fmt.Errorf("amount mismatch")
	}
	_, err = s.billingService().FulfillPaymentOrder(r.Context(), order, zpayTradeNo, paymentType, time.Now().UTC())
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

func microsToUSD(micros int64) float64 {
	return float64(micros) / 1_000_000
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

func clientIP(r *http.Request) string {
	for _, h := range []string{"x-forwarded-for", "x-real-ip"} {
		v := r.Header.Get(h)
		if v == "" {
			continue
		}
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return "127.0.0.1"
}

func (s *Server) zpayCID() string {
	if s.cfg == nil {
		return ""
	}
	return s.cfg.ZPayCID
}

func maxInt64Local(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
