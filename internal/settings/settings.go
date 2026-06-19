package settings

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/crypto"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/store"
)

const integrationSecretSaltPrefix = "integration:"

type RuntimeSpec struct {
	Key             string `json:"key"`
	Group           string `json:"group"`
	Label           string `json:"label"`
	Kind            string `json:"kind"`
	RestartRequired bool   `json:"restart_required"`
	Help            string `json:"help"`
	Default         any    `json:"default"`
}

type Service struct {
	store          store.Store
	crypto         *crypto.Crypto
	fingerprintKey string
}

func NewService(st store.Store, c *crypto.Crypto, fingerprintKey string) *Service {
	return &Service{store: st, crypto: c, fingerprintKey: fingerprintKey}
}

func RuntimeSpecs() []RuntimeSpec {
	return []RuntimeSpec{
		{Key: "site_url", Group: "general", Label: "站点 URL", Kind: "string", Default: "", Help: "没有请求 Host 可用时用于生成公开链接。"},
		{Key: "customer_session_ttl_ms", Group: "general", Label: "用户登录有效期", Kind: "duration_ms", Default: int64((30 * 24 * time.Hour) / time.Millisecond)},
		{Key: "codex_api_url", Group: "advanced", Label: "Codex upstream URL", Kind: "url", RestartRequired: true, Default: "https://chatgpt.com/backend-api/codex/responses"},
		{Key: "codex_request_timeout_ms", Group: "advanced", Label: "Codex 请求超时", Kind: "duration_ms", RestartRequired: true, Default: int64((10 * time.Minute) / time.Millisecond)},
		{Key: "request_timeout_ms", Group: "advanced", Label: "全局请求超时", Kind: "duration_ms", RestartRequired: true, Default: int64((5 * time.Minute) / time.Millisecond)},
		{Key: "graceful_shutdown_timeout_ms", Group: "advanced", Label: "优雅关闭等待", Kind: "duration_ms", RestartRequired: true, Default: int64((35 * time.Minute) / time.Millisecond)},
		{Key: "request_max_size_mb", Group: "advanced", Label: "最大请求体 MB", Kind: "int", RestartRequired: true, Default: 60},
		{Key: "max_retry_accounts", Group: "advanced", Label: "单请求最大重试账号数", Kind: "int", RestartRequired: true, Default: 2},
		{Key: "max_cache_controls", Group: "advanced", Label: "最大 cache_control 数", Kind: "int", RestartRequired: true, Default: 4},
		{Key: "session_binding_ttl_ms", Group: "advanced", Label: "会话绑定有效期", Kind: "duration_ms", RestartRequired: true, Default: int64((24 * time.Hour) / time.Millisecond)},
		{Key: "token_refresh_advance_ms", Group: "advanced", Label: "Token 提前刷新时间", Kind: "duration_ms", RestartRequired: true, Default: int64((60 * time.Second) / time.Millisecond)},
		{Key: "error_pause_401_ms", Group: "advanced", Label: "401 冷却", Kind: "duration_ms", RestartRequired: true, Default: int64((30 * time.Minute) / time.Millisecond)},
		{Key: "error_pause_401_refresh_ms", Group: "advanced", Label: "401 后台刷新冷却", Kind: "duration_ms", RestartRequired: true, Default: int64((30 * time.Second) / time.Millisecond)},
		{Key: "error_pause_403_ms", Group: "advanced", Label: "403 冷却", Kind: "duration_ms", RestartRequired: true, Default: int64((10 * time.Minute) / time.Millisecond)},
		{Key: "error_pause_429_ms", Group: "advanced", Label: "429 冷却", Kind: "duration_ms", RestartRequired: true, Default: int64((60 * time.Second) / time.Millisecond)},
		{Key: "error_pause_529_ms", Group: "advanced", Label: "529 冷却", Kind: "duration_ms", RestartRequired: true, Default: int64((5 * time.Minute) / time.Millisecond)},
		{Key: "cell_error_pause_ms", Group: "advanced", Label: "节点网络错误冷却", Kind: "duration_ms", RestartRequired: true, Default: int64((60 * time.Second) / time.Millisecond)},
		{Key: "compat_max_requests_per_minute", Group: "advanced", Label: "Compat 每分钟限制", Kind: "int", RestartRequired: true, Default: 0},
		{Key: "compat_max_concurrent", Group: "advanced", Label: "Compat 并发限制", Kind: "int", RestartRequired: true, Default: 4},
		{Key: "log_level", Group: "advanced", Label: "日志等级", Kind: "enum:debug,info,warn,error", RestartRequired: true, Default: "info"},
		{Key: "log_retention_days", Group: "advanced", Label: "请求日志保留天数", Kind: "int", Default: 3},
		{Key: "trace_compat", Group: "advanced", Label: "Compat trace", Kind: "bool", RestartRequired: true, Default: false},
	}
}

