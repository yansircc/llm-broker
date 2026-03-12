package server

import (
	"fmt"
	"net/http"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
)

func requireSurface(w http.ResponseWriter, r *http.Request, surface domain.Surface) bool {
	ki := auth.GetKeyInfo(r.Context())
	if ki == nil {
		writeSurfaceError(w, http.StatusUnauthorized, "missing authentication context")
		return false
	}
	if ki.IsAdmin || domain.AllowsSurface(ki.AllowedSurface, surface) {
		return true
	}
	writeSurfaceError(w, http.StatusForbidden, fmt.Sprintf("API key cannot access the %s surface", surface))
	return false
}

func requireSurfaceHandler(surface domain.Surface, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireSurface(w, r, surface) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeSurfaceError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":"forbidden","message":"%s"}}`, msg)
}
