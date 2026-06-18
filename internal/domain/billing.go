package domain

import "time"

const CreditMicrosPerUnit = int64(1_000_000)

type BillingSetting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ModelPrice struct {
	Model                       string    `json:"model"`
	InputMicrosPerMillion       int64     `json:"input_micros_per_million"`
	OutputMicrosPerMillion      int64     `json:"output_micros_per_million"`
	CacheReadMicrosPerMillion   int64     `json:"cache_read_micros_per_million"`
	CacheCreateMicrosPerMillion int64     `json:"cache_create_micros_per_million"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

type BillingLedgerEntry struct {
	Seq               int64     `json:"seq"`
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	AmountMicros      int64     `json:"amount_micros"`
	Kind              string    `json:"kind"`
	SourceType        string    `json:"source_type"`
	SourceID          string    `json:"source_id"`
	IdempotencyKey    string    `json:"idempotency_key"`
	Description       string    `json:"description"`
	PriceSnapshotJSON string    `json:"price_snapshot_json"`
	MetadataJSON      string    `json:"metadata_json"`
	CreatedAt         time.Time `json:"created_at"`
}

type BillingLedgerSummary struct {
	CreditMicros int64 `json:"credit_micros"`
	UsageMicros  int64 `json:"usage_micros"`
}

type BillingBalanceCheckpoint struct {
	UserID        string    `json:"user_id"`
	LedgerSeq     int64     `json:"ledger_seq"`
	BalanceMicros int64     `json:"balance_micros"`
	CreatedAt     time.Time `json:"created_at"`
}

type BillableRequest struct {
	RequestID         string     `json:"request_id"`
	UserID            string     `json:"user_id"`
	APIKeyID          string     `json:"api_key_id"`
	Model             string     `json:"model"`
	Surface           Surface    `json:"surface"`
	Status            string     `json:"status"`
	InputTokens       int        `json:"input_tokens"`
	OutputTokens      int        `json:"output_tokens"`
	CacheReadTokens   int        `json:"cache_read_tokens"`
	CacheCreateTokens int        `json:"cache_create_tokens"`
	PriceSnapshotJSON string     `json:"price_snapshot_json"`
	LedgerID          string     `json:"ledger_id"`
	Error             string     `json:"error,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UsageObservedAt   *time.Time `json:"usage_observed_at,omitempty"`
	SettledAt         *time.Time `json:"settled_at,omitempty"`
}

type Referral struct {
	ID            string    `json:"id"`
	InviterUserID string    `json:"inviter_user_id"`
	InviteeUserID string    `json:"invitee_user_id"`
	InviteCode    string    `json:"invite_code"`
	CreatedAt     time.Time `json:"created_at"`
	CreditedAt    time.Time `json:"credited_at"`
}

type ReferralStats struct {
	Signups      int   `json:"signups"`
	PaidInvitees int   `json:"paid_invitees"`
	CreditMicros int64 `json:"credit_micros"`
}
