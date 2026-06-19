package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/email"
	runtimecfg "github.com/yansircc/llm-broker/internal/settings"
)

type RuntimeSettingView struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

type IntegrationView struct {
	ID                string         `json:"id"`
	Kind              string         `json:"kind"`
	Provider          string         `json:"provider"`
	DisplayName       string         `json:"display_name"`
	Enabled           bool           `json:"enabled"`
	Priority          int            `json:"priority"`
	Config            map[string]any `json:"config"`
	SecretConfigured  bool           `json:"secret_configured"`
	SecretFingerprint string         `json:"secret_fingerprint"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	UpdatedBy         string         `json:"updated_by"`
}

type SettingsResponse struct {
	RuntimeSpecs    []runtimecfg.RuntimeSpec      `json:"runtime_specs"`
	RuntimeSettings map[string]RuntimeSettingView `json:"runtime_settings"`
	BillingSettings map[string]string             `json:"billing_settings"`
	Integrations    []IntegrationView             `json:"integrations"`
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.ListRuntimeSettings(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load settings")
		return
	}
	out := SettingsResponse{
		RuntimeSpecs:    runtimecfg.RuntimeSpecs(),
		RuntimeSettings: make(map[string]RuntimeSettingView),
		BillingSettings: make(map[string]string),
	}
	for _, setting := range settings {
		out.RuntimeSettings[setting.Key] = runtimeSettingView(setting)
	}
	for _, key := range billingSettingKeys() {
		value, err := s.store.GetBillingSetting(r.Context(), key)
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load billing settings")
			return
		}
		out.BillingSettings[key] = value
	}
	integrations, err := s.store.ListIntegrations(r.Context(), "")
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load integrations")
		return
	}
	for _, integration := range integrations {
		out.Integrations = append(out.Integrations, integrationView(integration))
	}
	if out.Integrations == nil {
		out.Integrations = []IntegrationView{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings        map[string]any `json:"settings"`
		BillingSettings map[string]any `json:"billing_settings"`
	}
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	actor := adminActorID(r)
	now := time.Now().UTC()
	for key, value := range req.Settings {
		before, _ := s.store.GetRuntimeSetting(r.Context(), key)
		updated, err := s.settings.UpsertRuntimeValue(r.Context(), key, value, actor)
		if err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		s.auditSettingChange(r, "runtime_setting", key, "update", before, updated)
	}
	for key, value := range req.BillingSettings {
		if !validBillingSettingKey(key) {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "unknown billing setting "+key)
			return
		}
		before, _ := s.store.GetBillingSetting(r.Context(), key)
		normalized := strings.TrimSpace(fmt.Sprint(value))
		if _, ok := value.(json.Number); ok {
			normalized = value.(json.Number).String()
		}
		if normalized == "" {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", key+" is required")
			return
		}
		if err := s.store.UpsertBillingSetting(r.Context(), key, normalized, now); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to update billing setting")
			return
		}
		s.auditSettingChange(r, "billing_setting", key, "update", before, normalized)
	}
	s.handleAdminSettings(w, r)
}

func (s *Server) handleAdminCreateIntegration(w http.ResponseWriter, r *http.Request) {
	integration, secrets, err := s.decodeIntegrationRequest(r, nil)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if integration.DisplayName == "" {
		integration.DisplayName = integration.Provider
	}
	if err := s.settings.SaveIntegrationWithSecrets(r.Context(), integration, secrets); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to save integration")
		return
	}
	s.auditSettingChange(r, "integration", integration.ID, "create", nil, integrationView(integration))
	writeJSON(w, http.StatusOK, integrationView(integration))
}

func (s *Server) handleAdminUpdateIntegration(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetIntegration(r.Context(), id)
	if err != nil || existing == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "integration not found")
		return
	}
	before := integrationView(existing)
	integration, secrets, err := s.decodeIntegrationRequest(r, existing)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := s.settings.SaveIntegrationWithSecrets(r.Context(), integration, secrets); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to save integration")
		return
	}
	s.auditSettingChange(r, "integration", integration.ID, "update", before, integrationView(integration))
	writeJSON(w, http.StatusOK, integrationView(integration))
}

func (s *Server) handleAdminTestIntegration(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	integration, err := s.store.GetIntegration(r.Context(), id)
	if err != nil || integration == nil {
		writeAdminError(w, http.StatusNotFound, "not_found", "integration not found")
		return
	}
	var req struct {
		To string `json:"to"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	switch integration.Kind {
	case "email":
		if strings.TrimSpace(req.To) == "" {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "to is required")
			return
		}
		msg := email.Message{To: req.To, Subject: "CDX 邮件配置测试", Text: "这是一封 CDX 邮件集成测试消息。"}
		cfg, err := s.decryptedIntegrationConfig(r.Context(), integration)
		if err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", "failed to decrypt integration")
			return
		}
		sender, err := emailSenderFromIntegration(integration.Provider, cfg)
		if err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err := sender.Send(r.Context(), msg); err != nil {
			s.recordIntegrationEvent(r.Context(), integration, "test", false, "send_failed", nil)
			writeAdminError(w, http.StatusBadGateway, "integration_test_failed", err.Error())
			return
		}
		s.recordIntegrationEvent(r.Context(), integration, "test", true, "", nil)
		writeJSON(w, http.StatusOK, integrationTestResponse{OK: true})
	case "payment":
		if _, _, _, err := s.paymentConfigForIntegration(r.Context(), integration); err != nil {
			writeAdminError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		s.recordIntegrationEvent(r.Context(), integration, "test", true, "", nil)
		writeJSON(w, http.StatusOK, integrationTestResponse{OK: true})
	default:
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "integration kind cannot be tested")
	}
}

