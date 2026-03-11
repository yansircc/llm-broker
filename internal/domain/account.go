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
	ProviderGemini Provider = "gemini"
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

// Account represents an OAuth account.
// All fields are persisted via db tags; json tags use snake_case.
type Account struct {
	ID           string   `db:"id"            json:"id"`
	Email        string   `db:"email"         json:"email"`
	Provider     Provider `db:"provider"      json:"provider"`
	Status       Status   `db:"status"        json:"status"`
	Priority     int      `db:"priority"      json:"priority"`
	PriorityMode string   `db:"priority_mode" json:"priority_mode"`
	ErrorMessage string   `db:"error_message" json:"error_message,omitempty"`
	BucketKey    string   `db:"bucket_key"    json:"bucket_key,omitempty"`

	// Encrypted tokens (never exposed via JSON)
	RefreshTokenEnc string `db:"refresh_token_enc" json:"-"`
	AccessTokenEnc  string `db:"access_token_enc"  json:"-"`
	ExpiresAt       int64  `db:"expires_at"        json:"expires_at"`

	// Timestamps
	CreatedAt     time.Time  `db:"created_at"      json:"created_at"`
	LastUsedAt    *time.Time `db:"last_used_at"     json:"last_used_at,omitempty"`
	LastRefreshAt *time.Time `db:"last_refresh_at"  json:"last_refresh_at,omitempty"`

	// Proxy & persisted identity (stored as JSON strings in DB)
	ProxyJSON    string `db:"proxy_json"    json:"-"`
	CellID       string `db:"cell_id"       json:"cell_id,omitempty"`
	IdentityJSON string `db:"identity_json" json:"-"`

	Subject string `db:"subject" json:"subject,omitempty"`

	// Quota state is projected from the assigned bucket at read time.
	CooldownUntil     *time.Time `db:"-" json:"cooldown_until,omitempty"`
	ProviderStateJSON string     `db:"-" json:"-"`

	// Runtime only (not stored in DB)
	Proxy    *ProxyConfig      `db:"-" json:"-"`
	Identity map[string]string `db:"-" json:"-"`
	Cell     *EgressCell       `db:"-" json:"-"`
}

// ProxyConfig holds per-account proxy settings.
type ProxyConfig struct {
	Type     string `json:"type"` // socks5, http, https
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// HydrateRuntime populates the transient Proxy and Identity fields from their
// JSON column counterparts. Called after loading from the database.
func (a *Account) HydrateRuntime() {
	if a.ProxyJSON != "" {
		var p ProxyConfig
		if json.Unmarshal([]byte(a.ProxyJSON), &p) == nil && p.Host != "" {
			a.Proxy = &p
		}
	}
	if a.IdentityJSON != "" {
		var identity map[string]string
		if json.Unmarshal([]byte(a.IdentityJSON), &identity) == nil {
			a.Identity = identity
		}
	}
}

// PersistRuntime serialises the transient Proxy and Identity fields into their
// JSON column counterparts. Called before saving to the database.
func (a *Account) PersistRuntime() {
	if a.Proxy != nil {
		data, _ := json.Marshal(a.Proxy)
		a.ProxyJSON = string(data)
	} else {
		a.ProxyJSON = ""
	}
	if a.Identity != nil {
		data, _ := json.Marshal(a.Identity)
		a.IdentityJSON = string(data)
	} else {
		a.IdentityJSON = ""
	}
}

// IdentityString returns a string value from persisted account identity data.
func (a *Account) IdentityString(key string) string {
	if a.Identity == nil {
		return ""
	}
	return a.Identity[key]
}

func (a *Account) TransportProxy() *ProxyConfig {
	if a == nil {
		return nil
	}
	if a.Cell != nil && a.Cell.Proxy != nil {
		return a.Cell.Proxy
	}
	return a.Proxy
}
