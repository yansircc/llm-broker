package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

type contextKey string

const KeyInfoKey contextKey = "keyInfo"

// KeyInfo is attached to the request context after authentication.
type KeyInfo struct {
	ID             string // legacy alias for CustomerID
	CustomerID     string
	APIKeyID       string
	CredentialKind string
	Name           string
	Email          string
	AllowedSurface domain.Surface
	BoundAccountID string
	IsAdmin        bool
}

// Middleware validates API tokens against the admin token and user store.
type Middleware struct {
	adminToken string
	store      store.Store
}

func NewMiddleware(adminToken string, s store.Store) *Middleware {
	return &Middleware{adminToken: adminToken, store: s}
}

// Authenticate is the HTTP middleware that validates tokens.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, source := extractToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "authentication_error", "missing or invalid API key")
			return
		}

		keyInfo, err := m.validateToken(r.Context(), token, source)
		if err != nil {
			slog.Warn("auth failed", "error", err)
			writeError(w, http.StatusUnauthorized, "authentication_error", err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), KeyInfoKey, keyInfo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ValidateToken validates a token and returns KeyInfo if valid.
func (m *Middleware) ValidateToken(ctx context.Context, token string) (*KeyInfo, bool) {
	ki, err := m.validateToken(ctx, token, "direct")
	return ki, err == nil && ki != nil
}

func (m *Middleware) validateToken(ctx context.Context, token, source string) (*KeyInfo, error) {
	if subtle.ConstantTimeCompare([]byte(token), []byte(m.adminToken)) == 1 {
		return &KeyInfo{
			ID:             "admin",
			CustomerID:     "admin",
			CredentialKind: "admin",
			Name:           "admin",
			AllowedSurface: domain.SurfaceAll,
			IsAdmin:        true,
		}, nil
	}
	if source == "cookie" {
		return nil, fmt.Errorf("cookie session is not an API credential")
	}

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	apiKey, user, err := m.store.GetAPIKeyByTokenHash(ctx, hashHex)
	if err != nil {
		return nil, fmt.Errorf("token lookup failed: %w", err)
	}
	if apiKey == nil || user == nil {
		return nil, fmt.Errorf("invalid API key")
	}
	if user.Status != "active" {
		return nil, fmt.Errorf("user %s is %s", user.Name, user.Status)
	}
	if apiKey.Status != "active" {
		return nil, fmt.Errorf("api key %s is %s", apiKey.Name, apiKey.Status)
	}
	allowedSurface := apiKey.AllowedSurface
	if allowedSurface == "" {
		allowedSurface = user.AllowedSurface
	}
	if allowedSurface == "" {
		allowedSurface = domain.SurfaceNative
	}

	go m.store.UpdateAPIKeyLastUsed(context.Background(), apiKey.ID)

	return &KeyInfo{
		ID:             user.ID,
		CustomerID:     user.ID,
		APIKeyID:       apiKey.ID,
		CredentialKind: "api_key",
		Name:           user.Name,
		Email:          user.Email,
		AllowedSurface: allowedSurface,
		BoundAccountID: user.BoundAccountID,
	}, nil
}

func extractToken(r *http.Request) (string, string) {
	if key := r.Header.Get("x-api-key"); key != "" {
		return key, "header"
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer "), "header"
	}
	if c, err := r.Cookie("cc_session"); err == nil && c.Value != "" {
		return c.Value, "cookie"
	}
	return "", ""
}

func GetKeyInfo(ctx context.Context) *KeyInfo {
	v, _ := ctx.Value(KeyInfoKey).(*KeyInfo)
	return v
}

func writeError(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":"%s","message":"%s"}}`, errType, msg)
}
