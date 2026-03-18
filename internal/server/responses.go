package server

import (
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

type DashboardResponse struct {
	Health         HealthInfo                 `json:"health"`
	Usage          []domain.UsagePeriod       `json:"usage"`
	Accounts       []DashboardAccount         `json:"accounts"`
	Users          []DashboardUser            `json:"users"`
	Events         []DashboardEvent           `json:"events"`
	OutcomeStats   []RelayOutcomeStatResponse `json:"outcome_stats"`
	CellRisk       []CellRiskResponse         `json:"cell_risk"`
	RecentFailures []*domain.RequestLog       `json:"recent_failures"`
}

type HealthInfo struct {
	SQLite  string `json:"sqlite"`
	Uptime  string `json:"uptime"`
	Version string `json:"version"`
}

type UtilizationWindowResponse struct {
	Label string `json:"label"`
	Pct   int    `json:"pct"`
	Reset int64  `json:"reset,omitempty"`
}

type AccountFieldResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type EgressCellSummaryResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	Labels        map[string]string `json:"labels,omitempty"`
	CooldownUntil *time.Time        `json:"cooldown_until,omitempty"`
	AccountCount  int               `json:"account_count,omitempty"`
}

type DashboardAccount struct {
	ID              string                      `json:"id"`
	Email           string                      `json:"email"`
	Provider        string                      `json:"provider"`
	Status          string                      `json:"status"`
	WeightMode      string                      `json:"weight_mode"`
	Weight          int                         `json:"weight"`
	CooldownUntil   *time.Time                  `json:"cooldown_until,omitempty"`
	LastUsedAt      *time.Time                  `json:"last_used_at,omitempty"`
	CellID          string                      `json:"cell_id,omitempty"`
	AvailableNative bool                        `json:"available_native"`
	AvailableCompat bool                        `json:"available_compat"`
	Windows         []UtilizationWindowResponse `json:"windows"`
}

type DashboardUser struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Status            string         `json:"status"`
	AllowedSurface    domain.Surface `json:"allowed_surface"`
	BoundAccountID    string         `json:"bound_account_id,omitempty"`
	BoundAccountEmail string         `json:"bound_account_email,omitempty"`
	LastActiveAt      *time.Time     `json:"last_active_at,omitempty"`
	TotalCost         float64        `json:"total_cost"`
}

type DashboardEvent struct {
	Type                 string     `json:"type"`
	AccountID            string     `json:"account_id,omitempty"`
	UserID               string     `json:"user_id,omitempty"`
	BucketKey            string     `json:"bucket_key,omitempty"`
	CellID               string     `json:"cell_id,omitempty"`
	CooldownUntil        *time.Time `json:"cooldown_until,omitempty"`
	UpstreamStatus       int        `json:"upstream_status,omitempty"`
	UpstreamErrorType    string     `json:"upstream_error_type,omitempty"`
	UpstreamErrorMessage string     `json:"upstream_error_message,omitempty"`
	Message              string     `json:"message"`
	Timestamp            string     `json:"ts"`
}

type RelayOutcomeStatResponse struct {
	Provider         string    `json:"provider"`
	Surface          string    `json:"surface"`
	EffectKind       string    `json:"effect_kind"`
	UpstreamStatus   int       `json:"upstream_status,omitempty"`
	Requests         int       `json:"requests"`
	DistinctUsers    int       `json:"distinct_users"`
	DistinctAccounts int       `json:"distinct_accounts"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

type CellRiskResponse struct {
	CellID           string    `json:"cell_id,omitempty"`
	CellName         string    `json:"cell_name"`
	Provider         string    `json:"provider"`
	Region           string    `json:"region"`
	Transport        string    `json:"transport"`
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

type ProviderOptionResponse struct {
	ID                  string `json:"id"`
	Label               string `json:"label"`
	CallbackPlaceholder string `json:"callback_placeholder"`
	CallbackHint        string `json:"callback_hint"`
}

// ---------------------------------------------------------------------------
// Account Detail
// ---------------------------------------------------------------------------

type AccountDetailResponse struct {
	ID             string                      `json:"id"`
	Email          string                      `json:"email"`
	Provider       domain.Provider             `json:"provider"`
	Subject        string                      `json:"subject"`
	Status         domain.Status               `json:"status"`
	ProbeLabel     string                      `json:"probe_label"`
	Weight         int                         `json:"weight"`
	WeightMode     string                      `json:"weight_mode"`
	AutoWeight     int                         `json:"auto_weight"`
	ErrorMessage   string                      `json:"error_message,omitempty"`
	ProviderFields []AccountFieldResponse      `json:"provider_fields"`
	CreatedAt      time.Time                   `json:"created_at"`
	LastUsedAt     *time.Time                  `json:"last_used_at,omitempty"`
	LastRefreshAt  *time.Time                  `json:"last_refresh_at,omitempty"`
	ExpiresAt      int64                       `json:"expires_at"`
	CooldownUntil  *time.Time                  `json:"cooldown_until,omitempty"`
	CellID         string                      `json:"cell_id,omitempty"`
	Cell           *EgressCellSummaryResponse  `json:"cell,omitempty"`
	Windows        []UtilizationWindowResponse `json:"windows"`
	Stainless      map[string]interface{}      `json:"stainless,omitempty"`
	Sessions       []domain.SessionBindingInfo `json:"sessions"`
}

// ---------------------------------------------------------------------------
// Account List Item
// ---------------------------------------------------------------------------

type AccountListItem struct {
	ID              string                      `json:"id"`
	Email           string                      `json:"email"`
	Provider        string                      `json:"provider"`
	Status          string                      `json:"status"`
	Weight          int                         `json:"weight"`
	WeightMode      string                      `json:"weight_mode"`
	LastUsedAt      *time.Time                  `json:"last_used_at,omitempty"`
	CooldownUntil   *time.Time                  `json:"cooldown_until,omitempty"`
	CellID          string                      `json:"cell_id,omitempty"`
	AvailableNative bool                        `json:"available_native"`
	AvailableCompat bool                        `json:"available_compat"`
	Cell            *EgressCellSummaryResponse  `json:"cell,omitempty"`
	Windows         []UtilizationWindowResponse `json:"windows"`
}

type EgressCellResponse struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	Proxy         *domain.ProxyConfig    `json:"proxy,omitempty"`
	Labels        map[string]string      `json:"labels,omitempty"`
	CooldownUntil *time.Time             `json:"cooldown_until,omitempty"`
	StateJSON     string                 `json:"state_json,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Accounts      []EgressCellAccountRef `json:"accounts"`
}

type EgressCellAccountRef struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

// ---------------------------------------------------------------------------
// Account Test Result
// ---------------------------------------------------------------------------

type TestAccountResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// User Detail
// ---------------------------------------------------------------------------

type UserDetailResponse struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	TokenPrefix       string                 `json:"token_prefix"`
	Status            string                 `json:"status"`
	AllowedSurface    domain.Surface         `json:"allowed_surface"`
	BoundAccountID    string                 `json:"bound_account_id,omitempty"`
	BoundAccountEmail string                 `json:"bound_account_email,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	LastActiveAt      *time.Time             `json:"last_active_at,omitempty"`
	Usage             []domain.UsagePeriod   `json:"usage"`
	ModelUsage        []domain.ModelUsageRow `json:"model_usage"`
	RecentRequests    []*domain.RequestLog   `json:"recent_requests"`
}
