package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yansircc/llm-broker/internal/requestlog"
)

type Config struct {
	// Server
	Host string
	Port int

	// Database
	DBPath string

	// Security
	EncryptionKey      string
	StaticToken        string
	AdminEmails        map[string]struct{}
	SiteURL            string
	SessionTTL         time.Duration
	TurnstileEnabled   bool
	TurnstileSiteKey   string
	TurnstileSecretKey string
	TrustedProxyCIDRs  []string

	// Email
	SMTPAddr     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// ZPay / 7pay
	ZPayPID string
	ZPayKey string
	ZPayCID string

	// Claude API
	ClaudeAPIURL     string
	ClaudeAPIVersion string
	ClaudeBetaHeader string

	// Codex API
	CodexAPIURL         string
	CodexRequestTimeout time.Duration

	// Gemini API / OAuth
	GeminiAPIURL            string
	GeminiOAuthClientID     string
	GeminiOAuthClientSecret string
	GeminiOAuthRedirectURI  string

	// Scheduling
	SessionBindingTTL   time.Duration
	TokenRefreshAdvance time.Duration

	// Error pause durations
	ErrorPause401        time.Duration
	ErrorPause401Refresh time.Duration // short cooldown for background token refresh on 401
	ErrorPause403        time.Duration
	ErrorPause429        time.Duration
	ErrorPause529        time.Duration
	CellErrorPause       time.Duration

	// Request
	RequestTimeout             time.Duration
	MaxRequestBodyMB           int
	MaxRetryAccounts           int
	MaxCacheControls           int
	CompatMaxRequestsPerMinute int // 0 disables the per-minute compat limiter
	CompatMaxConcurrent        int // 0 disables the compat concurrency limiter

	// Logging
	LogLevel         string
	LogBlobsMode     requestlog.BlobMode
	LogRetentionDays int
	TraceCompat      bool

	// Prompt environment masking (opt-in)
	PromptEnvHome string // canonical home path; enables prompt env masking when set

	// Runtime
	BackgroundJobsMode       string
	BackgroundLeaderLockPath string
	GracefulShutdownTimeout  time.Duration
}

