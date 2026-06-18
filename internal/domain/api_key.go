package domain

import "time"

// APIKey is a relay credential owned by a customer.
type APIKey struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Name           string     `json:"name"`
	TokenHash      string     `json:"-"`
	TokenPrefix    string     `json:"token_prefix"`
	Status         string     `json:"status"`
	AllowedSurface Surface    `json:"allowed_surface"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}
