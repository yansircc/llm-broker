package domain

import (
	"encoding/json"
	"time"
)

// Provider identifies the upstream API provider.
type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
)

// Status represents the lifecycle state of an account.
type Status string

const (
	StatusActive   Status = "active"
	StatusCreated  Status = "created"
	StatusError    Status = "error"
	StatusDisabled Status = "disabled"
	StatusBlocked  Status = "blocked"
)

// Account represents an OAuth account (Claude or Codex).
// All fields are persisted via db tags; json tags use snake_case.
type Account struct {
	ID           string   `db:"id"            json:"id"`
	Email        string   `db:"email"         json:"email"`
	Provider     Provider `db:"provider"      json:"provider"`
	Status       Status   `db:"status"        json:"status"`
	Schedulable  bool     `db:"schedulable"   json:"schedulable"`
	Priority     int      `db:"priority"      json:"priority"`
	PriorityMode string   `db:"priority_mode" json:"priority_mode"`
	ErrorMessage string   `db:"error_message" json:"error_message,omitempty"`

	// Encrypted tokens (never exposed via JSON)
	RefreshTokenEnc string `db:"refresh_token_enc" json:"-"`
	AccessTokenEnc  string `db:"access_token_enc"  json:"-"`
	ExpiresAt       int64  `db:"expires_at"        json:"expires_at"`

	// Timestamps
	CreatedAt     time.Time  `db:"created_at"      json:"created_at"`
	LastUsedAt    *time.Time `db:"last_used_at"     json:"last_used_at,omitempty"`
	LastRefreshAt *time.Time `db:"last_refresh_at"  json:"last_refresh_at,omitempty"`

	// Proxy & extra info (stored as JSON strings in DB)
	ProxyJSON   string `db:"proxy_json"    json:"-"`
	ExtInfoJSON string `db:"ext_info_json" json:"-"`

	// Claude rate limits
	FiveHourStatus     string     `db:"five_hour_status"       json:"five_hour_status,omitempty"`
	FiveHourUtil       float64    `db:"five_hour_util"         json:"five_hour_util"`
	FiveHourReset      int64      `db:"five_hour_reset"        json:"five_hour_reset"`
	SevenDayUtil       float64    `db:"seven_day_util"         json:"seven_day_util"`
	SevenDayReset      int64      `db:"seven_day_reset"        json:"seven_day_reset"`
	OpusRateLimitEndAt *time.Time `db:"opus_rate_limit_end_at" json:"opus_rate_limit_end_at,omitempty"`
	OverloadedAt       *time.Time `db:"overloaded_at"          json:"overloaded_at,omitempty"`
	OverloadedUntil    *time.Time `db:"overloaded_until"       json:"overloaded_until,omitempty"`
	RateLimitedAt      *time.Time `db:"rate_limited_at"        json:"rate_limited_at,omitempty"`

	// Codex rate limits
	CodexPrimaryUtil    float64 `db:"codex_primary_util"    json:"codex_primary_util"`
	CodexPrimaryReset   int64   `db:"codex_primary_reset"   json:"codex_primary_reset"`
	CodexSecondaryUtil  float64 `db:"codex_secondary_util"  json:"codex_secondary_util"`
	CodexSecondaryReset int64   `db:"codex_secondary_reset" json:"codex_secondary_reset"`

	// Runtime only (not stored in DB)
	Proxy   *ProxyConfig           `db:"-" json:"proxy,omitempty"`
	ExtInfo map[string]interface{} `db:"-" json:"ext_info,omitempty"`
}

// ProxyConfig holds per-account proxy settings.
type ProxyConfig struct {
	Type     string `json:"type"` // socks5, http, https
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// HydrateRuntime populates the transient Proxy and ExtInfo fields from their
// JSON column counterparts. Called after loading from the database.
func (a *Account) HydrateRuntime() {
	if a.ProxyJSON != "" {
		var p ProxyConfig
		if json.Unmarshal([]byte(a.ProxyJSON), &p) == nil && p.Host != "" {
			a.Proxy = &p
		}
	}
	if a.ExtInfoJSON != "" {
		var ext map[string]interface{}
		if json.Unmarshal([]byte(a.ExtInfoJSON), &ext) == nil {
			a.ExtInfo = ext
		}
	}
}

// PersistRuntime serialises the transient Proxy and ExtInfo fields into their
// JSON column counterparts. Called before saving to the database.
func (a *Account) PersistRuntime() {
	if a.Proxy != nil {
		data, _ := json.Marshal(a.Proxy)
		a.ProxyJSON = string(data)
	} else {
		a.ProxyJSON = ""
	}
	if a.ExtInfo != nil {
		data, _ := json.Marshal(a.ExtInfo)
		a.ExtInfoJSON = string(data)
	} else {
		a.ExtInfoJSON = ""
	}
}

// GetAccountUUID extracts account_uuid from ExtInfo.
func (a *Account) GetAccountUUID() string {
	if a.ExtInfo == nil {
		return ""
	}
	if v, ok := a.ExtInfo["account_uuid"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