func Load() *Config {
	return &Config{
		Host: envOr("HOST", "0.0.0.0"),
		Port: envInt("PORT", 3000),

		DBPath: envOr("DB_PATH", "./llm-broker.db"),

		EncryptionKey:      os.Getenv("ENCRYPTION_KEY"),
		StaticToken:        os.Getenv("API_TOKEN"),
		AdminEmails:        envSet("ADMIN_EMAILS"),
		SiteURL:            os.Getenv("SITE_URL"),
		SessionTTL:         envDuration("CUSTOMER_SESSION_TTL", 30*24*time.Hour),
		TurnstileEnabled:   envBool("TURNSTILE_ENABLED", false),
		TurnstileSiteKey:   os.Getenv("TURNSTILE_SITE_KEY"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		TrustedProxyCIDRs:  envList("TRUSTED_PROXY_CIDRS", []string{"127.0.0.1/32", "::1/128"}),

		SMTPAddr:     os.Getenv("SMTP_ADDR"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),

		ZPayPID: os.Getenv("ZPAY_PID"),
		ZPayKey: os.Getenv("ZPAY_KEY"),
		ZPayCID: os.Getenv("ZPAY_CID"),

		ClaudeAPIURL:     envOr("CLAUDE_API_URL", "https://api.anthropic.com/v1/messages"),
		ClaudeAPIVersion: envOr("CLAUDE_API_VERSION", "2023-06-01"),
		ClaudeBetaHeader: envOr("CLAUDE_BETA_HEADER", "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"),

		CodexAPIURL:         envOr("CODEX_API_URL", "https://chatgpt.com/backend-api/codex/responses"),
		CodexRequestTimeout: envDuration("CODEX_REQUEST_TIMEOUT", 10*time.Minute),

		GeminiAPIURL:            envOr("GEMINI_API_URL", "https://cloudcode-pa.googleapis.com"),
		GeminiOAuthClientID:     os.Getenv("GEMINI_OAUTH_CLIENT_ID"),
		GeminiOAuthClientSecret: os.Getenv("GEMINI_OAUTH_CLIENT_SECRET"),
		GeminiOAuthRedirectURI:  envOr("GEMINI_OAUTH_REDIRECT_URI", "https://codeassist.google.com/authcode"),

		SessionBindingTTL:   envDuration("SESSION_BINDING_TTL", 24*time.Hour),
		TokenRefreshAdvance: envDuration("TOKEN_REFRESH_ADVANCE", 60*time.Second),

		ErrorPause401:        envDuration("ERROR_PAUSE_401", 30*time.Minute),
		ErrorPause401Refresh: envDuration("ERROR_PAUSE_401_REFRESH", 30*time.Second),
		ErrorPause403:        envDuration("ERROR_PAUSE_403", 10*time.Minute),
		ErrorPause429:        envDuration("ERROR_PAUSE_429", 60*time.Second),
		ErrorPause529:        envDuration("ERROR_PAUSE_529", 5*time.Minute),
		CellErrorPause:       envDuration("CELL_ERROR_PAUSE", 60*time.Second),

		RequestTimeout:             envDuration("REQUEST_TIMEOUT", 5*time.Minute),
		MaxRequestBodyMB:           envInt("REQUEST_MAX_SIZE_MB", 60),
		MaxRetryAccounts:           envInt("MAX_RETRY_ACCOUNTS", 2),
		MaxCacheControls:           envInt("MAX_CACHE_CONTROLS", 4),
		CompatMaxRequestsPerMinute: envInt("COMPAT_MAX_REQUESTS_PER_MINUTE", 0),
		CompatMaxConcurrent:        envInt("COMPAT_MAX_CONCURRENT", 4),

		LogLevel:         envOr("LOG_LEVEL", "info"),
		LogBlobsMode:     requestlog.ParseBlobMode(os.Getenv("LOG_BLOBS")),
		LogRetentionDays: envInt("LOG_RETENTION_DAYS", 3),
		TraceCompat:      envBool("TRACE_COMPAT", false),

		PromptEnvHome: os.Getenv("PROMPT_ENV_HOME"),

		BackgroundJobsMode:       envOr("BACKGROUND_JOBS_MODE", "all"),
		BackgroundLeaderLockPath: envOr("BACKGROUND_LEADER_LOCK_PATH", "/var/run/llm-broker/background.lock"),
		GracefulShutdownTimeout:  envDuration("GRACEFUL_SHUTDOWN_TIMEOUT", 35*time.Minute),
	}
}

func (c *Config) Validate() error {
	if c.EncryptionKey == "" {
		return errMissing("ENCRYPTION_KEY")
	}
	if c.StaticToken == "" {
		return errMissing("API_TOKEN")
	}
	if c.TurnstileEnabled {
		if strings.TrimSpace(c.TurnstileSiteKey) == "" {
			return errMissing("TURNSTILE_SITE_KEY")
		}
		if strings.TrimSpace(c.TurnstileSecretKey) == "" {
			return errMissing("TURNSTILE_SECRET_KEY")
		}
	}
	if c.requiresPublicURL() {
		if strings.TrimSpace(c.SiteURL) == "" {
			return errMissing("SITE_URL")
		}
		if !validSiteURL(c.SiteURL) {
			return &configError{field: "SITE_URL (must include http(s) scheme and host)"}
		}
	}
	for _, cidr := range c.TrustedProxyCIDRs {
		if strings.TrimSpace(cidr) == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(strings.TrimSpace(cidr)); err != nil {
			return &configError{field: "TRUSTED_PROXY_CIDRS (must be comma-separated CIDR blocks)"}
		}
	}
	switch c.BackgroundJobsMode {
	case "all", "leader", "off":
	default:
		return &configError{field: "BACKGROUND_JOBS_MODE (must be all, leader, or off)"}
	}
	return nil
}

func (c *Config) requiresPublicURL() bool {
	return c.SMTPAddr != "" || c.SMTPFrom != "" || c.ZPayPID != "" || c.ZPayKey != ""
}

func validSiteURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func (c *Config) GeminiEnabled() bool {
	return c.GeminiOAuthClientID != "" && c.GeminiOAuthClientSecret != ""
}

type configError struct{ field string }

func (e *configError) Error() string { return "missing required env: " + e.field }
func errMissing(f string) error      { return &configError{field: f} }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
			return true
		case "0", "false", "FALSE", "False", "no", "NO", "off", "OFF":
			return false
		}
	}
	return fallback
}

func envSet(key string) map[string]struct{} {
	raw := os.Getenv(key)
	result := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		result[value] = struct{}{}
	}
	return result
}

func envList(key string, fallback []string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return append([]string(nil), fallback...)
	}
	var result []string
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func (c *Config) IsAdminEmail(email string) bool {
	if c == nil || len(c.AdminEmails) == 0 {
		return false
	}
	_, ok := c.AdminEmails[strings.ToLower(strings.TrimSpace(email))]
	return ok
}
