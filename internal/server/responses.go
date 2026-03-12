package server

import (
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

type DashboardResponse struct {
	Health   HealthInfo           `json:"health"`
	Usage    []domain.UsagePeriod `json:"usage"`
	Accounts []DashboardAccount   `json:"accounts"`
	Users    []DashboardUser      `json:"users"`
	Events   []DashboardEvent     `json:"events"`
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
	ID            string                      `json:"id"`
	Email         string                      `json:"email"`
	Provider      string                      `json:"provider"`
	Status        string                      `json:"status"`
	PriorityMode  string                      `json:"priority_mode"`
	Priority      int                         `json:"priority"`
	CooldownUntil *time.Time                  `json:"cooldown_until,omitempty"`
	LastUsedAt    *time.Time                  `json:"last_used_at,omitempty"`
	CellID        string                      `json:"cell_id,omitempty"`
	Windows       []UtilizationWindowResponse `json:"windows"`
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
	Type      string `json:"type"`
	AccountID string `json:"account_id,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"ts"`
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
	Priority       int                         `json:"priority"`
	PriorityMode   string                      `json:"priority_mode"`
	AutoScore      int                         `json:"auto_score"`
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
	ID            string                      `json:"id"`
	Email         string                      `json:"email"`
	Provider      string                      `json:"provider"`
	Status        string                      `json:"status"`
	Priority      int                         `json:"priority"`
	PriorityMode  string                      `json:"priority_mode"`
	LastUsedAt    *time.Time                  `json:"last_used_at,omitempty"`
	CooldownUntil *time.Time                  `json:"cooldown_until,omitempty"`
	CellID        string                      `json:"cell_id,omitempty"`
	Cell          *EgressCellSummaryResponse  `json:"cell,omitempty"`
	Windows       []UtilizationWindowResponse `json:"windows"`
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
