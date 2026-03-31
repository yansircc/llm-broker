package server

import (
	"encoding/json"
	"net/http"

	"github.com/yansircc/llm-broker/internal/auth"
)

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	ki := auth.GetKeyInfo(r.Context())
	if ki == nil || !ki.IsAdmin {
		writeAdminError(w, http.StatusForbidden, "forbidden", "admin access required")
		return false
	}
	return true
}

func requireAdminHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireAdmin(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}

	ki, valid := s.authMw.ValidateToken(r.Context(), req.Token)
	if !valid || ki == nil {
		writeAdminError(w, http.StatusUnauthorized, "authentication_error", "invalid token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "cc_session",
		Value:    req.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	})
	writeJSON(w, http.StatusOK, struct {
		Status  string `json:"status"`
		IsAdmin bool   `json:"is_admin"`
		Name    string `json:"name"`
	}{"ok", ki.IsAdmin, ki.Name})
}
