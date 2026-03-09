package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/driver"
)

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	options := make([]ProviderOptionResponse, 0, len(s.drivers))
	for _, provider := range sortedDriverProviders(s.drivers) {
		info := s.drivers[provider].Info()
		options = append(options, ProviderOptionResponse{
			ID:                  string(provider),
			Label:               info.Label,
			CallbackPlaceholder: info.CallbackPlaceholder,
			CallbackHint:        info.CallbackHint,
		})
	}
	writeJSON(w, http.StatusOK, options)
}

// handleGenerateAuthURL generates a PKCE-secured auth URL for manual browser-based OAuth.
func (s *Server) handleGenerateAuthURL(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "provider is required")
		return
	}

	drv, ok := s.driverByID(provider)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "unknown provider")
		return
	}
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
		Provider     string `json:"provider"`
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

	provider := req.Provider

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
			req.Code = extractCodeFromCallback(req.CallbackURL)
		}
	}
	if req.Code != "" {
		req.Code = extractCodeFromCallback(req.Code)
	}

	if req.Code == "" || req.CodeVerifier == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
		return
	}

	if provider == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "provider is required")
		return
	}

	drv, ok := s.driverByID(provider)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "unknown provider")
		return
	}
	if drv.Info().OAuthStateRequired && req.State == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "state is required")
		return
	}

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
			a.Identity = result.Identity
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
		Priority:        50,
		PriorityMode:    "auto",
		RefreshTokenEnc: encRefresh,
		AccessTokenEnc:  encAccess,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now().UTC(),
		Identity:        result.Identity,
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

func sortedDriverProviders(drivers map[domain.Provider]driver.Driver) []domain.Provider {
	providers := make([]domain.Provider, 0, len(drivers))
	for provider := range drivers {
		providers = append(providers, provider)
	}
	slices.Sort(providers)
	return providers
}

func (s *Server) driverByID(id string) (driver.Driver, bool) {
	drv, ok := s.drivers[domain.Provider(id)]
	return drv, ok
}

func extractCodeFromCallback(callbackURL string) string {
	s := strings.TrimSpace(callbackURL)
	if s == "" {
		return ""
	}

	parsed, err := url.Parse(s)
	if err != nil || parsed.Scheme == "" {
		if i := strings.Index(s, "#"); i >= 0 {
			s = s[:i]
		}
		if i := strings.Index(s, "&"); i >= 0 {
			s = s[:i]
		}
		if i := strings.Index(s, "?"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimPrefix(s, "code=")
		return strings.TrimSpace(s)
	}
	if code := parsed.Query().Get("code"); code != "" {
		return code
	}
	return strings.TrimSpace(s)
}
