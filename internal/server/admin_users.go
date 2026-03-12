package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

// ---------------------------------------------------------------------------
// User CRUD (admin only)
// ---------------------------------------------------------------------------

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		AllowedSurface string `json:"allowed_surface"`
		BoundAccountID string `json:"bound_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	allowedSurface, err := parseUserAllowedSurface(req.AllowedSurface)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.validateUserBoundAccount(req.BoundAccountID); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	plaintext, hashStr, prefix, err := generateUserToken(req.Name)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to generate token")
		return
	}
	u := &domain.User{
		ID:             uuid.New().String(),
		Name:           req.Name,
		TokenHash:      hashStr,
		TokenPrefix:    prefix,
		Status:         "active",
		AllowedSurface: allowedSurface,
		BoundAccountID: req.BoundAccountID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		slog.Error("create user failed", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create user")
		return
	}

	slog.Info("user created", "id", u.ID, "name", u.Name)
	writeJSON(w, http.StatusOK, struct {
		ID                string         `json:"id"`
		Name              string         `json:"name"`
		Token             string         `json:"token"`
		AllowedSurface    domain.Surface `json:"allowed_surface"`
		BoundAccountID    string         `json:"bound_account_id,omitempty"`
		BoundAccountEmail string         `json:"bound_account_email,omitempty"`
	}{
		ID:                u.ID,
		Name:              u.Name,
		Token:             plaintext,
		AllowedSurface:    u.AllowedSurface,
		BoundAccountID:    u.BoundAccountID,
		BoundAccountEmail: s.boundAccountEmail(u.BoundAccountID),
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	if users == nil {
		users = []*domain.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to delete user")
		return
	}
	slog.Info("user deleted", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleRegenerateUserToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to lookup user")
		return
	}
	var userName string
	for _, u := range users {
		if u.ID == id {
			userName = u.Name
			break
		}
	}
	if userName == "" {
		writeAdminError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	plaintext, hashStr, prefix, err := generateUserToken(userName)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to generate token")
		return
	}
	if err := s.store.UpdateUserToken(r.Context(), id, hashStr, prefix); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to update token")
		return
	}

	slog.Info("user token regenerated", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{
		"id":    id,
		"token": plaintext,
	})
}

func (s *Server) handleUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Status != "active" && req.Status != "disabled") {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "status must be 'active' or 'disabled'")
		return
	}
	if err := s.store.UpdateUserStatus(r.Context(), id, req.Status); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to update user status")
		return
	}
	slog.Info("user status updated", "id", id, "status", req.Status)
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": req.Status})
}

func (s *Server) handleUpdateUserPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		AllowedSurface string `json:"allowed_surface"`
		BoundAccountID string `json:"bound_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	allowedSurface, err := parseUserAllowedSurface(req.AllowedSurface)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.validateUserBoundAccount(req.BoundAccountID); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.store.UpdateUserPolicy(r.Context(), id, allowedSurface, req.BoundAccountID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to update user policy")
		return
	}
	slog.Info("user policy updated", "id", id, "allowedSurface", allowedSurface, "boundAccountId", req.BoundAccountID)
	writeJSON(w, http.StatusOK, struct {
		ID                string         `json:"id"`
		AllowedSurface    domain.Surface `json:"allowed_surface"`
		BoundAccountID    string         `json:"bound_account_id,omitempty"`
		BoundAccountEmail string         `json:"bound_account_email,omitempty"`
	}{
		ID:                id,
		AllowedSurface:    allowedSurface,
		BoundAccountID:    req.BoundAccountID,
		BoundAccountEmail: s.boundAccountEmail(req.BoundAccountID),
	})
}

func parseUserAllowedSurface(raw string) (domain.Surface, error) {
	if raw == "" {
		return domain.SurfaceNative, nil
	}
	allowedSurface := domain.NormalizeSurface(raw)
	switch allowedSurface {
	case domain.SurfaceNative, domain.SurfaceCompat, domain.SurfaceAll:
		return allowedSurface, nil
	default:
		return "", fmt.Errorf("allowed_surface must be 'native', 'compat', or 'all'")
	}
}

func (s *Server) validateUserBoundAccount(accountID string) error {
	if accountID == "" {
		return nil
	}
	if s.pool.Get(accountID) == nil {
		return fmt.Errorf("bound_account_id not found")
	}
	return nil
}

func generateUserToken(name string) (plaintext, hashStr, prefix string, err error) {
	b := make([]byte, 8)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	hexStr := hex.EncodeToString(b)
	plaintext = fmt.Sprintf("tk_%s_%s", name, hexStr)
	h := sha256.Sum256([]byte(plaintext))
	hashStr = hex.EncodeToString(h[:])
	prefix = fmt.Sprintf("tk_%s_%s...", name, hexStr[:4])
	return plaintext, hashStr, prefix, nil
}

// ---------------------------------------------------------------------------
// User detail
// ---------------------------------------------------------------------------

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	users, err := s.store.ListUsers(ctx)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}
	var user *domain.User
	for _, u := range users {
		if u.ID == id {
			user = u
			break
		}
	}
	if user == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	loc := parseTZParam(r)
	usage, err := s.store.QueryUsagePeriods(ctx, id, loc)
	if err != nil {
		slog.Warn("user detail: query usage periods failed", "error", err, "userId", id)
	}
	modelUsage, err := s.store.QueryModelUsage(ctx, id)
	if err != nil {
		slog.Warn("user detail: query model usage failed", "error", err, "userId", id)
	}
	recentRequests, _, err := s.store.QueryRequestLogs(ctx, domain.RequestLogQuery{
		UserID: id,
		Limit:  20,
	})
	if err != nil {
		slog.Warn("user detail: query request logs failed", "error", err, "userId", id)
	}

	if usage == nil {
		usage = []domain.UsagePeriod{}
	}
	if modelUsage == nil {
		modelUsage = []domain.ModelUsageRow{}
	}
	if recentRequests == nil {
		recentRequests = []*domain.RequestLog{}
	}

	writeJSON(w, http.StatusOK, UserDetailResponse{
		ID:                user.ID,
		Name:              user.Name,
		TokenPrefix:       user.TokenPrefix,
		Status:            user.Status,
		AllowedSurface:    user.AllowedSurface,
		BoundAccountID:    user.BoundAccountID,
		BoundAccountEmail: s.boundAccountEmail(user.BoundAccountID),
		CreatedAt:         user.CreatedAt,
		LastActiveAt:      user.LastActiveAt,
		Usage:             usage,
		ModelUsage:        modelUsage,
		RecentRequests:    recentRequests,
	})
}
