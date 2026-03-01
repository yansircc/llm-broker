package server

import (
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
)

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

type DashboardResponse struct {
	Health   HealthInfo         `json:"health"`
	Usage    []domain.UsagePeriod `json:"usage"`
	Accounts []DashboardAccount `json:"accounts"`
	Users    []DashboardUser    `json:"users"`
	Events   []DashboardEvent   `json:"events"`
}

type HealthInfo struct {
	SQLite  string `json:"sqlite"`
	Uptime  string `json:"uptime"`
	Version string `json:"version"`
}

type DashboardAccount struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Provider        string     `json:"provider"`
	Status          string     `json:"status"`
	PriorityMode    string     `json:"priority_mode"`
	Priority        int        `json:"priority"`
	OverloadedUntil *time.Time `json:"overloaded_until,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	FiveHourUtil    *int       `json:"five_hour_util"`
	SevenDayUtil    *int       `json:"seven_day_util"`
	FiveHourReset   *int64     `json:"five_hour_reset"`
	SevenDayReset   *int64     `json:"seven_day_reset"`
}

type DashboardUser struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
	TotalCost    float64    `json:"total_cost"`
}

type DashboardEvent struct {
	Type      string `json:"type"`
	AccountID string `json:"account_id,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"ts"`
}

// ---------------------------------------------------------------------------
// Account Detail
// ---------------------------------------------------------------------------

type AccountDetailResponse struct {
	ID                 string                         `json:"id"`
	Email              string                         `json:"email"`
	Provider           domain.Provider                `json:"provider"`
	Status             domain.Status                  `json:"status"`
	Priority           int                            `json:"priority"`
	PriorityMode       string                         `json:"priority_mode"`
	AutoScore          int                            `json:"auto_score"`
	Schedulable        bool                           `json:"schedulable"`
	ErrorMessage       string                         `json:"error_message,omitempty"`
	ExtInfo            map[string]interface{}          `json:"ext_info,omitempty"`
	CreatedAt          time.Time                      `json:"created_at"`
	LastUsedAt         *time.Time                     `json:"last_used_at,omitempty"`
	LastRefreshAt      *time.Time                     `json:"last_refresh_at,omitempty"`
	ExpiresAt          int64                          `json:"expires_at"`
	FiveHourStatus     string                         `json:"five_hour_status"`
	OverloadedUntil    *time.Time                     `json:"overloaded_until,omitempty"`
	OpusRateLimitEndAt *time.Time                     `json:"opus_rate_limit_end_at,omitempty"`
	Stainless          map[string]interface{}          `json:"stainless,omitempty"`
	Sessions           []domain.SessionBindingInfo     `json:"sessions"`
}

// ---------------------------------------------------------------------------
// Account List Item
// ---------------------------------------------------------------------------

type AccountListItem struct {
	ID                 string                 `json:"id"`
	Email              string                 `json:"email"`
	Provider           string                 `json:"provider"`
	Status             string                 `json:"status"`
	Priority           int                    `json:"priority"`
	PriorityMode       string                 `json:"priority_mode"`
	Schedulable        bool                   `json:"schedulable"`
	ExtInfo            map[string]interface{} `json:"ext_info,omitempty"`
	LastUsedAt         *time.Time             `json:"last_used_at,omitempty"`
	OverloadedUntil    *time.Time             `json:"overloaded_until,omitempty"`
	FiveHourStatus     string                 `json:"five_hour_status"`
	OpusRateLimitEndAt *time.Time             `json:"opus_rate_limit_end_at,omitempty"`
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
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	TokenPrefix    string                `json:"token_prefix"`
	Status         string                `json:"status"`
	CreatedAt      time.Time             `json:"created_at"`
	LastActiveAt   *time.Time            `json:"last_active_at,omitempty"`
	Usage          []domain.UsagePeriod  `json:"usage"`
	ModelUsage     []domain.ModelUsageRow `json:"model_usage"`
	RecentRequests []*domain.RequestLog  `json:"recent_requests"`
}
