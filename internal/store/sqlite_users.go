package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) CreateUser(ctx context.Context, u *domain.User) error {
	allowedSurface := u.AllowedSurface
	if allowedSurface == "" {
		allowedSurface = domain.SurfaceNative
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, token_hash, token_prefix, status, allowed_surface, bound_account_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.TokenHash, u.TokenPrefix, u.Status, string(allowedSurface), u.BoundAccountID, u.CreatedAt.Unix())
	return err
}

func (s *SQLiteStore) GetUserByTokenHash(ctx context.Context, tokenHash string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, token_hash, token_prefix, status, allowed_surface, bound_account_id, created_at, last_active_at FROM users WHERE token_hash = ?`,
		tokenHash)
	return scanUser(row)
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]*domain.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, token_hash, token_prefix, status, allowed_surface, bound_account_id, created_at, last_active_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLiteStore) DeleteUser(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM user_route_bindings WHERE user_id = ?", id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(result); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) UpdateUserStatus(ctx context.Context, id, status string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE users SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) UpdateUserToken(ctx context.Context, id, tokenHash, tokenPrefix string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE users SET token_hash = ?, token_prefix = ? WHERE id = ?", tokenHash, tokenPrefix, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) UpdateUserPolicy(ctx context.Context, id string, allowedSurface domain.Surface, boundAccountID string) error {
	if allowedSurface == "" {
		allowedSurface = domain.SurfaceNative
	}
	result, err := s.db.ExecContext(ctx,
		"UPDATE users SET allowed_surface = ?, bound_account_id = ? WHERE id = ?",
		string(allowedSurface), boundAccountID, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) UpdateUserLastActive(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET last_active_at = ? WHERE id = ?", time.Now().Unix(), id)
	return err
}

func scanUser(scanner interface{ Scan(...any) error }) (*domain.User, error) {
	var (
		id, name, tokenHash, tokenPrefix, status, allowedSurface, boundAccountID string
		createdAt                                                                int64
		lastActiveAt                                                             sql.NullInt64
	)
	err := scanner.Scan(&id, &name, &tokenHash, &tokenPrefix, &status, &allowedSurface, &boundAccountID, &createdAt, &lastActiveAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		ID:             id,
		Name:           name,
		TokenHash:      tokenHash,
		TokenPrefix:    tokenPrefix,
		Status:         status,
		AllowedSurface: domain.NormalizeSurface(allowedSurface),
		BoundAccountID: boundAccountID,
		CreatedAt:      time.Unix(createdAt, 0).UTC(),
	}
	if u.AllowedSurface == "" {
		u.AllowedSurface = domain.SurfaceNative
	}
	if lastActiveAt.Valid {
		t := time.Unix(lastActiveAt.Int64, 0).UTC()
		u.LastActiveAt = &t
	}
	return u, nil
}

func ensureRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
