package domain

import "time"

// SessionBinding keeps sticky-session routing state durable across restarts.
type SessionBinding struct {
	SessionUUID string
	Provider    Provider
	Subject     string
	CreatedAt   time.Time
	LastUsedAt  time.Time
	ExpiresAt   time.Time
}

func (b SessionBinding) Info(accountID string) SessionBindingInfo {
	return SessionBindingInfo{
		SessionUUID: b.SessionUUID,
		AccountID:   accountID,
		CreatedAt:   b.CreatedAt.UTC().Format(time.RFC3339),
		LastUsedAt:  b.LastUsedAt.UTC().Format(time.RFC3339),
		ExpiresAt:   b.ExpiresAt.UTC(),
	}
}
