package server

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	options := make([]ProviderOptionResponse, 0, len(s.oauthDrivers))
	for _, provider := range sortedProviders(s.oauthDrivers) {
		info := s.oauthDrivers[provider].Info()
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
	req, err := decodeGenerateAuthURLRequest(r)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.Provider == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "provider is required")
		return
	}

	drv, ok := s.oauthDriverByID(req.Provider)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "unknown provider")
		return
	}
	authURL, session, err := drv.GenerateAuthURL()
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if err := s.validateExchangeCellSelection(nil, req.CellID, drv.Provider()); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	sessionID, err := s.storeOAuthSession(r.Context(), req.Provider, req.CellID, session)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to store oauth session")
		return
	}

	slog.Info("oauth auth URL generated", "sessionId", sessionID, "provider", req.Provider, "cellId", req.CellID)
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": sessionID,
		"auth_url":   authURL,
	})
}

// handleExchangeCode accepts an auth code and exchanges it for tokens.
func (s *Server) handleExchangeCode(w http.ResponseWriter, r *http.Request) {
	req, err := decodeExchangeCodeRequest(r)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if err := s.hydrateExchangeCodeRequest(r.Context(), req); err != nil {
		switch err {
		case errInvalidOAuthSession:
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid or expired session_id")
		case errCorruptOAuthSession:
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "corrupt session data")
		default:
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load oauth session")
		}
		return
	}

	if req.Code == "" || req.CodeVerifier == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
		return
	}

	if req.Provider == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "provider is required")
		return
	}

	drv, ok := s.oauthDriverByID(req.Provider)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "unknown provider")
		return
	}
	if drv.Info().OAuthStateRequired && req.State == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "state is required")
		return
	}

	client, err := s.oauthClientForRoute(req.CellID)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := drv.ExchangeCode(r.Context(), client, req.Code, req.CodeVerifier, req.State)
	if err != nil {
		slog.Error("exchange code failed", "provider", req.Provider, "error", err)
		writeAdminError(w, http.StatusBadGateway, "oauth_error", err.Error())
		return
	}
	if result.Subject == "" {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "exchange returned empty subject")
		return
	}

	existing := s.pool.FindBySubject(drv.Provider(), result.Subject)
	if err := s.validateExchangeCellSelection(existing, req.CellID, drv.Provider()); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	resp, err := s.upsertExchangedAccount(drv, result, req.CellID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to persist exchanged account")
		return
	}
	slog.Info("account persisted via code exchange", "id", resp.ID, "email", resp.Email, "provider", req.Provider, "cellId", req.CellID)

	// Delete the session only after successful exchange so retries are safe.
	if req.SessionID != "" {
		if err := s.pool.DelOAuthSession(r.Context(), req.SessionID); err != nil {
			slog.Warn("failed to delete oauth session after exchange", "sessionId", req.SessionID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
