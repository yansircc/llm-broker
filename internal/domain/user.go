package domain

import "time"

// User represents an API user with a hashed token.
type User struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	TokenHash    string     `json:"-"`
	TokenPrefix  string     `json:"token_prefix"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
}
