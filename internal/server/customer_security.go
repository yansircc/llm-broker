package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
)

const (
	customerSecurityWindow = 10 * time.Minute
	signupFailedIPLimit    = 5
	signupTotalIPLimit     = 20
	loginFailedLimit       = 10
)

type customerSecurityAudit struct {
	server    *Server
	r         *http.Request
	kind      string
	ipHash    string
	emailHash string
	done      bool
}

func (s *Server) newCustomerSecurityAudit(r *http.Request, kind, email string) *customerSecurityAudit {
	return &customerSecurityAudit{
		server:    s,
		r:         r,
		kind:      kind,
		ipHash:    s.securityHash("ip", s.clientIP(r)),
		emailHash: s.securityHash("email", strings.TrimSpace(strings.ToLower(email))),
	}
}

func (a *customerSecurityAudit) Success(reason string) {
	_ = a.record(true, reason)
}

func (a *customerSecurityAudit) Fail(reason string) {
	_ = a.record(false, reason)
}

func (a *customerSecurityAudit) record(success bool, reason string) error {
	if a == nil || a.done || a.server == nil || a.server.store == nil {
		return nil
	}
	if err := a.server.store.SaveSecurityEvent(a.r.Context(), &domain.SecurityEvent{
		ID:        uuid.NewString(),
		Kind:      a.kind,
		IPHash:    a.ipHash,
		EmailHash: a.emailHash,
		Success:   success,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	a.done = true
	return nil
}

func (s *Server) recordCustomerSecurityFailure(w http.ResponseWriter, audit *customerSecurityAudit, reason string) bool {
	if err := audit.record(false, reason); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to record security event")
		return false
	}
	return true
}

func (s *Server) recordCustomerSecuritySuccess(w http.ResponseWriter, audit *customerSecurityAudit, reason string) bool {
	if err := audit.record(true, reason); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to record security event")
		return false
	}
	return true
}

func (s *Server) securityHash(kind, value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	salt := "local"
	if s != nil && s.cfg != nil && s.cfg.StaticToken != "" {
		salt = s.cfg.StaticToken
	}
	return sha256Hex("customer-security:" + salt + ":" + kind + ":" + value)
}

func (s *Server) enforceSignupRisk(ctx context.Context, audit *customerSecurityAudit) (string, error) {
	if audit == nil {
		return "", nil
	}
	since := time.Now().UTC().Add(-customerSecurityWindow)
	failed := false
	failedIP, err := s.store.CountSecurityEvents(ctx, domain.SecurityEventQuery{
		Kind:    "signup",
		IPHash:  audit.ipHash,
		Success: &failed,
		Since:   since,
	})
	if err != nil {
		return "risk_unavailable", err
	}
	if failedIP >= signupFailedIPLimit {
		return "signup_failed_ip_limit", fmt.Errorf("too many failed signup attempts")
	}
	totalIP, err := s.store.CountSecurityEvents(ctx, domain.SecurityEventQuery{
		Kind:   "signup",
		IPHash: audit.ipHash,
		Since:  since,
	})
	if err != nil {
		return "risk_unavailable", err
	}
	if totalIP >= signupTotalIPLimit {
		return "signup_total_ip_limit", fmt.Errorf("too many signup attempts")
	}
	return "", nil
}

func (s *Server) enforceLoginRisk(ctx context.Context, audit *customerSecurityAudit) (string, error) {
	if audit == nil {
		return "", nil
	}
	since := time.Now().UTC().Add(-customerSecurityWindow)
	failed := false
	failedIP, err := s.store.CountSecurityEvents(ctx, domain.SecurityEventQuery{
		Kind:    "login",
		IPHash:  audit.ipHash,
		Success: &failed,
		Since:   since,
	})
	if err != nil {
		return "risk_unavailable", err
	}
	failedEmail := 0
	if audit.emailHash != "" {
		failedEmail, err = s.store.CountSecurityEvents(ctx, domain.SecurityEventQuery{
			Kind:      "login",
			EmailHash: audit.emailHash,
			Success:   &failed,
			Since:     since,
		})
		if err != nil {
			return "risk_unavailable", err
		}
	}
	if failedIP >= loginFailedLimit || failedEmail >= loginFailedLimit {
		return "login_failed_limit", fmt.Errorf("too many failed login attempts")
	}
	return "", nil
}

func (s *Server) verifyTurnstile(ctx context.Context, token, remoteIP string) error {
	enabled, _, secret := s.turnstileConfig(ctx)
	if !enabled {
		return nil
	}
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("turnstile secret is not configured")
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("turnstile token required")
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("turnstile verification failed: %s", strings.Join(out.ErrorCodes, ","))
	}
	return nil
}
