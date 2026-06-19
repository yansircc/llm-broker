package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/billing"
	"github.com/yansircc/llm-broker/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const customerSessionCookie = "cc_customer_session"

type customerContext struct {
	User    *domain.User
	Session *domain.WebSession
}

func (s *Server) handleCustomerRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		Name           string `json:"name"`
		ReferralCode   string `json:"referral_code"`
		TurnstileToken string `json:"turnstile_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		audit := s.newCustomerSecurityAudit(r, "signup", "")
		if !s.recordCustomerSecurityFailure(w, audit, "invalid_json") {
			return
		}
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	audit := s.newCustomerSecurityAudit(r, "signup", req.Email)
	defer func() {
		if !audit.done {
			audit.Fail("abandoned")
		}
	}()
	if req.Email == "" || !strings.Contains(req.Email, "@") || len(req.Password) < 8 {
		if !s.recordCustomerSecurityFailure(w, audit, "invalid_request") {
			return
		}
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "valid email and password length >= 8 required")
		return
	}
	if reason, err := s.enforceSignupRisk(r.Context(), audit); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, reason) {
			return
		}
		writeAdminError(w, http.StatusTooManyRequests, "rate_limit", "too many attempts, please retry later")
		return
	}
	if err := s.verifyTurnstile(r.Context(), req.TurnstileToken, s.clientIP(r)); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "turnstile_failed") {
			return
		}
		writeAdminError(w, http.StatusForbidden, "verification_failed", "human verification failed")
		return
	}
	if req.Name == "" {
		req.Name = strings.Split(req.Email, "@")[0]
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "hash_failed") {
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to hash password")
		return
	}
	referredBy := ""
	if req.ReferralCode != "" {
		inviter, err := s.store.GetUserByReferralCode(r.Context(), req.ReferralCode)
		if err != nil {
			if !s.recordCustomerSecurityFailure(w, audit, "referral_lookup_failed") {
				return
			}
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to check referral")
			return
		}
		if inviter != nil {
			referredBy = inviter.ID
		}
	}
	now := time.Now().UTC()
	user := &domain.User{
		ID:               uuid.NewString(),
		Email:            req.Email,
		Name:             req.Name,
		PasswordHash:     string(hash),
		Status:           "active",
		AllowedSurface:   domain.SurfaceNative,
		ReferredByUserID: referredBy,
		CreatedAt:        now,
	}
	if err := s.createUserWithReferralCode(r.Context(), user); err != nil {
		if isReferralCodeAllocationError(err) {
			if !s.recordCustomerSecurityFailure(w, audit, "referral_code_allocation_failed") {
				return
			}
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to allocate referral code")
			return
		}
		if !s.recordCustomerSecurityFailure(w, audit, "email_conflict") {
			return
		}
		writeAdminError(w, http.StatusConflict, "conflict", "email already registered")
		return
	}
	if _, err := s.createCustomerSession(w, r, user); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "session_create_failed") {
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create session")
		return
	}
	if err := s.billingService().FulfillReferralSignup(r.Context(), user); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "referral_fulfill_failed") {
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to fulfill referral")
		return
	}
	if !s.recordCustomerSecuritySuccess(w, audit, "ok") {
		return
	}
	writeJSON(w, http.StatusOK, s.authResponse(user))
}

func (s *Server) handleCustomerLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		TurnstileToken string `json:"turnstile_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		audit := s.newCustomerSecurityAudit(r, "login", "")
		if !s.recordCustomerSecurityFailure(w, audit, "invalid_json") {
			return
		}
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	audit := s.newCustomerSecurityAudit(r, "login", req.Email)
	defer func() {
		if !audit.done {
			audit.Fail("abandoned")
		}
	}()
	if reason, err := s.enforceLoginRisk(r.Context(), audit); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, reason) {
			return
		}
		writeAdminError(w, http.StatusTooManyRequests, "rate_limit", "too many attempts, please retry later")
		return
	}
	if err := s.verifyTurnstile(r.Context(), req.TurnstileToken, s.clientIP(r)); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "turnstile_failed") {
			return
		}
		writeAdminError(w, http.StatusForbidden, "verification_failed", "human verification failed")
		return
	}
	user, err := s.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user == nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "invalid_credentials") {
			return
		}
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "invalid email or password")
		return
	}
	if user.Status != "active" {
		if !s.recordCustomerSecurityFailure(w, audit, "user_disabled") {
			return
		}
		writeAdminError(w, http.StatusForbidden, "forbidden", "user disabled")
		return
	}
	if _, err := s.createCustomerSession(w, r, user); err != nil {
		if !s.recordCustomerSecurityFailure(w, audit, "session_create_failed") {
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create session")
		return
	}
	_ = s.store.UpdateUserLastLogin(r.Context(), user.ID)
	if !s.recordCustomerSecuritySuccess(w, audit, "ok") {
		return
	}
	writeJSON(w, http.StatusOK, s.authResponse(user))
}

