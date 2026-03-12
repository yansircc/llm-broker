package domain

import "time"

// OAuthSessionState stores a pending OAuth exchange envelope until callback consumption.
type OAuthSessionState struct {
	SessionID string
	DataJSON  string
	CreatedAt time.Time
	ExpiresAt time.Time
}
