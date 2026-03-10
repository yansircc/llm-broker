package domain

import "time"

// QuotaBucket stores provider-owned quota state shared by one or more credentials.
type QuotaBucket struct {
	BucketKey     string     `db:"bucket_key"     json:"bucket_key"`
	Provider      Provider   `db:"provider"       json:"provider"`
	CooldownUntil *time.Time `db:"cooldown_until" json:"cooldown_until,omitempty"`
	StateJSON     string     `db:"state_json"     json:"-"`
	UpdatedAt     time.Time  `db:"updated_at"     json:"updated_at"`
}
