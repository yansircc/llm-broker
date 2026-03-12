package domain

import "time"

// RefreshLock coordinates token refresh work across instances.
type RefreshLock struct {
	AccountID string
	LockID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}