func specByKey(key string) (RuntimeSpec, bool) {
	for _, spec := range RuntimeSpecs() {
		if spec.Key == key {
			return spec, true
		}
	}
	return RuntimeSpec{}, false
}

func (s *Service) SeedFromConfig(ctx context.Context, cfg *config.Config) error {
	if s == nil || s.store == nil || cfg == nil {
		return nil
	}
	values := map[string]any{
		"site_url":                       cfg.SiteURL,
		"customer_session_ttl_ms":        int64(cfg.SessionTTL / time.Millisecond),
		"codex_api_url":                  cfg.CodexAPIURL,
		"codex_request_timeout_ms":       int64(cfg.CodexRequestTimeout / time.Millisecond),
		"request_timeout_ms":             int64(cfg.RequestTimeout / time.Millisecond),
		"graceful_shutdown_timeout_ms":   int64(cfg.GracefulShutdownTimeout / time.Millisecond),
		"request_max_size_mb":            cfg.MaxRequestBodyMB,
		"max_retry_accounts":             cfg.MaxRetryAccounts,
		"max_cache_controls":             cfg.MaxCacheControls,
		"session_binding_ttl_ms":         int64(cfg.SessionBindingTTL / time.Millisecond),
		"token_refresh_advance_ms":       int64(cfg.TokenRefreshAdvance / time.Millisecond),
		"error_pause_401_ms":             int64(cfg.ErrorPause401 / time.Millisecond),
		"error_pause_401_refresh_ms":     int64(cfg.ErrorPause401Refresh / time.Millisecond),
		"error_pause_403_ms":             int64(cfg.ErrorPause403 / time.Millisecond),
		"error_pause_429_ms":             int64(cfg.ErrorPause429 / time.Millisecond),
		"error_pause_529_ms":             int64(cfg.ErrorPause529 / time.Millisecond),
		"cell_error_pause_ms":            int64(cfg.CellErrorPause / time.Millisecond),
		"compat_max_requests_per_minute": cfg.CompatMaxRequestsPerMinute,
		"compat_max_concurrent":          cfg.CompatMaxConcurrent,
		"log_level":                      cfg.LogLevel,
		"log_retention_days":             cfg.LogRetentionDays,
		"trace_compat":                   cfg.TraceCompat,
	}
	now := time.Now().UTC()
	for key, value := range values {
		if existing, err := s.store.GetRuntimeSetting(ctx, key); err != nil {
			return err
		} else if existing != nil {
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err := s.store.UpsertRuntimeSetting(ctx, &domain.RuntimeSetting{
			Key:       key,
			ValueJSON: string(raw),
			UpdatedAt: now,
			UpdatedBy: "env-seed",
		}); err != nil {
			return err
		}
	}
	if err := s.seedZPay(ctx, cfg, now); err != nil {
		return err
	}
	if err := s.seedSMTP(ctx, cfg, now); err != nil {
		return err
	}
	return s.seedTurnstile(ctx, cfg, now)
}

func (s *Service) ApplyToConfig(ctx context.Context, cfg *config.Config) error {
	if s == nil || cfg == nil {
		return nil
	}
	var err error
	if cfg.SiteURL, err = s.GetString(ctx, "site_url", cfg.SiteURL); err != nil {
		return err
	}
	if cfg.SessionTTL, err = s.GetDuration(ctx, "customer_session_ttl_ms", cfg.SessionTTL); err != nil {
		return err
	}
	if cfg.CodexAPIURL, err = s.GetString(ctx, "codex_api_url", cfg.CodexAPIURL); err != nil {
		return err
	}
	if cfg.CodexRequestTimeout, err = s.GetDuration(ctx, "codex_request_timeout_ms", cfg.CodexRequestTimeout); err != nil {
		return err
	}
	if cfg.RequestTimeout, err = s.GetDuration(ctx, "request_timeout_ms", cfg.RequestTimeout); err != nil {
		return err
	}
	if cfg.GracefulShutdownTimeout, err = s.GetDuration(ctx, "graceful_shutdown_timeout_ms", cfg.GracefulShutdownTimeout); err != nil {
		return err
	}
	if cfg.MaxRequestBodyMB, err = s.GetInt(ctx, "request_max_size_mb", cfg.MaxRequestBodyMB); err != nil {
		return err
	}
	if cfg.MaxRetryAccounts, err = s.GetInt(ctx, "max_retry_accounts", cfg.MaxRetryAccounts); err != nil {
		return err
	}
	if cfg.MaxCacheControls, err = s.GetInt(ctx, "max_cache_controls", cfg.MaxCacheControls); err != nil {
		return err
	}
	if cfg.SessionBindingTTL, err = s.GetDuration(ctx, "session_binding_ttl_ms", cfg.SessionBindingTTL); err != nil {
		return err
	}
	if cfg.TokenRefreshAdvance, err = s.GetDuration(ctx, "token_refresh_advance_ms", cfg.TokenRefreshAdvance); err != nil {
		return err
	}
	if cfg.ErrorPause401, err = s.GetDuration(ctx, "error_pause_401_ms", cfg.ErrorPause401); err != nil {
		return err
	}
	if cfg.ErrorPause401Refresh, err = s.GetDuration(ctx, "error_pause_401_refresh_ms", cfg.ErrorPause401Refresh); err != nil {
		return err
	}
	if cfg.ErrorPause403, err = s.GetDuration(ctx, "error_pause_403_ms", cfg.ErrorPause403); err != nil {
		return err
	}
	if cfg.ErrorPause429, err = s.GetDuration(ctx, "error_pause_429_ms", cfg.ErrorPause429); err != nil {
		return err
	}
	if cfg.ErrorPause529, err = s.GetDuration(ctx, "error_pause_529_ms", cfg.ErrorPause529); err != nil {
		return err
	}
	if cfg.CellErrorPause, err = s.GetDuration(ctx, "cell_error_pause_ms", cfg.CellErrorPause); err != nil {
		return err
	}
	if cfg.CompatMaxRequestsPerMinute, err = s.GetInt(ctx, "compat_max_requests_per_minute", cfg.CompatMaxRequestsPerMinute); err != nil {
		return err
	}
	if cfg.CompatMaxConcurrent, err = s.GetInt(ctx, "compat_max_concurrent", cfg.CompatMaxConcurrent); err != nil {
		return err
	}
	if cfg.LogLevel, err = s.GetString(ctx, "log_level", cfg.LogLevel); err != nil {
		return err
	}
	if cfg.LogRetentionDays, err = s.GetInt(ctx, "log_retention_days", cfg.LogRetentionDays); err != nil {
		return err
	}
	if cfg.TraceCompat, err = s.GetBool(ctx, "trace_compat", cfg.TraceCompat); err != nil {
		return err
	}
	return nil
}

func (s *Service) UpsertRuntimeValue(ctx context.Context, key string, value any, actor string) (*domain.RuntimeSetting, error) {
	spec, ok := specByKey(key)
	if !ok {
		return nil, fmt.Errorf("unknown setting %q", key)
	}
	normalized, err := normalizeValue(spec, value)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	setting := &domain.RuntimeSetting{
		Key:       key,
		ValueJSON: string(raw),
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: actor,
	}
	if err := s.store.UpsertRuntimeSetting(ctx, setting); err != nil {
		return nil, err
	}
	return setting, nil
}

func normalizeValue(spec RuntimeSpec, value any) (any, error) {
	switch {
	case spec.Kind == "string":
		return strings.TrimSpace(fmt.Sprint(value)), nil
	case spec.Kind == "url":
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			return "", nil
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("%s must be http(s) URL", spec.Key)
		}
		return raw, nil
	case spec.Kind == "bool":
		v, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be boolean", spec.Key)
		}
		return v, nil
	case spec.Kind == "int" || spec.Kind == "duration_ms":
		v, err := numberToInt64(value)
		if err != nil {
			return nil, fmt.Errorf("%s must be integer", spec.Key)
		}
		if v < 0 {
			return nil, fmt.Errorf("%s must be >= 0", spec.Key)
		}
		return v, nil
	case strings.HasPrefix(spec.Kind, "enum:"):
		raw := strings.TrimSpace(fmt.Sprint(value))
		allowed := strings.Split(strings.TrimPrefix(spec.Kind, "enum:"), ",")
		for _, item := range allowed {
			if raw == item {
				return raw, nil
			}
		}
		return nil, fmt.Errorf("%s must be one of %s", spec.Key, strings.Join(allowed, ", "))
	default:
		return nil, fmt.Errorf("unsupported setting type %s", spec.Kind)
	}
}

func numberToInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		if x != float64(int64(x)) {
			return 0, fmt.Errorf("not integer")
		}
		return int64(x), nil
	case json.Number:
		return x.Int64()
	default:
		return 0, fmt.Errorf("not number")
	}
}

func (s *Service) GetString(ctx context.Context, key, fallback string) (string, error) {
	var out string
	ok, err := s.getValue(ctx, key, &out)
	if err != nil || !ok {
		return fallback, err
	}
	return out, nil
}

func (s *Service) GetBool(ctx context.Context, key string, fallback bool) (bool, error) {
	var out bool
	ok, err := s.getValue(ctx, key, &out)
	if err != nil || !ok {
		return fallback, err
	}
	return out, nil
}

func (s *Service) GetInt(ctx context.Context, key string, fallback int) (int, error) {
	var out int64
	ok, err := s.getValue(ctx, key, &out)
	if err != nil || !ok {
		return fallback, err
	}
	return int(out), nil
}

func (s *Service) GetDuration(ctx context.Context, key string, fallback time.Duration) (time.Duration, error) {
	var out int64
	ok, err := s.getValue(ctx, key, &out)
	if err != nil || !ok {
		return fallback, err
	}
	return time.Duration(out) * time.Millisecond, nil
}

func (s *Service) getValue(ctx context.Context, key string, out any) (bool, error) {
	if s == nil || s.store == nil {
		return false, nil
	}
	setting, err := s.store.GetRuntimeSetting(ctx, key)
	if err != nil || setting == nil || setting.ValueJSON == "" {
		return false, err
	}
	dec := json.NewDecoder(strings.NewReader(setting.ValueJSON))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) EncryptSecrets(integrationID string, secrets map[string]string) (string, string, error) {
	canonical, err := canonicalSecretJSON(secrets)
	if err != nil || canonical == "{}" {
		return "", "", err
	}
	enc, err := s.crypto.Encrypt(canonical, integrationSecretSaltPrefix+integrationID)
	if err != nil {
		return "", "", err
	}
	return enc, s.fingerprint(canonical), nil
}

