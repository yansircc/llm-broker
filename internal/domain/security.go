package domain

import "time"

// SecurityEvent is an audit row for customer auth/signup risk controls.
// Raw IPs and emails must be hashed before this boundary.
type SecurityEvent struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	IPHash    string    `json:"ip_hash"`
	EmailHash string    `json:"email_hash"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type SecurityEventQuery struct {
	Kind      string
	IPHash    string
	EmailHash string
	Success   *bool
	Since     time.Time
}
