package domain

import "time"

// User is the customer identity and policy source.
// API credentials and browser sessions live in api_keys and web_sessions.
type User struct {
	ID               string     `json:"id"`
	Email            string     `json:"email"`
	Name             string     `json:"name"`
	PasswordHash     string     `json:"-"`
	EmailVerifiedAt  *time.Time `json:"email_verified_at,omitempty"`
	Status           string     `json:"status"`
	AllowedSurface   Surface    `json:"allowed_surface"`
	BoundAccountID   string     `json:"bound_account_id,omitempty"`
	ReferralCode     string     `json:"referral_code"`
	ReferredByUserID string     `json:"referred_by_user_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
}
