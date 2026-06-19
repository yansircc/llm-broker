package domain

import "time"

type RuntimeSetting struct {
	Key       string    `json:"key"`
	ValueJSON string    `json:"value_json"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

type Integration struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Provider          string    `json:"provider"`
	DisplayName       string    `json:"display_name"`
	Enabled           bool      `json:"enabled"`
	Priority          int       `json:"priority"`
	ConfigJSON        string    `json:"config_json"`
	SecretJSONEnc     string    `json:"-"`
	SecretFingerprint string    `json:"secret_fingerprint"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	UpdatedBy         string    `json:"updated_by"`
}

type IntegrationEvent struct {
	ID                  string    `json:"id"`
	IntegrationID       string    `json:"integration_id"`
	Kind                string    `json:"kind"`
	EventType           string    `json:"event_type"`
	Success             bool      `json:"success"`
	ErrorCode           string    `json:"error_code"`
	RedactedPayloadJSON string    `json:"redacted_payload_json"`
	CreatedAt           time.Time `json:"created_at"`
}

type SettingsAudit struct {
	ID          string    `json:"id"`
	ActorUserID string    `json:"actor_user_id"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Action      string    `json:"action"`
	BeforeJSON  string    `json:"before_json"`
	AfterJSON   string    `json:"after_json"`
	CreatedAt   time.Time `json:"created_at"`
}
