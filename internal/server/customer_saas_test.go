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

func TestUnifiedLoginRedirectsAdminEmailToConsole(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.AdminEmails = map[string]struct{}{"admin@example.com": {}}

	registerBody := `{"email":"admin@example.com","password":"password-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	srv.handleCustomerRegister(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("register status %d body %s", resp.Code, resp.Body.String())
	}
	var payload struct {
		RedirectTo string `json:"redirect_to"`
		User       struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, resp.Body.String())
	}
	if payload.RedirectTo != "/console/dashboard" || payload.User.Role != "admin" {
		t.Fatalf("redirect/role = %q/%q, want /console/dashboard/admin", payload.RedirectTo, payload.User.Role)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(customerCookie(t, resp))
	meResp := httptest.NewRecorder()
	srv.handleCustomerMe(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me status %d body %s", meResp.Code, meResp.Body.String())
	}
	payload = struct {
		RedirectTo string `json:"redirect_to"`
		User       struct {
			Role string `json:"role"`
		} `json:"user"`
	}{}
	if err := json.Unmarshal(meResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode me response: %v body=%s", err, meResp.Body.String())
	}
	if payload.RedirectTo != "/console/dashboard" || payload.User.Role != "admin" {
		t.Fatalf("me redirect/role = %q/%q, want /console/dashboard/admin", payload.RedirectTo, payload.User.Role)
	}
}

func TestAdminRoutesUseCustomerSessionRole(t *testing.T) {
	srv := newTestServer(t)
	srv.authMw = auth.NewMiddleware("admin-secret", srv.store)
	srv.cfg.AdminEmails = map[string]struct{}{"admin@example.com": {}}
	now := time.Now().UTC()
	adminUser := &domain.User{
		ID:             "admin-user",
		Email:          "admin@example.com",
		Name:           "admin",
		Status:         "active",
		AllowedSurface: domain.SurfaceNative,
		ReferralCode:   "ADMIN1",
		CreatedAt:      now,
	}
	normalUser := &domain.User{
		ID:             "normal-user",
		Email:          "normal@example.com",
		Name:           "normal",
		Status:         "active",
		AllowedSurface: domain.SurfaceNative,
		ReferralCode:   "NORMAL1",
		CreatedAt:      now,
	}
	if err := srv.store.CreateUser(context.Background(), adminUser); err != nil {
		t.Fatal(err)
	}
	if err := srv.store.CreateUser(context.Background(), normalUser); err != nil {
		t.Fatal(err)
	}
	adminSession := httptest.NewRecorder()
	if _, err := srv.createCustomerSession(adminSession, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), adminUser); err != nil {
		t.Fatalf("create admin session: %v", err)
	}
	normalSession := httptest.NewRecorder()
	if _, err := srv.createCustomerSession(normalSession, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), normalUser); err != nil {
		t.Fatalf("create normal session: %v", err)
	}

	mux := http.NewServeMux()
	srv.registerAdminRoutes(mux)

	adminReq := httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	adminReq.AddCookie(customerCookie(t, adminSession))
	adminResp := httptest.NewRecorder()
	mux.ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusOK {
		t.Fatalf("admin session status %d body %s", adminResp.Code, adminResp.Body.String())
	}

	normalReq := httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	normalReq.AddCookie(customerCookie(t, normalSession))
	normalResp := httptest.NewRecorder()
	mux.ServeHTTP(normalResp, normalReq)
	if normalResp.Code != http.StatusForbidden {
		t.Fatalf("normal session status %d, want %d body %s", normalResp.Code, http.StatusForbidden, normalResp.Body.String())
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

func TestCustomerDataEndpointsExposeLedgerOrdersUsageAndReferralStats(t *testing.T) {
	srv := newTestServer(t)
	now := time.Now().UTC()
	user := &domain.User{
		ID:             "customer-data-user",
		Email:          "data@example.com",
		Name:           "data",
		Status:         "active",
		AllowedSurface: domain.SurfaceNative,
		ReferralCode:   "DATA1",
		CreatedAt:      now,
	}
	if err := srv.store.CreateUser(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	resp := httptest.NewRecorder()
	if _, err := srv.createCustomerSession(resp, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), user); err != nil {
		t.Fatal(err)
	}
	cookie := customerCookie(t, resp)

	if _, err := srv.billingService().Credit(context.Background(), user.ID, "payment_credit", "payment_order", "out-data", "payment:out-data", "payment recharge", 5_000_000); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.billingService().Credit(context.Background(), user.ID, "usage_debit", "request", "req-data-1", "usage:req-data-1", "usage charge", -1_250_000); err != nil {
		t.Fatal(err)
	}
	order := &domain.PaymentOrder{
		ID:                 "order-data-1",
		OutTradeNo:         "out-data",
		UserID:             user.ID,
		Gateway:            "zpay",
		Status:             "paid",
		ProductName:        "credit",
		AmountCNYFen:       500,
		CreditMicros:       5_000_000,
		ExchangeRateMicros: 1_000_000,
		PaymentType:        "alipay",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := srv.store.SavePaymentOrder(context.Background(), order); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.store.InsertRequestLog(context.Background(), &domain.RequestLog{
		UserID:       user.ID,
		RequestID:    "req-data-1",
		APIKeyID:     "key-1",
		AccountID:    "acct-1",
		Provider:     "openai",
		Surface:      string(domain.SurfaceNative),
		Model:        "gpt-5",
		Status:       "ok",
		InputTokens:  1000,
		OutputTokens: 250,
		CostUSD:      1.25,
		DurationMs:   900,
		CreatedAt:    now,
	}); err != nil {
		t.Fatal(err)
	}

	invitee := &domain.User{
		ID:               "customer-data-invitee",
		Email:            "invitee-data@example.com",
		Name:             "invitee",
		Status:           "active",
		AllowedSurface:   domain.SurfaceNative,
		ReferralCode:     "INVITEE1",
		ReferredByUserID: user.ID,
		CreatedAt:        now,
	}
	if err := srv.store.CreateUser(context.Background(), invitee); err != nil {
		t.Fatal(err)
	}
	_ = srv.store.UpsertBillingSetting(context.Background(), "referral_inviter_bonus_micros", "2000000", now)
	if err := srv.billingService().FulfillReferralSignup(context.Background(), invitee); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.billingService().FulfillReferralInviterAfterPayment(context.Background(), invitee.ID); err != nil {
		t.Fatal(err)
	}

	summaryResp := httptest.NewRecorder()
	summaryReq := httptest.NewRequest(http.MethodGet, "/api/billing/summary", nil)
	summaryReq.AddCookie(cookie)
	srv.handleCustomerBillingSummary(summaryResp, summaryReq)
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("summary status %d body %s", summaryResp.Code, summaryResp.Body.String())
	}
	var summary struct {
		BalanceUSD float64 `json:"balance_usd"`
		CreditsUSD float64 `json:"credits_usd"`
		UsageUSD   float64 `json:"usage_usd"`
	}
	if err := json.Unmarshal(summaryResp.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.BalanceUSD != 5.75 || summary.CreditsUSD != 7 || summary.UsageUSD != 1.25 {
		t.Fatalf("summary = %+v, want balance=5.75 credits=7 usage=1.25", summary)
	}

	ledgerResp := httptest.NewRecorder()
	ledgerReq := httptest.NewRequest(http.MethodGet, "/api/billing/ledger", nil)
	ledgerReq.AddCookie(cookie)
	srv.handleCustomerBillingLedger(ledgerResp, ledgerReq)
	if ledgerResp.Code != http.StatusOK || !strings.Contains(ledgerResp.Body.String(), "usage_debit") {
		t.Fatalf("ledger status=%d body=%s", ledgerResp.Code, ledgerResp.Body.String())
	}

	ordersResp := httptest.NewRecorder()
	ordersReq := httptest.NewRequest(http.MethodGet, "/api/payments/orders", nil)
	ordersReq.AddCookie(cookie)
	srv.handleCustomerPaymentOrders(ordersResp, ordersReq)
	if ordersResp.Code != http.StatusOK || !strings.Contains(ordersResp.Body.String(), "out-data") {
		t.Fatalf("orders status=%d body=%s", ordersResp.Code, ordersResp.Body.String())
	}

	usageResp := httptest.NewRecorder()
	usageReq := httptest.NewRequest(http.MethodGet, "/api/usage?key_id=key-1&model=gpt-5", nil)
	usageReq.AddCookie(cookie)
	srv.handleCustomerUsage(usageResp, usageReq)
	if usageResp.Code != http.StatusOK || !strings.Contains(usageResp.Body.String(), "req-data-1") {
		t.Fatalf("usage status=%d body=%s", usageResp.Code, usageResp.Body.String())
	}

	refResp := httptest.NewRecorder()
	refReq := httptest.NewRequest(http.MethodGet, "/api/referrals", nil)
	refReq.AddCookie(cookie)
	srv.handleCustomerReferrals(refResp, refReq)
	if refResp.Code != http.StatusOK || !strings.Contains(refResp.Body.String(), `"paid_invitees":1`) {
		t.Fatalf("referrals status=%d body=%s", refResp.Code, refResp.Body.String())
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
