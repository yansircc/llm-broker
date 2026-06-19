package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		view, err := s.apiKeyViewWithUsage(r.Context(), key, "")
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load api key usage")
			return
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCustomerCreateKey(w http.ResponseWriter, r *http.Request) {
	cc, ok := s.requireCustomer(w, r)
	if !ok {
		return
	}
	var req struct {
		Name             string  `json:"name"`
		DailyBudgetUSD   float64 `json:"daily_budget_usd"`
		MonthlyBudgetUSD float64 `json:"monthly_budget_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if req.DailyBudgetUSD < 0 || req.MonthlyBudgetUSD < 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "budget must be >= 0")
		return
	}
	name := normalizeAPIKeyName(req.Name)
	if name == "" {
		name = "default"
	}
	keys, err := s.store.ListAPIKeysByUser(r.Context(), cc.User.ID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to list api keys")
		return
	}
	name = uniqueAPIKeyName(name, keys)
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
		ID:                  uuid.NewString(),
		UserID:              cc.User.ID,
		Name:                name,
		TokenHash:           sha256Hex(token),
		TokenPrefix:         prefix,
		Status:              "active",
		AllowedSurface:      domain.SurfaceAll,
		DailyBudgetMicros:   usdToMicros(req.DailyBudgetUSD),
		MonthlyBudgetMicros: usdToMicros(req.MonthlyBudgetUSD),
		CreatedAt:           time.Now().UTC(),
	}
	if err := s.store.CreateAPIKey(r.Context(), key); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to create api key")
		return
	}
	view, err := s.apiKeyViewWithUsage(r.Context(), key, token)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load api key usage")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func normalizeAPIKeyName(name string) string {
	return strings.TrimSpace(name)
}

func uniqueAPIKeyName(base string, keys []*domain.APIKey, excludeIDs ...string) string {
	exclude := make(map[string]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		exclude[id] = struct{}{}
	}
	used := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := exclude[key.ID]; ok {
			continue
		}
		used[strings.ToLower(normalizeAPIKeyName(key.Name))] = struct{}{}
	}
	if _, ok := used[strings.ToLower(base)]; !ok {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if _, ok := used[strings.ToLower(candidate)]; !ok {
			return candidate
		}
	}
}

func (s *Server) handleCustomerUpdateKey(w http.ResponseWriter, r *http.Request) {
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
	var key *domain.APIKey
	for _, item := range keys {
		if item.ID == keyID {
			copy := *item
			key = &copy
			break
		}
	}
	if key == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "api key not found")
		return
	}
	var req struct {
		Name             string   `json:"name"`
		Status           string   `json:"status"`
		DailyBudgetUSD   *float64 `json:"daily_budget_usd"`
		MonthlyBudgetUSD *float64 `json:"monthly_budget_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	name := normalizeAPIKeyName(req.Name)
	if name == "" {
		name = key.Name
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	if status == "" {
		status = key.Status
	}
	if status != "active" && status != "disabled" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "status must be active or disabled")
		return
	}
	if (req.DailyBudgetUSD != nil && *req.DailyBudgetUSD < 0) || (req.MonthlyBudgetUSD != nil && *req.MonthlyBudgetUSD < 0) {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "budget must be >= 0")
		return
	}
	key.Name = uniqueAPIKeyName(name, keys, key.ID)
	key.Status = status
	if req.DailyBudgetUSD != nil {
		key.DailyBudgetMicros = usdToMicros(*req.DailyBudgetUSD)
	}
	if req.MonthlyBudgetUSD != nil {
		key.MonthlyBudgetMicros = usdToMicros(*req.MonthlyBudgetUSD)
	}
	if key.AllowedSurface == "" {
		key.AllowedSurface = domain.SurfaceAll
	}
	if err := s.store.UpdateAPIKey(r.Context(), key); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to update api key")
		return
	}
	view, err := s.apiKeyViewWithUsage(r.Context(), key, "")
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load api key usage")
		return
	}
	writeJSON(w, http.StatusOK, view)
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
		"id":                 key.ID,
		"name":               key.Name,
		"prefix":             key.TokenPrefix,
		"token_prefix":       key.TokenPrefix,
		"status":             key.Status,
		"daily_budget_usd":   microsToUSD(key.DailyBudgetMicros),
		"monthly_budget_usd": microsToUSD(key.MonthlyBudgetMicros),
		"created_at":         key.CreatedAt,
		"last_used_at":       key.LastUsedAt,
	}
	if token != "" {
		out["token"] = token
	}
	return out
}

func (s *Server) apiKeyViewWithUsage(ctx context.Context, key *domain.APIKey, token string) (map[string]any, error) {
	view := apiKeyView(key, token)
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	dailyUsage, err := s.store.SumAPIKeyUsageMicros(ctx, key.ID, dayStart, now)
	if err != nil {
		return nil, err
	}
	monthlyUsage, err := s.store.SumAPIKeyUsageMicros(ctx, key.ID, monthStart, now)
	if err != nil {
		return nil, err
	}
	view["daily_usage_usd"] = microsToUSD(dailyUsage)
	view["monthly_usage_usd"] = microsToUSD(monthlyUsage)
	view["daily_remaining_usd"] = budgetRemainingUSD(key.DailyBudgetMicros, dailyUsage)
	view["monthly_remaining_usd"] = budgetRemainingUSD(key.MonthlyBudgetMicros, monthlyUsage)
	return view, nil
}

func budgetRemainingUSD(budgetMicros, usedMicros int64) *float64 {
	if budgetMicros <= 0 {
		return nil
	}
	remaining := microsToUSD(budgetMicros - usedMicros)
	return &remaining
}
