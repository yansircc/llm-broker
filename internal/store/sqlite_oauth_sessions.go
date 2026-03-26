package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const oauthSessionCols = `session_id, data_json, created_at, expires_at`

func scanOAuthSession(scanner interface{ Scan(...any) error }) (*domain.OAuthSessionState, error) {
	var (
		sessionID string
		dataJSON  string
		createdAt int64
		expiresAt int64
	)
	if err := scanner.Scan(&sessionID, &dataJSON, &createdAt, &expiresAt); err != nil {
		return nil, err
	}
	return &domain.OAuthSessionState{
		SessionID: sessionID,
		DataJSON:  dataJSON,
		CreatedAt: time.Unix(createdAt, 0).UTC(),
		ExpiresAt: time.Unix(expiresAt, 0).UTC(),
	}, nil
}

func (s *SQLiteStore) SaveOAuthSession(ctx context.Context, session *domain.OAuthSessionState) error {
	createdAt := session.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	expiresAt := session.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = createdAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_sessions (
			session_id, data_json, created_at, expires_at
		) VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			data_json=excluded.data_json,
			created_at=excluded.created_at,
			expires_at=excluded.expires_at
	`, session.SessionID, session.DataJSON, createdAt.Unix(), expiresAt.Unix())
	return err
}

func (s *SQLiteStore) GetOAuthSession(ctx context.Context, sessionID string) (*domain.OAuthSessionState, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+oauthSessionCols+" FROM oauth_sessions WHERE session_id = ? AND expires_at > ?",
		sessionID,
		time.Now().UTC().Unix(),
	)
	session, err := scanOAuthSession(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return session, err
}

func (s *SQLiteStore) DeleteOAuthSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM oauth_sessions WHERE session_id = ?", sessionID)
	return err
}

func (s *SQLiteStore) GetAndDeleteOAuthSession(ctx context.Context, sessionID string) (*domain.OAuthSessionState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		"SELECT "+oauthSessionCols+" FROM oauth_sessions WHERE session_id = ? AND expires_at > ?",
		sessionID,
		time.Now().UTC().Unix(),
	)
	session, err := scanOAuthSession(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM oauth_sessions WHERE session_id = ?", sessionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SQLiteStore) PurgeExpiredOAuthSessions(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM oauth_sessions WHERE expires_at <= ?", before.UTC().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