type integrationTestResponse struct {
	OK bool `json:"ok"`
}

func (s *Server) decodeIntegrationRequest(r *http.Request, existing *domain.Integration) (*domain.Integration, map[string]string, error) {
	var raw integrationRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON body")
	}
	integration := &domain.Integration{ID: uuid.NewString(), Priority: 100, ConfigJSON: "{}", UpdatedBy: adminActorID(r)}
	if existing != nil {
		copy := *existing
		integration = &copy
		integration.UpdatedBy = adminActorID(r)
	}
	if raw.Kind != nil {
		integration.Kind = strings.ToLower(strings.TrimSpace(*raw.Kind))
	}
	if raw.Provider != nil {
		integration.Provider = strings.ToLower(strings.TrimSpace(*raw.Provider))
	}
	if raw.DisplayName != nil {
		integration.DisplayName = strings.TrimSpace(*raw.DisplayName)
	}
	if raw.Enabled != nil {
		integration.Enabled = *raw.Enabled
	}
	if raw.Priority != nil {
		integration.Priority = *raw.Priority
	}
	if raw.Config != nil {
		normalized := make(map[string]any, len(raw.Config))
		for key, value := range raw.Config {
			normalized[strings.TrimSpace(key)] = value
		}
		configJSON, err := json.Marshal(normalized)
		if err != nil {
			return nil, nil, err
		}
		integration.ConfigJSON = string(configJSON)
	}
	if integration.Kind == "" || integration.Provider == "" {
		return nil, nil, fmt.Errorf("kind and provider are required")
	}
	switch integration.Kind {
	case "payment", "email", "security":
	default:
		return nil, nil, fmt.Errorf("kind must be payment, email, or security")
	}
	return integration, raw.Secrets, nil
}

type integrationRequest struct {
	Kind        *string           `json:"kind"`
	Provider    *string           `json:"provider"`
	DisplayName *string           `json:"display_name"`
	Enabled     *bool             `json:"enabled"`
	Priority    *int              `json:"priority"`
	Config      map[string]any    `json:"config"`
	Secrets     map[string]string `json:"secrets"`
}

func runtimeSettingView(setting *domain.RuntimeSetting) RuntimeSettingView {
	var value any
	dec := json.NewDecoder(strings.NewReader(setting.ValueJSON))
	dec.UseNumber()
	_ = dec.Decode(&value)
	return RuntimeSettingView{Key: setting.Key, Value: value, UpdatedAt: setting.UpdatedAt, UpdatedBy: setting.UpdatedBy}
}

func integrationView(integration *domain.Integration) IntegrationView {
	config := map[string]any{}
	if integration.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(integration.ConfigJSON), &config)
	}
	return IntegrationView{
		ID:                integration.ID,
		Kind:              integration.Kind,
		Provider:          integration.Provider,
		DisplayName:       integration.DisplayName,
		Enabled:           integration.Enabled,
		Priority:          integration.Priority,
		Config:            config,
		SecretConfigured:  integration.SecretJSONEnc != "",
		SecretFingerprint: integration.SecretFingerprint,
		CreatedAt:         integration.CreatedAt,
		UpdatedAt:         integration.UpdatedAt,
		UpdatedBy:         integration.UpdatedBy,
	}
}

func billingSettingKeys() []string {
	return []string{
		"cny_to_usd_rate_micros",
		"referral_new_user_bonus_micros",
		"referral_inviter_bonus_micros",
		"low_balance_alert_threshold_micros",
	}
}

func validBillingSettingKey(key string) bool {
	for _, allowed := range billingSettingKeys() {
		if key == allowed {
			return true
		}
	}
	return false
}

func adminActorID(r *http.Request) string {
	if ki := auth.GetKeyInfo(r.Context()); ki != nil {
		return ki.CustomerID
	}
	return ""
}

func (s *Server) auditSettingChange(r *http.Request, targetType, targetID, action string, before, after any) {
	beforeJSON, _ := json.Marshal(redactAuditValue(before))
	afterJSON, _ := json.Marshal(redactAuditValue(after))
	_ = s.store.SaveSettingsAudit(r.Context(), &domain.SettingsAudit{
		ID:          uuid.NewString(),
		ActorUserID: adminActorID(r),
		TargetType:  targetType,
		TargetID:    targetID,
		Action:      action,
		BeforeJSON:  string(beforeJSON),
		AfterJSON:   string(afterJSON),
		CreatedAt:   time.Now().UTC(),
	})
}

func redactAuditValue(v any) any {
	switch x := v.(type) {
	case *domain.Integration:
		return integrationView(x)
	case domain.Integration:
		return integrationView(&x)
	default:
		return v
	}
}
