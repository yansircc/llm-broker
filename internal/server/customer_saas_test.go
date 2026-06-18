package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/email"
	"github.com/yansircc/llm-broker/internal/payments/zpay"
)

type captureEmailSender struct {
	messages []email.Message
}

func (s *captureEmailSender) Send(_ context.Context, msg email.Message) error {
	s.messages = append(s.messages, msg)
	return nil
}

func TestCustomerRegistrationCreatesUsableKeyAndFulfillsReferral(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.ZPayKey = "secret"
	mailer := &captureEmailSender{}
	srv.emailSender = mailer
	now := time.Now().UTC()

	inviter := &domain.User{
		ID:             "inviter-1",
		Email:          "inviter@example.com",
		Name:           "inviter",
		Status:         "active",
		AllowedSurface: domain.SurfaceNative,
		ReferralCode:   "INVITE1",
		CreatedAt:      now,
	}
	if err := srv.store.CreateUser(context.Background(), inviter); err != nil {
		t.Fatal(err)
	}
	_ = srv.store.UpsertBillingSetting(context.Background(), "referral_new_user_bonus_micros", "1000000", now)
	_ = srv.store.UpsertBillingSetting(context.Background(), "referral_inviter_bonus_micros", "2000000", now)

	registerBody := `{"email":"invitee@example.com","password":"password-1","referral_code":"INVITE1"}`
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerResp := httptest.NewRecorder()
	srv.handleCustomerRegister(registerResp, registerReq)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status %d body %s", registerResp.Code, registerResp.Body.String())
	}
	if len(mailer.messages) != 0 {
		t.Fatalf("verification emails = %d, want 0", len(mailer.messages))
	}
	sessionCookie := customerCookie(t, registerResp)

	invitee, err := srv.store.GetUserByEmail(context.Background(), "invitee@example.com")
	if err != nil || invitee == nil {
		t.Fatalf("invitee lookup: user=%#v err=%v", invitee, err)
	}
	if invitee.EmailVerifiedAt != nil {
		t.Fatal("new invitee should start unverified")
	}
	assertBalanceMicros(t, srv, invitee.ID, 1_000_000)
	assertBalanceMicros(t, srv, inviter.ID, 0)

	createKeyReq := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(`{"name":"default"}`))
	createKeyReq.AddCookie(sessionCookie)
	createKeyResp := httptest.NewRecorder()
	srv.handleCustomerCreateKey(createKeyResp, createKeyReq)
	if createKeyResp.Code != http.StatusOK {
		t.Fatalf("create key status %d body %s", createKeyResp.Code, createKeyResp.Body.String())
	}
	var keyResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(createKeyResp.Body.Bytes(), &keyResp); err != nil || keyResp.Token == "" {
		t.Fatalf("missing created api token: token=%q err=%v body=%s", keyResp.Token, err, createKeyResp.Body.String())
	}
	ki, ok := auth.NewMiddleware("admin-token", srv.store).ValidateToken(context.Background(), keyResp.Token)
	if !ok || ki.CustomerID != invitee.ID || ki.AllowedSurface != domain.SurfaceAll {
		t.Fatalf("created api key did not authenticate as unverified invitee: ok=%v keyInfo=%#v", ok, ki)
	}

	order := &domain.PaymentOrder{
		ID:                 "invitee-order-1",
		OutTradeNo:         "invitee-out-1",
		UserID:             invitee.ID,
		Gateway:            "zpay",
		Status:             "pending",
		ProductName:        "credit",
		AmountCNYFen:       990,
		CreditMicros:       9_900_000,
		ExchangeRateMicros: 1_000_000,
		PaymentType:        "alipay",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := srv.store.SavePaymentOrder(context.Background(), order); err != nil {
		t.Fatal(err)
	}
	notify := signedZPayNotify("secret", map[string]string{
		"out_trade_no": "invitee-out-1",
		"trade_no":     "zpay-invitee-1",
		"trade_status": "TRADE_SUCCESS",
		"type":         "alipay",
		"money":        "9.90",
	})
	for i := 0; i < 2; i++ {
		resp := httptest.NewRecorder()
		srv.handlePaymentNotify(resp, httptest.NewRequest(http.MethodGet, "/api/payments/notify?"+notify.Encode(), nil))
		if resp.Code != http.StatusOK || strings.TrimSpace(resp.Body.String()) != "success" {
			t.Fatalf("invitee notify #%d status=%d body=%q", i+1, resp.Code, resp.Body.String())
		}
	}
	assertBalanceMicros(t, srv, invitee.ID, 10_900_000)
	assertBalanceMicros(t, srv, inviter.ID, 2_000_000)
}

