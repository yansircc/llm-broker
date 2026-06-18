package domain

import "time"

// WebSession is a browser credential for customer pages.
type WebSession struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TokenHash  string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// EmailVerification proves ownership of a customer's email.
type EmailVerification struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Email      string     `json:"email"`
	TokenHash  string     `json:"-"`
	Purpose    string     `json:"purpose"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty"`
}
