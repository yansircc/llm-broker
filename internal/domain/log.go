package domain

import "time"

// RequestLog represents a single API request log entry.
type RequestLog struct {
	ID                int64     `json:"id"`
	UserID            string    `json:"user_id"`
	AccountID         string    `json:"account_id"`
	Model             string    `json:"model"`
	InputTokens       int       `json:"input_tokens"`
	OutputTokens      int       `json:"output_tokens"`
	CacheReadTokens   int       `json:"cache_read_tokens"`
	CacheCreateTokens int       `json:"cache_create_tokens"`
	CostUSD           float64   `json:"cost_usd"`
	Status            string    `json:"status"`
	DurationMs        int64     `json:"duration_ms"`
	CreatedAt         time.Time `json:"created_at"`
}

// RequestLogQuery is a paginated request log query.
type RequestLogQuery struct {
	UserID    string
	AccountID string
	Limit     int
	Offset    int
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