func TestPaymentNotifyCreditsOnceAndRejectsAmountMismatch(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.ZPayKey = "secret"
	now := time.Now().UTC()
	user := &domain.User{
		ID:             "payer-1",
		Email:          "payer@example.com",
		Name:           "payer",
		Status:         "active",
		AllowedSurface: domain.SurfaceNative,
		ReferralCode:   "PAYER1",
		CreatedAt:      now,
	}
	if err := srv.store.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}

	order := &domain.PaymentOrder{
		ID:                 "order-1",
		OutTradeNo:         "out-1",
		UserID:             user.ID,
		Gateway:            "zpay",
		Status:             "pending",
		ProductName:        "credit",
		AmountCNYFen:       990,
		CreditMicros:       9_900_000,
		ExchangeRateMicros: 1_000_000,
		PaymentType:        "alipay",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := srv.store.SavePaymentOrder(context.Background(), order); err != nil {
		t.Fatal(err)
	}

	notify := signedZPayNotify("secret", map[string]string{
		"out_trade_no": "out-1",
		"trade_no":     "zpay-1",
		"trade_status": "TRADE_SUCCESS",
		"type":         "alipay",
		"money":        "9.90",
	})
	for i := 0; i < 2; i++ {
		resp := httptest.NewRecorder()
		srv.handlePaymentNotify(resp, httptest.NewRequest(http.MethodGet, "/api/payments/notify?"+notify.Encode(), nil))
		if resp.Code != http.StatusOK || strings.TrimSpace(resp.Body.String()) != "success" {
			t.Fatalf("notify #%d status=%d body=%q", i+1, resp.Code, resp.Body.String())
		}
	}
	assertBalanceMicros(t, srv, user.ID, 9_900_000)
	paid, _ := srv.store.GetPaymentOrderByOutTradeNo(context.Background(), "out-1")
	if paid == nil || paid.Status != "paid" || paid.ZpayTradeNo != "zpay-1" {
		t.Fatalf("paid order = %#v", paid)
	}

	mismatch := &domain.PaymentOrder{
		ID:                 "order-2",
		OutTradeNo:         "out-2",
		UserID:             user.ID,
		Gateway:            "zpay",
		Status:             "pending",
		ProductName:        "credit",
		AmountCNYFen:       990,
		CreditMicros:       9_900_000,
		ExchangeRateMicros: 1_000_000,
		PaymentType:        "alipay",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := srv.store.SavePaymentOrder(context.Background(), mismatch); err != nil {
		t.Fatal(err)
	}
	badNotify := signedZPayNotify("secret", map[string]string{
		"out_trade_no": "out-2",
		"trade_no":     "zpay-2",
		"trade_status": "TRADE_SUCCESS",
		"type":         "alipay",
		"money":        "0.01",
	})
	resp := httptest.NewRecorder()
	srv.handlePaymentNotify(resp, httptest.NewRequest(http.MethodGet, "/api/payments/notify?"+badNotify.Encode(), nil))
	if resp.Code != http.StatusBadRequest || strings.TrimSpace(resp.Body.String()) != "fail" {
		t.Fatalf("mismatch notify status=%d body=%q", resp.Code, resp.Body.String())
	}
	assertBalanceMicros(t, srv, user.ID, 9_900_000)
}

func TestCustomerSessionRejectsDisabledUser(t *testing.T) {
	srv := newTestServer(t)
	now := time.Now().UTC()
	user := &domain.User{
		ID:              "disabled-user",
		Email:           "disabled@example.com",
		Name:            "disabled",
		EmailVerifiedAt: &now,
		Status:          "active",
		AllowedSurface:  domain.SurfaceNative,
		ReferralCode:    "DISABLED1",
		CreatedAt:       now,
	}
	if err := srv.store.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	sessionResp := httptest.NewRecorder()
	if _, err := srv.createCustomerSession(sessionResp, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), user); err != nil {
		t.Fatalf("createCustomerSession: %v", err)
	}
	if err := srv.store.UpdateUserStatus(context.Background(), user.ID, "disabled"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(`{"name":"blocked"}`))
	req.AddCookie(customerCookie(t, sessionResp))
	resp := httptest.NewRecorder()
	srv.handleCustomerCreateKey(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("disabled session status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestCustomerSessionCookieSecureWhenSiteURLIsHTTPS(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.SiteURL = "https://relay.example.com"
	now := time.Now().UTC()
	user := &domain.User{
		ID:             "secure-cookie-user",
		Email:          "secure@example.com",
		Name:           "secure",
		Status:         "active",
		AllowedSurface: domain.SurfaceNative,
		ReferralCode:   "SECURE1",
		CreatedAt:      now,
	}
	if err := srv.store.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}

	resp := httptest.NewRecorder()
	if _, err := srv.createCustomerSession(resp, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), user); err != nil {
		t.Fatalf("createCustomerSession: %v", err)
	}

	if cookie := customerCookie(t, resp); !cookie.Secure {
		t.Fatalf("customer session cookie Secure = false, want true when SITE_URL is https")
	}
}

