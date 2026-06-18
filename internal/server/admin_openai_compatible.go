package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

type openAICompatibleAccountRequest struct {
	Name    string   `json:"name"`
	BaseURL string   `json:"base_url"`
	APIKey  string   `json:"api_key"`
	Models  []string `json:"models"`
	Status  string   `json:"status"`
	Weight  *int     `json:"weight"`
}

func (s *Server) handleCreateOpenAICompatibleAccount(w http.ResponseWriter, r *http.Request) {
	var req openAICompatibleAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	spec, err := s.validateOpenAICompatibleAccountRequest(r.Context(), req, true)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	subject := openAICompatibleSubject(spec.baseURL, spec.keyFingerprint)
	if existing := s.pool.FindBySubject(domain.ProviderOpenAICompatible, subject); existing != nil {
		writeAdminError(w, http.StatusConflict, "conflict", "openai-compatible account already exists")
		return
	}
	if s.tokens == nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "token manager unavailable")
		return
	}
	encKey, err := s.tokens.EncryptToken(spec.apiKey)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to encrypt api key")
		return
	}
	now := time.Now().UTC()
	acct := &domain.Account{
		ID:             uuid.NewString(),
		Email:          spec.name,
		Provider:       domain.ProviderOpenAICompatible,
		Status:         spec.status,
		Priority:       spec.weight,
		PriorityMode:   "manual",
		AccessTokenEnc: encKey,
		ExpiresAt:      0,
		CreatedAt:      now,
		Identity:       openAICompatibleIdentityMap(spec),
		Subject:        subject,
	}
	acct.BucketKey = driver.NewOpenAICompatibleDriver(driver.ErrorPauses{}).BucketKey(acct)
	if err := s.pool.Add(acct); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to persist account")
		return
	}
	slog.Info("openai-compatible account created", "id", acct.ID, "baseURL", spec.baseURL, "models", strings.Join(spec.models, ","))
	writeJSON(w, http.StatusOK, openAICompatibleAccountResponse(acct))
}

func (s *Server) handleUpdateOpenAICompatibleAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acct := s.pool.Get(id)
	if acct == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "account not found")
		return
	}
	if acct.Provider != domain.ProviderOpenAICompatible {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "account is not openai_compatible")
		return
	}
	var req openAICompatibleAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	keyProvided := strings.TrimSpace(req.APIKey) != ""
	current := openAICompatibleSpecFromAccount(acct)
	if strings.TrimSpace(req.Name) == "" {
		req.Name = current.name
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		req.BaseURL = current.baseURL
	}
	if len(req.Models) == 0 {
		req.Models = current.models
	}
	if req.Status == "" {
		req.Status = string(current.status)
	}
	if req.Weight == nil {
		req.Weight = &current.weight
	}
	if !keyProvided {
		req.APIKey = current.apiKey
	}
	spec, err := s.validateOpenAICompatibleAccountRequest(r.Context(), req, false)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if !keyProvided {
		spec.keyFingerprint = current.keyFingerprint
	}
	subject := openAICompatibleSubject(spec.baseURL, spec.keyFingerprint)
	if existing := s.pool.FindBySubject(domain.ProviderOpenAICompatible, subject); existing != nil && existing.ID != id {
		writeAdminError(w, http.StatusConflict, "conflict", "openai-compatible account already exists")
		return
	}
	var encKey string
	if keyProvided {
		if s.tokens == nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "token manager unavailable")
			return
		}
		encKey, err = s.tokens.EncryptToken(spec.apiKey)
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to encrypt api key")
			return
		}
	}
	if err := s.pool.Update(id, func(a *domain.Account) {
		a.Email = spec.name
		a.Status = spec.status
		a.Priority = spec.weight
		a.PriorityMode = "manual"
		if encKey != "" {
			a.AccessTokenEnc = encKey
		}
		a.Identity = openAICompatibleIdentityMap(spec)
		a.Subject = subject
	}); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to update account")
		return
	}
	updated := s.pool.Get(id)
	writeJSON(w, http.StatusOK, openAICompatibleAccountResponse(updated))
}

type openAICompatibleAccountSpec struct {
	name           string
	baseURL        string
	apiKey         string
	keyFingerprint string
	models         []string
	status         domain.Status
	weight         int
}

