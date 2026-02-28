package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansir/cc-relayer/internal/domain"
	"github.com/yansir/cc-relayer/internal/identity"
	"github.com/yansir/cc-relayer/internal/oauth"
)

// handleGenerateAuthURL generates a PKCE-secured auth URL for manual browser-based OAuth.
func (s *Server) handleGenerateAuthURL(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = "claude"
	}

	var authURL string
	var session oauth.OAuthSession
	var err error

	switch provider {
	case "codex":
		authURL, session, err = oauth.GenerateCodexAuthURL()
	default:
		authURL, session, err = oauth.GenerateAuthURL()
	}
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	sessionID := uuid.New().String()
	sessionData := struct {
		oauth.OAuthSession
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
			oauth.OAuthSession
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

	if provider == "codex" {
		s.exchangeCodexCode(w, r, req.Code, req.CodeVerifier)
		return
	}

	if req.State == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "state is required for Claude OAuth")
		return
	}
	s.exchangeClaudeCode(w, r, req.Code, req.CodeVerifier, req.State)
}

func (s *Server) exchangeClaudeCode(w http.ResponseWriter, r *http.Request, code, verifier, state string) {
	result, err := oauth.ExchangeCode(r.Context(), code, verifier, state)
	if err != nil {
		slog.Error("exchange code failed", "error", err)
		writeAdminError(w, http.StatusBadGateway, "oauth_error", err.Error())
		return
	}

	orgUUID, email, orgName, err := oauth.FetchOrgWithToken(r.Context(), result.AccessToken)
	if err != nil {
		slog.Warn("fetch org info via claude.ai failed, trying API header", "error", err)
		orgUUID = fetchOrgUUIDFromAPIHeader(r.Context(), s.cfg.ClaudeAPIURL, result.AccessToken, s.cfg.ClaudeAPIVersion, s.cfg.ClaudeBetaHeader)
		email = "account-" + time.Now().Format("0102-1504")
	}

	existing := s.findAccountByExtInfoKey("orgUUID", orgUUID)

	extInfo := map[string]interface{}{
		"orgUUID": orgUUID,
		"orgName": orgName,
		"email":   email,
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
			a.Email = email
			a.Status = domain.StatusActive
			a.ExtInfo = extInfo
		})

		slog.Info("account updated via code exchange", "id", existing.ID, "email", email)
		writeJSON(w, http.StatusOK, map[string]string{"id": existing.ID, "email": email, "status": "active"})
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
		Email:           email,
		Provider:        domain.ProviderClaude,
		Status:          domain.StatusActive,
		Schedulable:     true,
		Priority:        50,
		PriorityMode:    "auto",
		RefreshTokenEnc: encRefresh,
		AccessTokenEnc:  encAccess,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now().UTC(),
		ExtInfo:         extInfo,
	}
	now := time.Now().UTC()
	acct.LastRefreshAt = &now

	if err := s.pool.Add(acct); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create account")
		return
	}

	slog.Info("account created via code exchange", "id", acct.ID, "email", email)
	writeJSON(w, http.StatusOK, map[string]string{"id": acct.ID, "email": email, "status": "active"})
}

func (s *Server) exchangeCodexCode(w http.ResponseWriter, r *http.Request, code, verifier string) {
	result, err := oauth.ExchangeCodexCode(r.Context(), code, verifier)
	if err != nil {
		slog.Error("codex exchange code failed", "error", err)
		writeAdminError(w, http.StatusBadGateway, "oauth_error", err.Error())
		return
	}

	email := "codex-" + time.Now().Format("0102-1504")
	extInfo := map[string]interface{}{}
	if result.CodexInfo != nil {
		if result.CodexInfo.Email != "" {
			email = result.CodexInfo.Email
		}
		extInfo["chatgptAccountId"] = result.CodexInfo.ChatGPTAccountID
		extInfo["email"] = result.CodexInfo.Email
		extInfo["orgTitle"] = result.CodexInfo.OrgTitle
	}

	var existing *domain.Account
	if email != "" {
		existing = s.findAccountByExtInfoKey("email", email)
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
			a.Email = email
			a.Status = domain.StatusActive
			a.ExtInfo = extInfo
		})

		slog.Info("codex account updated via code exchange", "id", existing.ID, "email", email)
		writeJSON(w, http.StatusOK, map[string]string{"id": existing.ID, "email": email, "status": "active"})
		return
	}

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
		Email:           email,
		Provider:        domain.ProviderCodex,
		Status:          domain.StatusActive,
		Schedulable:     true,
		Priority:        50,
		PriorityMode:    "auto",
		RefreshTokenEnc: encRefresh,
		AccessTokenEnc:  encAccess,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now().UTC(),
		ExtInfo:         extInfo,
	}
	now := time.Now().UTC()
	acct.LastRefreshAt = &now

	if err := s.pool.Add(acct); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create account")
		return
	}

	slog.Info("codex account created via code exchange", "id", acct.ID, "email", email)
	writeJSON(w, http.StatusOK, map[string]string{"id": acct.ID, "email": email, "status": "active"})
}

func (s *Server) findAccountByExtInfoKey(key, value string) *domain.Account {
	if value == "" {
		return nil
	}
	for _, a := range s.pool.List() {
		if a.ExtInfo != nil {
			if v, ok := a.ExtInfo[key].(string); ok && v == value {
				return a
			}
		}
	}
	return nil
}

func fetchOrgUUIDFromAPIHeader(ctx context.Context, apiURL, accessToken, apiVersion, betaHeader string) string {
	body := `{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	identity.SetRequiredHeaders(req.Header, accessToken, apiVersion, betaHeader)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return resp.Header.Get("Anthropic-Organization-Id")
}