func TestPublicURLUsesCurrentRequestOrigin(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.SiteURL = "https://configured.example"

	req := httptest.NewRequest(http.MethodGet, "/api/referrals", nil)
	req.Host = "current.example"
	req.Header.Set("X-Forwarded-Proto", "https")

	got := srv.publicURL(req, "/app/register?ref=INVITE")
	want := "https://current.example/app/register?ref=INVITE"
	if got != want {
		t.Fatalf("publicURL() = %q, want %q", got, want)
	}
}

func TestPublicURLUsesForwardedHost(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.SiteURL = "https://configured.example"

	req := httptest.NewRequest(http.MethodGet, "/api/payments/create", nil)
	req.Host = "127.0.0.1:3000"
	req.Header.Set("X-Forwarded-Host", "pay.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")

	got := srv.publicURL(req, "/api/payments/notify")
	want := "https://pay.example.com/api/payments/notify"
	if got != want {
		t.Fatalf("publicURL() = %q, want %q", got, want)
	}
}

func customerCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == customerSessionCookie {
			return cookie
		}
	}
	t.Fatal("customer session cookie not set")
	return nil
}

func verificationToken(t *testing.T, msg email.Message) string {
	t.Helper()
	const marker = "token="
	idx := strings.Index(msg.Text, marker)
	if idx < 0 {
		t.Fatalf("verification message missing token: %q", msg.Text)
	}
	token := msg.Text[idx+len(marker):]
	if end := strings.IndexAny(token, "\r\n& "); end >= 0 {
		token = token[:end]
	}
	if token == "" {
		t.Fatalf("empty verification token in message: %q", msg.Text)
	}
	return token
}

func assertBalanceMicros(t *testing.T, srv *Server, userID string, want int64) {
	t.Helper()
	got, _, err := srv.billingService().Balance(context.Background(), userID)
	if err != nil {
		t.Fatalf("Balance(%s): %v", userID, err)
	}
	if got != want {
		t.Fatalf("Balance(%s) = %d, want %d", userID, got, want)
	}
}

func signedZPayNotify(key string, params map[string]string) url.Values {
	params["sign"] = zpay.Sign(params, key)
	values := make(url.Values, len(params))
	for k, v := range params {
		values.Set(k, v)
	}
	return values
}