func (s *Server) validateOpenAICompatibleAccountRequest(ctx context.Context, req openAICompatibleAccountRequest, requireKey bool) (openAICompatibleAccountSpec, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		return openAICompatibleAccountSpec{}, fmt.Errorf("name must be 1-100 characters")
	}
	baseURL, err := driver.NormalizeOpenAICompatibleBaseURL(req.BaseURL)
	if err != nil {
		return openAICompatibleAccountSpec{}, err
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if requireKey && apiKey == "" {
		return openAICompatibleAccountSpec{}, fmt.Errorf("api_key is required")
	}
	models := compactOpenAICompatibleModels(req.Models)
	if len(models) == 0 {
		return openAICompatibleAccountSpec{}, fmt.Errorf("models must not be empty")
	}
	for _, model := range models {
		price, err := s.store.GetModelPrice(ctx, model)
		if err != nil {
			return openAICompatibleAccountSpec{}, err
		}
		if price == nil {
			return openAICompatibleAccountSpec{}, fmt.Errorf("model %q is not priced", model)
		}
	}
	status := domain.StatusActive
	switch req.Status {
	case "", string(domain.StatusActive):
		status = domain.StatusActive
	case string(domain.StatusDisabled):
		status = domain.StatusDisabled
	default:
		return openAICompatibleAccountSpec{}, fmt.Errorf("status must be active or disabled")
	}
	weight := 50
	if req.Weight != nil {
		weight = *req.Weight
	}
	if weight < 0 {
		return openAICompatibleAccountSpec{}, fmt.Errorf("weight must be non-negative")
	}
	return openAICompatibleAccountSpec{
		name:           name,
		baseURL:        baseURL,
		apiKey:         apiKey,
		keyFingerprint: openAICompatibleKeyFingerprint(apiKey),
		models:         models,
		status:         status,
		weight:         weight,
	}, nil
}

func openAICompatibleIdentityMap(spec openAICompatibleAccountSpec) map[string]string {
	modelsJSON, _ := json.Marshal(spec.models)
	return map[string]string{
		"name":                spec.name,
		"base_url":            spec.baseURL,
		"models":              string(modelsJSON),
		"api_key_fingerprint": spec.keyFingerprint,
	}
}

func openAICompatibleAccountResponse(acct *domain.Account) OpenAICompatibleAccountResponse {
	spec := openAICompatibleSpecFromAccount(acct)
	return OpenAICompatibleAccountResponse{
		ID:                acct.ID,
		Name:              spec.name,
		BaseURL:           spec.baseURL,
		Models:            spec.models,
		APIKeyFingerprint: spec.keyFingerprint,
		Status:            acct.Status,
		Weight:            acct.Priority,
		Subject:           acct.Subject,
	}
}

func openAICompatibleSpecFromAccount(acct *domain.Account) openAICompatibleAccountSpec {
	spec := openAICompatibleAccountSpec{status: domain.StatusActive, weight: 50}
	if acct == nil {
		return spec
	}
	if acct.Identity == nil {
		acct.HydrateRuntime()
	}
	spec.name = acct.Email
	spec.status = acct.Status
	spec.weight = acct.Priority
	if acct.Identity != nil {
		if name := strings.TrimSpace(acct.Identity["name"]); name != "" {
			spec.name = name
		}
		spec.baseURL = strings.TrimSpace(acct.Identity["base_url"])
		spec.models = parseOpenAICompatibleModels(acct.Identity["models"])
		spec.keyFingerprint = acct.Identity["api_key_fingerprint"]
	}
	return spec
}

func compactOpenAICompatibleModels(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func parseOpenAICompatibleModels(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var models []string
	if json.Unmarshal([]byte(raw), &models) == nil {
		return compactOpenAICompatibleModels(models)
	}
	return compactOpenAICompatibleModels(strings.Split(raw, ","))
}

func openAICompatibleKeyFingerprint(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])[:16]
}

func openAICompatibleSubject(baseURL, fingerprint string) string {
	sum := sha256.Sum256([]byte(baseURL + "\n" + fingerprint))
	return hex.EncodeToString(sum[:])
}
