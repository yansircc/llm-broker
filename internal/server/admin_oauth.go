package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
	"github.com/yansir/cc-relayer/internal/oauth"
)

// handleGenerateAuthURL generates a PKCE-secured auth URL for manual browser-based OAuth.
func (s *Server) handleGenerateAuthURL(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = "claude"
	}

	drv := s.drivers[resolveProvider(provider)]
	authURL, session, err := drv.GenerateAuthURL()
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	sessionID := uuid.New().String()
	sessionData := struct {
		driver.OAuthSession
		Provider string `json:"provider"`
	}{session, provider}
	sessionJSON, _ := json.Marshal(sessionData)

	s.pool.SetOAuthSession(sessionID, string(sessionJSON), 10*time.Minute)

	slog.Info("oauth auth URL generated", "sessionId", sessionID, "provider", provider)
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"auth_url":   authURL,
	})
}

// handleExchangeCode accepts an auth code and exchanges it for tokens.
func (s *Server) handleExchangeCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID    string `json:"session_id"`
		CallbackURL  string `json:"callback_url"`
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
		State        string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	provider := "claude"

	if req.SessionID != "" {
		sessionJSON, ok := s.pool.GetDelOAuthSession(req.SessionID)
		if !ok {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid or expired session_id")
			return
		}
		var session struct {
			driver.OAuthSession
			Provider string `json:"provider"`
		}
		if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "corrupt session data")
			return
		}
		req.CodeVerifier = session.CodeVerifier
		req.State = session.State
		if session.Provider != "" {
			provider = session.Provider
		}
		if req.CallbackURL != "" && req.Code == "" {
			req.Code = oauth.ExtractCodeFromCallback(req.CallbackURL)
		}
	}
	if req.Code != "" {
		req.Code = oauth.ExtractCodeFromCallback(req.Code)
	}

	if req.Code == "" || req.CodeVerifier == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
		return
	}

	if provider != "codex" && req.State == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "state is required for Claude OAuth")
		return
	}

	// Unified exchange via driver
	drv := s.drivers[resolveProvider(provider)]
	result, err := drv.ExchangeCode(r.Context(), req.Code, req.CodeVerifier, req.State)
	if err != nil {
		slog.Error("exchange code failed", "provider", provider, "error", err)
		writeAdminError(w, http.StatusBadGateway, "oauth_error", err.Error())
		return
	}
	if result.Subject == "" {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "exchange returned empty subject")
		return
	}

	// Dedup by provider + subject
	existing := s.pool.FindBySubject(drv.Provider(), result.Subject)

	// Fallback: pre-migration accounts have subject='' — search by ExtInfo key
	if existing == nil {
		switch drv.Provider() {
		case domain.ProviderClaude:
			existing = s.pool.FindByExtInfoKey(drv.Provider(), "orgUUID", result.Subject)
		case domain.ProviderCodex:
			existing = s.pool.FindByExtInfoKey(drv.Provider(), "chatgptAccountId", result.Subject)
		}
	}

	if existing != nil {
		encAccess, err := s.tokens.EncryptToken(result.AccessToken)
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to encrypt token")
			return
		}
		encRefresh, err := s.tokens.EncryptToken(result.RefreshToken)
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to encrypt token")
			return
		}
		expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UnixMilli()
		if err := s.pool.StoreTokens(existing.ID, encAccess, encRefresh, expiresAt); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to store tokens")
			return
		}
		_ = s.pool.Update(existing.ID, func(a *domain.Account) {
			a.Email = result.Email
			a.Status = domain.StatusActive
			a.ExtInfo = result.ExtInfo
			a.Subject = result.Subject
		})

		slog.Info("account updated via code exchange", "id", existing.ID, "email", result.Email, "provider", provider)
		writeJSON(w, http.StatusOK, map[string]string{"id": existing.ID, "email": result.Email, "status": "active"})
		return
	}

	// Create new account
	encRefresh, err := s.tokens.EncryptToken(result.RefreshToken)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to encrypt token")
		return
	}
	encAccess, err := s.tokens.EncryptToken(result.AccessToken)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to encrypt token")
		return
	}
	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UnixMilli()

	acct := &domain.Account{
		ID:              uuid.New().String(),
		Email:           result.Email,
		Provider:        drv.Provider(),
		Subject:         result.Subject,
		Status:          domain.StatusActive,
		Schedulable:     true,
		Priority:        50,
		PriorityMode:    "auto",
		RefreshTokenEnc: encRefresh,
		AccessTokenEnc:  encAccess,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now().UTC(),
		ExtInfo:         result.ExtInfo,
	}
	now := time.Now().UTC()
	acct.LastRefreshAt = &now

	if err := s.pool.Add(acct); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create account")
		return
	}

	slog.Info("account created via code exchange", "id", acct.ID, "email", result.Email, "provider", provider)
	writeJSON(w, http.StatusOK, map[string]string{"id": acct.ID, "email": result.Email, "status": "active"})
}

// resolveProvider maps a string to a domain.Provider.
func resolveProvider(s string) domain.Provider {
	if s == "codex" {
		return domain.ProviderCodex
	}
	return domain.ProviderClaude
}
