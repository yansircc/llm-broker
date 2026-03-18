package domain

import "time"

// UserRouteBinding keeps a soft user->account affinity per provider/surface.
// It is advisory routing state: unavailable accounts are ignored and replaced.
type UserRouteBinding struct {
	UserID     string
	Provider   Provider
	Surface    Surface
	AccountID  string
	CreatedAt  time.Time
	LastUsedAt time.Time
}
