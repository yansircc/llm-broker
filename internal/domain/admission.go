package domain

import "time"

type AdmissionLimit struct {
	Scope             string    `json:"scope"`
	ScopeID           string    `json:"scope_id"`
	MaxConcurrent     int       `json:"max_concurrent"`
	RequestsPerMinute int       `json:"requests_per_minute"`
	MinBalanceMicros  int64     `json:"min_balance_micros"`
	UpdatedAt         time.Time `json:"updated_at"`
}
