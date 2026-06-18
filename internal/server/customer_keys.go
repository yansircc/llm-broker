package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *Server) handleCustomerListKeys(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	keys, err := s.store.ListAPIKeysByUser(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list api keys")
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, apiKeyView(key, ""))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCustomerCreateKey(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "default"
	}
	token, err := randomToken("sk")
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to generate api key")
		return
	}
	prefix := token
	if len(prefix) > 16 {
		prefix = prefix[:16] + "..."
	}
	key := &domain.APIKey{
		ID:             uuid.NewString(),
		UserID:         cc.User.ID,
		Name:           req.Name,
		TokenHash:      sha256Hex(token),
		TokenPrefix:    prefix,
		Status:         "active",
		AllowedSurface: domain.SurfaceAll,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.CreateAPIKey(r.Context(), key); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create api key")
		return
	}
	writeJSON(w, http.StatusOK, apiKeyView(key, token))
}

func (s *Server) handleCustomerDeleteKey(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	keyID := r.PathValue("id")
	keys, err := s.store.ListAPIKeysByUser(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to lookup api key")
		return
	}
	found := false
	for _, key := range keys {
		if key.ID == keyID {
			found = true
			break
		}
	}
	if !found {
		writeAdminError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	if err := s.store.DeleteAPIKey(r.Context(), keyID); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to delete api key")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": keyID})
}

func apiKeyView(key *domain.APIKey, token string) map[string]any {
	out := map[string]any{
		"id":           key.ID,
		"name":         key.Name,
		"prefix":       key.TokenPrefix,
		"token_prefix": key.TokenPrefix,
		"status":       key.Status,
		"created_at":   key.CreatedAt,
		"last_used_at": key.LastUsedAt,
	}
	if token != "" {
		out["token"] = token
	}
	return out
}
