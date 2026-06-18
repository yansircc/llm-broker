package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) UpsertBillingSetting(ctx context.Context, key, value string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, updatedAt.Unix())
	return err
}

func (s *SQLiteStore) GetBillingSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM billing_settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStore) UpsertModelPrice(ctx context.Context, price *domain.ModelPrice) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO model_prices (
			model, input_micros_per_million, output_micros_per_million,
			cache_read_micros_per_million, cache_create_micros_per_million, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(model) DO UPDATE SET
			input_micros_per_million = excluded.input_micros_per_million,
			output_micros_per_million = excluded.output_micros_per_million,
			cache_read_micros_per_million = excluded.cache_read_micros_per_million,
			cache_create_micros_per_million = excluded.cache_create_micros_per_million,
			updated_at = excluded.updated_at
	`,
		price.Model, price.InputMicrosPerMillion, price.OutputMicrosPerMillion,
		price.CacheReadMicrosPerMillion, price.CacheCreateMicrosPerMillion, price.UpdatedAt.Unix())
	return err
}

func (s *SQLiteStore) GetModelPrice(ctx context.Context, model string) (*domain.ModelPrice, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT model, input_micros_per_million, output_micros_per_million,
			cache_read_micros_per_million, cache_create_micros_per_million, updated_at
		FROM model_prices WHERE model = ?
	`, model)
	return scanModelPrice(row)
}

func (s *SQLiteStore) ListModelPrices(ctx context.Context) ([]*domain.ModelPrice, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT model, input_micros_per_million, output_micros_per_million,
			cache_read_micros_per_million, cache_create_micros_per_million, updated_at
		FROM model_prices ORDER BY model
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prices []*domain.ModelPrice
	for rows.Next() {
		price, err := scanModelPrice(rows)
		if err != nil {
			return nil, err
		}
		prices = append(prices, price)
	}
	return prices, rows.Err()
}

func (s *SQLiteStore) InsertBillingLedgerEntry(ctx context.Context, entry *domain.BillingLedgerEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_ledger (
			id, user_id, amount_micros, kind, source_type, source_id, idempotency_key,
			description, price_snapshot_json, metadata_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.ID, entry.UserID, entry.AmountMicros, entry.Kind, entry.SourceType, entry.SourceID,
		entry.IdempotencyKey, entry.Description, entry.PriceSnapshotJSON, entry.MetadataJSON, entry.CreatedAt.Unix())
	return err
}

func (s *SQLiteStore) GetBillingLedgerEntryByIdempotencyKey(ctx context.Context, key string) (*domain.BillingLedgerEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT seq, id, user_id, amount_micros, kind, source_type, source_id, idempotency_key,
			description, price_snapshot_json, metadata_json, created_at
		FROM billing_ledger WHERE idempotency_key = ?
	`, key)
	return scanBillingLedgerEntry(row)
}

func (s *SQLiteStore) SumBillingLedgerAfter(ctx context.Context, userID string, afterSeq int64) (int64, int64, error) {
	var sum sql.NullInt64
	var maxSeq sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros), 0), COALESCE(MAX(seq), ?)
		FROM billing_ledger WHERE user_id = ? AND seq > ?
	`, afterSeq, userID, afterSeq).Scan(&sum, &maxSeq)
	if err != nil {
		return 0, 0, err
	}
	return sum.Int64, maxSeq.Int64, nil
}

func (s *SQLiteStore) GetBillingBalanceCheckpoint(ctx context.Context, userID string) (*domain.BillingBalanceCheckpoint, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, ledger_seq, balance_micros, created_at
		FROM billing_balance_checkpoints WHERE user_id = ?
	`, userID)
	return scanBillingBalanceCheckpoint(row)
}

func (s *SQLiteStore) UpsertBillingBalanceCheckpoint(ctx context.Context, checkpoint *domain.BillingBalanceCheckpoint) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_balance_checkpoints (user_id, ledger_seq, balance_micros, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			ledger_seq = excluded.ledger_seq,
			balance_micros = excluded.balance_micros,
			created_at = excluded.created_at
	`, checkpoint.UserID, checkpoint.LedgerSeq, checkpoint.BalanceMicros, checkpoint.CreatedAt.Unix())
	return err
}

func (s *SQLiteStore) CreateBillableRequest(ctx context.Context, br *domain.BillableRequest) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO billable_requests (
			request_id, user_id, api_key_id, model, surface, status,
			input_tokens, output_tokens, cache_read_tokens, cache_create_tokens,
			price_snapshot_json, ledger_id, error, created_at, usage_observed_at, settled_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		br.RequestID, br.UserID, br.APIKeyID, br.Model, string(br.Surface), br.Status,
		br.InputTokens, br.OutputTokens, br.CacheReadTokens, br.CacheCreateTokens,
		br.PriceSnapshotJSON, br.LedgerID, br.Error, br.CreatedAt.Unix(),
		nullableUnix(br.UsageObservedAt), nullableUnix(br.SettledAt))
	return err
}

func (s *SQLiteStore) GetBillableRequest(ctx context.Context, requestID string) (*domain.BillableRequest, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT request_id, user_id, api_key_id, model, surface, status,
			input_tokens, output_tokens, cache_read_tokens, cache_create_tokens,
			price_snapshot_json, ledger_id, error, created_at, usage_observed_at, settled_at
		FROM billable_requests WHERE request_id = ?
	`, requestID)
	return scanBillableRequest(row)
}

func (s *SQLiteStore) UpdateBillableRequestUsage(ctx context.Context, br *domain.BillableRequest) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE billable_requests SET
			status = ?, input_tokens = ?, output_tokens = ?, cache_read_tokens = ?,
			cache_create_tokens = ?, price_snapshot_json = ?, usage_observed_at = ?
		WHERE request_id = ?
	`,
		br.Status, br.InputTokens, br.OutputTokens, br.CacheReadTokens, br.CacheCreateTokens,
		br.PriceSnapshotJSON, nullableUnix(br.UsageObservedAt), br.RequestID)
	return err
}

