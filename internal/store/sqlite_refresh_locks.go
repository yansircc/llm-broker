package store

import (
	"context"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) AcquireRefreshLock(ctx context.Context, lock *domain.RefreshLock) (bool, error) {
	createdAt := lock.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	expiresAt := lock.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = createdAt
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO refresh_locks (
			account_id, lock_id, created_at, expires_at
		) VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			lock_id=excluded.lock_id,
			created_at=excluded.created_at,
			expires_at=excluded.expires_at
		WHERE refresh_locks.expires_at <= excluded.created_at
	`, lock.AccountID, lock.LockID, createdAt.Unix(), expiresAt.Unix())
	if err != nil {
		return false, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (s *SQLiteStore) ReleaseRefreshLock(ctx context.Context, accountID, lockID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM refresh_locks WHERE account_id = ? AND lock_id = ?", accountID, lockID)
	return err
}

func (s *SQLiteStore) PurgeExpiredRefreshLocks(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM refresh_locks WHERE expires_at <= ?", before.UTC().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
