package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) UpsertAdmissionLimit(ctx context.Context, limit *domain.AdmissionLimit) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admission_limits (
			scope, scope_id, max_concurrent, requests_per_minute, min_balance_micros, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, scope_id) DO UPDATE SET
			max_concurrent = excluded.max_concurrent,
			requests_per_minute = excluded.requests_per_minute,
			min_balance_micros = excluded.min_balance_micros,
			updated_at = excluded.updated_at
	`, limit.Scope, limit.ScopeID, limit.MaxConcurrent, limit.RequestsPerMinute, limit.MinBalanceMicros, limit.UpdatedAt.Unix())
	return err
}

func (s *SQLiteStore) GetAdmissionLimit(ctx context.Context, scope, scopeID string) (*domain.AdmissionLimit, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT scope, scope_id, max_concurrent, requests_per_minute, min_balance_micros, updated_at
		FROM admission_limits WHERE scope = ? AND scope_id = ?
	`, scope, scopeID)
	return scanAdmissionLimit(row)
}

func (s *SQLiteStore) ListAdmissionLimits(ctx context.Context) ([]*domain.AdmissionLimit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT scope, scope_id, max_concurrent, requests_per_minute, min_balance_micros, updated_at
		FROM admission_limits ORDER BY scope, scope_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var limits []*domain.AdmissionLimit
	for rows.Next() {
		limit, err := scanAdmissionLimit(rows)
		if err != nil {
			return nil, err
		}
		limits = append(limits, limit)
	}
	return limits, rows.Err()
}

func scanAdmissionLimit(scanner interface{ Scan(...any) error }) (*domain.AdmissionLimit, error) {
	var limit domain.AdmissionLimit
	var updatedAt int64
	err := scanner.Scan(&limit.Scope, &limit.ScopeID, &limit.MaxConcurrent,
		&limit.RequestsPerMinute, &limit.MinBalanceMicros, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	limit.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &limit, nil
}
