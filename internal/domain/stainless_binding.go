package domain

import "time"

// StainlessBinding keeps per-account x-stainless identity headers stable across instances.
type StainlessBinding struct {
	AccountID   string
	HeadersJSON string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
