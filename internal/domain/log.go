package domain

import (
	"encoding/json"
	"time"
)

// RequestLog represents a single API request log entry.
type RequestLog struct {
	ID                          int64           `json:"id"`
	UserID                      string          `json:"user_id"`
	AccountID                   string          `json:"account_id"`
	Provider                    string          `json:"provider"`
	Surface                     string          `json:"surface"`
	Model                       string          `json:"model"`
	Path                        string          `json:"path"`
	CellID                      string          `json:"cell_id,omitempty"`
	BucketKey                   string          `json:"bucket_key,omitempty"`
	SessionUUID                 string          `json:"session_uuid,omitempty"`
	BindingSource               string          `json:"binding_source,omitempty"`
	ClientHeaders               json.RawMessage `json:"client_headers,omitempty"`
	ClientBodyExcerpt           string          `json:"client_body_excerpt,omitempty"`
	RequestMeta                 json.RawMessage `json:"request_meta,omitempty"`
	InputTokens                 int             `json:"input_tokens"`
	OutputTokens                int             `json:"output_tokens"`
	CacheReadTokens             int             `json:"cache_read_tokens"`
	CacheCreateTokens           int             `json:"cache_create_tokens"`
	CostUSD                     float64         `json:"cost_usd"`
	Status                      string          `json:"status"`
	EffectKind                  string          `json:"effect_kind,omitempty"`
	UpstreamStatus              int             `json:"upstream_status,omitempty"`
	UpstreamURL                 string          `json:"upstream_url,omitempty"`
	UpstreamRequestHeaders      json.RawMessage `json:"upstream_request_headers,omitempty"`
	UpstreamRequestMeta         json.RawMessage `json:"upstream_request_meta,omitempty"`
	UpstreamRequestBodyExcerpt  string          `json:"upstream_request_body_excerpt,omitempty"`
	UpstreamRequestID           string          `json:"upstream_request_id,omitempty"`
	UpstreamHeaders             json.RawMessage `json:"upstream_headers,omitempty"`
	UpstreamResponseMeta        json.RawMessage `json:"upstream_response_meta,omitempty"`
	UpstreamResponseBodyExcerpt string          `json:"upstream_response_body_excerpt,omitempty"`
	UpstreamErrorType           string          `json:"upstream_error_type,omitempty"`
	UpstreamErrorMessage        string          `json:"upstream_error_message,omitempty"`
	RequestBytes                int             `json:"request_bytes"`
	AttemptCount                int             `json:"attempt_count"`
	DurationMs                  int64           `json:"duration_ms"`
	CreatedAt                   time.Time       `json:"created_at"`
}

// RequestLogQuery is a paginated request log query.
type RequestLogQuery struct {
	UserID       string
	AccountID    string
	FailuresOnly bool
	Limit        int
	Offset       int
}

type RelayOutcomeStat struct {
	Provider         string    `json:"provider"`
	Surface          string    `json:"surface"`
	EffectKind       string    `json:"effect_kind"`
	UpstreamStatus   int       `json:"upstream_status,omitempty"`
	Requests         int       `json:"requests"`
	DistinctUsers    int       `json:"distinct_users"`
	DistinctAccounts int       `json:"distinct_accounts"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

type CellRiskStat struct {
	CellID           string    `json:"cell_id,omitempty"`
	Provider         string    `json:"provider"`
	Requests         int       `json:"requests"`
	Successes        int       `json:"successes"`
	Status400        int       `json:"status_400"`
	Status403        int       `json:"status_403"`
	Status429        int       `json:"status_429"`
	Blocks           int       `json:"blocks"`
	TransportErrors  int       `json:"transport_errors"`
	DistinctUsers    int       `json:"distinct_users"`
	DistinctAccounts int       `json:"distinct_accounts"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

// UsagePeriod represents usage for a named period (today, yesterday, 3d, 7d, 30d).
type UsagePeriod struct {
	Label           string  `json:"label"`
	Requests        int     `json:"requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	CostUSD         float64 `json:"cost_usd"`
}

// ModelUsageRow represents per-model usage breakdown.
type ModelUsageRow struct {
	Model           string  `json:"model"`
	Requests        int     `json:"requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	CostUSD         float64 `json:"cost_usd"`
}

// SessionBindingInfo describes an active session binding.
type SessionBindingInfo struct {
	SessionUUID string    `json:"session_uuid"`
	AccountID   string    `json:"account_id"`
	CreatedAt   string    `json:"created_at"`
	LastUsedAt  string    `json:"last_used_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}
