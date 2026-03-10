package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const quotaBucketCols = `bucket_key, provider, cooldown_until, state_json, updated_at`

func scanQuotaBucket(scanner interface{ Scan(...any) error }) (*domain.QuotaBucket, error) {
	var (
		bucketKey, provider, stateJSON string
		cooldownUntil                  sql.NullInt64
		updatedAt                      int64
	)
	if err := scanner.Scan(&bucketKey, &provider, &cooldownUntil, &stateJSON, &updatedAt); err != nil {
		return nil, err
	}
	return &domain.QuotaBucket{
		BucketKey:     bucketKey,
		Provider:      domain.Provider(provider),
		CooldownUntil: scanNullableTime(cooldownUntil),
		StateJSON:     stateJSON,
		UpdatedAt:     time.Unix(updatedAt, 0).UTC(),
	}, nil
}

func (s *SQLiteStore) GetQuotaBucket(ctx context.Context, bucketKey string) (*domain.QuotaBucket, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+quotaBucketCols+" FROM quota_buckets WHERE bucket_key = ?", bucketKey)
	b, err := scanQuotaBucket(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

func (s *SQLiteStore) ListQuotaBuckets(ctx context.Context) ([]*domain.QuotaBucket, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+quotaBucketCols+" FROM quota_buckets ORDER BY bucket_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []*domain.QuotaBucket
	for rows.Next() {
		b, err := scanQuotaBucket(rows)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func (s *SQLiteStore) SaveQuotaBucket(ctx context.Context, bucket *domain.QuotaBucket) error {
	updatedAt := bucket.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quota_buckets (
			bucket_key, provider, cooldown_until, state_json, updated_at
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(bucket_key) DO UPDATE SET
			provider=excluded.provider,
			cooldown_until=excluded.cooldown_until,
			state_json=excluded.state_json,
			updated_at=excluded.updated_at
	`, bucket.BucketKey, string(bucket.Provider), nullableUnix(bucket.CooldownUntil), bucket.StateJSON, updatedAt.Unix())
	return err
}

func (s *SQLiteStore) DeleteQuotaBucket(ctx context.Context, bucketKey string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM quota_buckets WHERE bucket_key = ?", bucketKey)
	return err
}