func (s *Service) DecryptSecrets(integration *domain.Integration) (map[string]string, error) {
	out := map[string]string{}
	if integration == nil || strings.TrimSpace(integration.SecretJSONEnc) == "" {
		return out, nil
	}
	raw, err := s.crypto.Decrypt(integration.SecretJSONEnc, integrationSecretSaltPrefix+integration.ID)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func canonicalSecretJSON(secrets map[string]string) (string, error) {
	if len(secrets) == 0 {
		return "{}", nil
	}
	clean := make(map[string]string, len(secrets))
	for k, v := range secrets {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		clean[k] = v
	}
	keys := make([]string, 0, len(clean))
	for key := range clean {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(keys))
	for _, key := range keys {
		ordered[key] = clean[key]
	}
	raw, err := json.Marshal(ordered)
	return string(raw), err
}

func (s *Service) fingerprint(raw string) string {
	mac := hmac.New(sha256.New, []byte(s.fingerprintKey))
	mac.Write([]byte(raw))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum)[:12]
}

func (s *Service) SaveIntegrationWithSecrets(ctx context.Context, integration *domain.Integration, secrets map[string]string) error {
	if integration.ID == "" {
		integration.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if integration.CreatedAt.IsZero() {
		integration.CreatedAt = now
	}
	integration.UpdatedAt = now
	if integration.ConfigJSON == "" {
		integration.ConfigJSON = "{}"
	}
	if secrets != nil {
		enc, fp, err := s.EncryptSecrets(integration.ID, secrets)
		if err != nil {
			return err
		}
		integration.SecretJSONEnc = enc
		integration.SecretFingerprint = fp
	}
	return s.store.SaveIntegration(ctx, integration)
}

func (s *Service) seedZPay(ctx context.Context, cfg *config.Config, now time.Time) error {
	if strings.TrimSpace(cfg.ZPayPID) == "" && strings.TrimSpace(cfg.ZPayKey) == "" {
		return nil
	}
	existing, err := s.store.ListIntegrations(ctx, "payment")
	if err != nil {
		return err
	}
	for _, integration := range existing {
		if integration.Provider == "zpay" {
			return nil
		}
	}
	configJSON, _ := json.Marshal(map[string]any{"pid": cfg.ZPayPID, "cid": cfg.ZPayCID})
	integration := &domain.Integration{
		ID:          "payment_zpay_default",
		Kind:        "payment",
		Provider:    "zpay",
		DisplayName: "7pay / ZPay",
		Enabled:     strings.TrimSpace(cfg.ZPayPID) != "" && strings.TrimSpace(cfg.ZPayKey) != "",
		Priority:    100,
		ConfigJSON:  string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   "env-seed",
	}
	return s.SaveIntegrationWithSecrets(ctx, integration, map[string]string{"key": cfg.ZPayKey})
}

func (s *Service) seedSMTP(ctx context.Context, cfg *config.Config, now time.Time) error {
	if strings.TrimSpace(cfg.SMTPAddr) == "" && strings.TrimSpace(cfg.SMTPFrom) == "" {
		return nil
	}
	existing, err := s.store.ListIntegrations(ctx, "email")
	if err != nil {
		return err
	}
	for _, integration := range existing {
		if integration.Provider == "smtp" {
			return nil
		}
	}
	configJSON, _ := json.Marshal(map[string]any{
		"addr":     cfg.SMTPAddr,
		"username": cfg.SMTPUsername,
		"from":     cfg.SMTPFrom,
	})
	integration := &domain.Integration{
		ID:          "email_smtp_default",
		Kind:        "email",
		Provider:    "smtp",
		DisplayName: "SMTP",
		Enabled:     strings.TrimSpace(cfg.SMTPAddr) != "" && strings.TrimSpace(cfg.SMTPFrom) != "",
		Priority:    100,
		ConfigJSON:  string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   "env-seed",
	}
	return s.SaveIntegrationWithSecrets(ctx, integration, map[string]string{"password": cfg.SMTPPassword})
}

func (s *Service) seedTurnstile(ctx context.Context, cfg *config.Config, now time.Time) error {
	if !cfg.TurnstileEnabled && strings.TrimSpace(cfg.TurnstileSiteKey) == "" && strings.TrimSpace(cfg.TurnstileSecretKey) == "" {
		return nil
	}
	existing, err := s.store.ListIntegrations(ctx, "security")
	if err != nil {
		return err
	}
	for _, integration := range existing {
		if integration.Provider == "turnstile" {
			return nil
		}
	}
	configJSON, _ := json.Marshal(map[string]any{"site_key": cfg.TurnstileSiteKey})
	integration := &domain.Integration{
		ID:          "security_turnstile_default",
		Kind:        "security",
		Provider:    "turnstile",
		DisplayName: "Cloudflare Turnstile",
		Enabled:     cfg.TurnstileEnabled,
		Priority:    100,
		ConfigJSON:  string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   "env-seed",
	}
	return s.SaveIntegrationWithSecrets(ctx, integration, map[string]string{"secret_key": cfg.TurnstileSecretKey})
}
