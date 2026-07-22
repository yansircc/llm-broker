package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const sessionBindingCols = `session_uuid, provider, subject, created_at, last_used_at, expires_at`

func scanSessionBinding(scanner interface{ Scan(...any) error }) (*domain.SessionBinding, error) {
	var (
		sessionUUID string
		provider    string
		subject     string
		createdAt   int64
		lastUsedAt  int64
		expiresAt   int64
	)
	if err := scanner.Scan(&sessionUUID, &provider, &subject, &createdAt, &lastUsedAt, &expiresAt); err != nil {
		return nil, err
	}
	return &domain.SessionBinding{
		SessionUUID: sessionUUID,
		Provider:    domain.Provider(provider),
		Subject:     subject,
		CreatedAt:   time.Unix(createdAt, 0).UTC(),
		LastUsedAt:  time.Unix(lastUsedAt, 0).UTC(),
		ExpiresAt:   time.Unix(expiresAt, 0).UTC(),
	}, nil
}

func (s *SQLiteStore) GetSessionBinding(ctx context.Context, sessionUUID string) (*domain.SessionBinding, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+sessionBindingCols+" FROM session_bindings WHERE session_uuid = ? AND expires_at > ?",
		sessionUUID,
		time.Now().UTC().Unix(),
	)
	binding, err := scanSessionBinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return binding, err
}

func (s *SQLiteStore) ListSessionBindingsByTarget(ctx context.Context, provider domain.Provider, subject string) ([]domain.SessionBinding, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+sessionBindingCols+" FROM session_bindings WHERE provider = ? AND subject = ? AND expires_at > ? ORDER BY last_used_at DESC, session_uuid",
		string(provider),
		subject,
		time.Now().UTC().Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bindings []domain.SessionBinding
	for rows.Next() {
		binding, err := scanSessionBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, *binding)
	}
	return bindings, rows.Err()
}

func (s *SQLiteStore) SaveSessionBinding(ctx context.Context, binding *domain.SessionBinding) error {
	createdAt := binding.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	lastUsedAt := binding.LastUsedAt.UTC()
	if lastUsedAt.IsZero() {
		lastUsedAt = createdAt
	}
	expiresAt := binding.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = lastUsedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO session_bindings (
			session_uuid, provider, subject, created_at, last_used_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_uuid) DO UPDATE SET
			provider=excluded.provider,
			subject=excluded.subject,
			created_at=excluded.created_at,
			last_used_at=excluded.last_used_at,
			expires_at=excluded.expires_at
	`, binding.SessionUUID, string(binding.Provider), binding.Subject, createdAt.Unix(), lastUsedAt.Unix(), expiresAt.Unix())
	return err
}

func (s *SQLiteStore) DeleteSessionBinding(ctx context.Context, sessionUUID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM session_bindings WHERE session_uuid = ?", sessionUUID)
	return err
}

func (s *SQLiteStore) PurgeExpiredSessionBindings(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM session_bindings WHERE expires_at <= ?", before.UTC().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