func (s *Server) handleCustomerLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(customerSessionCookie); err == nil && c.Value != "" {
		_ = s.store.DeleteWebSessionByTokenHash(r.Context(), sha256Hex(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: customerSessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: s.secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCustomerMe(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.authResponse(cc.User))
}

func (s *Server) createCustomerSession(w http.ResponseWriter, r *http.Request, user *domain.User) (*domain.WebSession, error) {
	raw, err := randomToken("sess")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	ttl := 30 * 24 * time.Hour
	if s.cfg != nil && s.cfg.SessionTTL > 0 {
		ttl = s.cfg.SessionTTL
	}
	session := &domain.WebSession{
		ID:         uuid.NewString(),
		UserID:     user.ID,
		TokenHash:  sha256Hex(raw),
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(ttl),
	}
	if err := s.store.CreateWebSession(r.Context(), session); err != nil {
		return nil, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     customerSessionCookie,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookie(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
	return session, nil
}

func (s *Server) requireCustomer(w http.ResponseWriter, r *http.Request) (*customerContext, bool) {
	c, err := r.Cookie(customerSessionCookie)
	if err != nil || c.Value == "" {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "login required")
		return nil, false
	}
	session, user, err := s.store.GetWebSessionByTokenHash(r.Context(), sha256Hex(c.Value))
	if err != nil || session == nil || user == nil {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "invalid session")
		return nil, false
	}
	if user.Status != "active" {
		writeAdminError(w, http.StatusForbidden, "forbidden", "user disabled")
		return nil, false
	}
	_ = s.store.TouchWebSession(r.Context(), session.ID, time.Now().UTC())
	return &customerContext{User: user, Session: session}, true
}

func (s *Server) publicURL(r *http.Request, path string) string {
	if origin := s.requestOrigin(r); origin != "" {
		return origin + path
	}
	if s != nil && s.cfg != nil && s.cfg.SiteURL != "" {
		return strings.TrimRight(s.cfg.SiteURL, "/") + path
	}
	return path
}

func (s *Server) requestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}

	scheme := "http"
	if forwardedProto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto == "https" || forwardedProto == "http" {
		scheme = forwardedProto
	} else if r.TLS != nil {
		scheme = "https"
	} else if s != nil && s.cfg != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(s.cfg.SiteURL)), "https://") {
		scheme = "https"
	}
	return scheme + "://" + host
}

func firstHeaderValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(value, ",")[0])
}

func (s *Server) secureCookie(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r != nil && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return s != nil && s.cfg != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(s.cfg.SiteURL)), "https://")
}

func (s *Server) billingService() *billing.Service {
	if s.billing != nil {
		return s.billing
	}
	s.billing = billing.NewService(s.store)
	return s.billing
}

func randomToken(prefix string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b)), nil
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Server) authResponse(user *domain.User) map[string]any {
	return map[string]any{
		"user":        s.customerUserView(user),
		"redirect_to": s.loginRedirectPath(user),
	}
}

func (s *Server) customerUserView(user *domain.User) map[string]any {
	return map[string]any{
		"id":                user.ID,
		"email":             user.Email,
		"name":              user.Name,
		"role":              s.userRole(user),
		"status":            user.Status,
		"email_verified_at": user.EmailVerifiedAt,
		"created_at":        user.CreatedAt,
	}
}

func (s *Server) userRole(user *domain.User) string {
	if s.isAdminUser(user) {
		return "admin"
	}
	return "user"
}

func (s *Server) isAdminUser(user *domain.User) bool {
	return user != nil && s.cfg != nil && s.cfg.IsAdminEmail(user.Email)
}

func (s *Server) loginRedirectPath(user *domain.User) string {
	if s.isAdminUser(user) {
		return "/console/dashboard"
	}
	return "/app/dashboard"
}
