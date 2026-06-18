package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
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

func (s *Server) adminAuth(handler http.HandlerFunc) http.Handler {
	next := http.HandlerFunc(handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasMachineAdminCredential(r) {
			s.authMw.Authenticate(requireAdminHandler(next)).ServeHTTP(w, r)
			return
		}
		cc, ok := s.requireCustomer(w, r)
		if !ok {
			return
		}
		if !s.isAdminUser(cc.User) {
			writeAdminError(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}
		ki := &auth.KeyInfo{
			ID:             cc.User.ID,
			CustomerID:     cc.User.ID,
			CredentialKind: "web_session",
			Name:           cc.User.Name,
			Email:          cc.User.Email,
			AllowedSurface: domain.SurfaceAll,
			IsAdmin:        true,
		}
		ctx := context.WithValue(r.Context(), auth.KeyInfoKey, ki)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func hasMachineAdminCredential(r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("x-api-key")) != "" {
		return true
	}
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		return true
	}
	if c, err := r.Cookie("cc_session"); err == nil && c.Value != "" {
		return true
	}
	return false
}
