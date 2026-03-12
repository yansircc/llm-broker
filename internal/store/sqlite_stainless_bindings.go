package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const stainlessBindingCols = `account_id, headers_json, created_at, expires_at`

func scanStainlessBinding(scanner interface{ Scan(...any) error }) (*domain.StainlessBinding, error) {
	var (
		accountID   string
		headersJSON string
		createdAt   int64
		expiresAt   int64
	)
	if err := scanner.Scan(&accountID, &headersJSON, &createdAt, &expiresAt); err != nil {
		return nil, err
	}
	return &domain.StainlessBinding{
		AccountID:   accountID,
		HeadersJSON: headersJSON,
		CreatedAt:   time.Unix(createdAt, 0).UTC(),
		ExpiresAt:   time.Unix(expiresAt, 0).UTC(),
	}, nil
}

func (s *SQLiteStore) GetStainlessBinding(ctx context.Context, accountID string) (*domain.StainlessBinding, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+stainlessBindingCols+" FROM stainless_bindings WHERE account_id = ? AND expires_at > ?",
		accountID,
		time.Now().UTC().Unix(),
	)
	binding, err := scanStainlessBinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return binding, err
}

func (s *SQLiteStore) SetStainlessBindingNX(ctx context.Context, binding *domain.StainlessBinding) (bool, error) {
	createdAt := binding.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	expiresAt := binding.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = createdAt
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO stainless_bindings (
			account_id, headers_json, created_at, expires_at
		) VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			headers_json=excluded.headers_json,
			created_at=excluded.created_at,
			expires_at=excluded.expires_at
		WHERE stainless_bindings.expires_at <= excluded.created_at
	`, binding.AccountID, binding.HeadersJSON, createdAt.Unix(), expiresAt.Unix())
	if err != nil {
		return false, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (s *SQLiteStore) DeleteStainlessBinding(ctx context.Context, accountID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM stainless_bindings WHERE account_id = ?", accountID)
	return err
}

func (s *SQLiteStore) PurgeExpiredStainlessBindings(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM stainless_bindings WHERE expires_at <= ?", before.UTC().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