func (s *SQLiteStore) MarkBillableRequestSettled(ctx context.Context, requestID, ledgerID string, settledAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE billable_requests SET status = 'settled', ledger_id = ?, settled_at = ?
		WHERE request_id = ?
	`, ledgerID, settledAt.Unix(), requestID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) MarkBillableRequestStatus(ctx context.Context, requestID, status, errMsg string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE billable_requests SET status = ?, error = ? WHERE request_id = ?
	`, status, errMsg, requestID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) ListUnsettledUsageObservedRequests(ctx context.Context, limit int) ([]*domain.BillableRequest, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT request_id, user_id, api_key_id, model, surface, status,
			input_tokens, output_tokens, cache_read_tokens, cache_create_tokens,
			price_snapshot_json, ledger_id, error, created_at, usage_observed_at, settled_at
		FROM billable_requests
		WHERE status = 'usage_observed' AND ledger_id = ''
		ORDER BY created_at LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []*domain.BillableRequest
	for rows.Next() {
		br, err := scanBillableRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, br)
	}
	return requests, rows.Err()
}

func (s *SQLiteStore) CreateReferralWithCredits(ctx context.Context, referral *domain.Referral, inviteeCredit, inviterCredit *domain.BillingLedgerEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO referrals (id, inviter_user_id, invitee_user_id, invite_code, created_at, credited_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, referral.ID, referral.InviterUserID, referral.InviteeUserID, referral.InviteCode, referral.CreatedAt.Unix(), referral.CreditedAt.Unix()); err != nil {
		return err
	}
	for _, entry := range []*domain.BillingLedgerEntry{inviteeCredit, inviterCredit} {
		if entry == nil || entry.AmountMicros == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO billing_ledger (
				id, user_id, amount_micros, kind, source_type, source_id, idempotency_key,
				description, price_snapshot_json, metadata_json, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			entry.ID, entry.UserID, entry.AmountMicros, entry.Kind, entry.SourceType, entry.SourceID,
			entry.IdempotencyKey, entry.Description, entry.PriceSnapshotJSON, entry.MetadataJSON, entry.CreatedAt.Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetReferralByInvitee(ctx context.Context, inviteeUserID string) (*domain.Referral, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, inviter_user_id, invitee_user_id, invite_code, created_at, credited_at
		FROM referrals WHERE invitee_user_id = ?
	`, inviteeUserID)
	return scanReferral(row)
}

func scanModelPrice(scanner interface{ Scan(...any) error }) (*domain.ModelPrice, error) {
	var p domain.ModelPrice
	var updatedAt int64
	err := scanner.Scan(&p.Model, &p.InputMicrosPerMillion, &p.OutputMicrosPerMillion,
		&p.CacheReadMicrosPerMillion, &p.CacheCreateMicrosPerMillion, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &p, nil
}

func scanBillingLedgerEntry(scanner interface{ Scan(...any) error }) (*domain.BillingLedgerEntry, error) {
	var entry domain.BillingLedgerEntry
	var createdAt int64
	err := scanner.Scan(&entry.Seq, &entry.ID, &entry.UserID, &entry.AmountMicros, &entry.Kind,
		&entry.SourceType, &entry.SourceID, &entry.IdempotencyKey, &entry.Description,
		&entry.PriceSnapshotJSON, &entry.MetadataJSON, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entry.CreatedAt = time.Unix(createdAt, 0).UTC()
	return &entry, nil
}

func scanBillingBalanceCheckpoint(scanner interface{ Scan(...any) error }) (*domain.BillingBalanceCheckpoint, error) {
	var checkpoint domain.BillingBalanceCheckpoint
	var createdAt int64
	err := scanner.Scan(&checkpoint.UserID, &checkpoint.LedgerSeq, &checkpoint.BalanceMicros, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	checkpoint.CreatedAt = time.Unix(createdAt, 0).UTC()
	return &checkpoint, nil
}

func scanBillableRequest(scanner interface{ Scan(...any) error }) (*domain.BillableRequest, error) {
	var br domain.BillableRequest
	var surface string
	var createdAt int64
	var usageObservedAt, settledAt sql.NullInt64
	err := scanner.Scan(&br.RequestID, &br.UserID, &br.APIKeyID, &br.Model, &surface, &br.Status,
		&br.InputTokens, &br.OutputTokens, &br.CacheReadTokens, &br.CacheCreateTokens,
		&br.PriceSnapshotJSON, &br.LedgerID, &br.Error, &createdAt, &usageObservedAt, &settledAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	br.Surface = domain.NormalizeSurface(surface)
	br.CreatedAt = time.Unix(createdAt, 0).UTC()
	br.UsageObservedAt = scanNullableTime(usageObservedAt)
	br.SettledAt = scanNullableTime(settledAt)
	return &br, nil
}

func scanReferral(scanner interface{ Scan(...any) error }) (*domain.Referral, error) {
	var r domain.Referral
	var createdAt, creditedAt int64
	err := scanner.Scan(&r.ID, &r.InviterUserID, &r.InviteeUserID, &r.InviteCode, &createdAt, &creditedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = time.Unix(createdAt, 0).UTC()
	r.CreditedAt = time.Unix(creditedAt, 0).UTC()
	return &r, nil
}
