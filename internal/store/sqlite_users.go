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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (
			id, email, name, password_hash, email_verified_at, status,
			allowed_surface, bound_account_id, referral_code, referred_by_user_id,
			created_at, last_login_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		u.ID, u.Email, u.Name, u.PasswordHash, nullableUnix(u.EmailVerifiedAt), u.Status,
		string(allowedSurface), u.BoundAccountID, u.ReferralCode, u.ReferredByUserID,
		u.CreatedAt.Unix(), nullableUnix(u.LastLoginAt))
	return err
}

func (s *SQLiteStore) GetUser(ctx context.Context, id string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, email_verified_at, status,
			allowed_surface, bound_account_id, referral_code, referred_by_user_id,
			created_at, last_login_at
		FROM users WHERE id = ?
	`, id)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, email_verified_at, status,
			allowed_surface, bound_account_id, referral_code, referred_by_user_id,
			created_at, last_login_at
		FROM users WHERE lower(email) = lower(?)
	`, email)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByReferralCode(ctx context.Context, code string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, email_verified_at, status,
			allowed_surface, bound_account_id, referral_code, referred_by_user_id,
			created_at, last_login_at
		FROM users WHERE referral_code = ?
	`, code)
	return scanUser(row)
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]*domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, name, password_hash, email_verified_at, status,
			allowed_surface, bound_account_id, referral_code, referred_by_user_id,
			created_at, last_login_at
		FROM users ORDER BY created_at
	`)
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

	for _, stmt := range []string{
		"DELETE FROM user_route_bindings WHERE user_id = ?",
		"DELETE FROM api_keys WHERE user_id = ?",
		"DELETE FROM web_sessions WHERE user_id = ?",
		"DELETE FROM email_verifications WHERE user_id = ?",
	} {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return err
		}
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

func (s *SQLiteStore) UpdateUserLastLogin(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET last_login_at = ? WHERE id = ?", time.Now().Unix(), id)
	return err
}

func (s *SQLiteStore) MarkUserEmailVerified(ctx context.Context, id string, verifiedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, "UPDATE users SET email_verified_at = ? WHERE id = ?", verifiedAt.Unix(), id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func scanUser(scanner interface{ Scan(...any) error }) (*domain.User, error) {
	var (
		id, email, name, passwordHash, status, allowedSurface, boundAccountID, referralCode, referredByUserID string
		createdAt                                                                                             int64
		emailVerifiedAt, lastLoginAt                                                                          sql.NullInt64
	)
	err := scanner.Scan(
		&id, &email, &name, &passwordHash, &emailVerifiedAt, &status,
		&allowedSurface, &boundAccountID, &referralCode, &referredByUserID,
		&createdAt, &lastLoginAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		ID:               id,
		Email:            email,
		Name:             name,
		PasswordHash:     passwordHash,
		Status:           status,
		AllowedSurface:   domain.NormalizeSurface(allowedSurface),
		BoundAccountID:   boundAccountID,
		ReferralCode:     referralCode,
		ReferredByUserID: referredByUserID,
		CreatedAt:        time.Unix(createdAt, 0).UTC(),
		EmailVerifiedAt:  scanNullableTime(emailVerifiedAt),
		LastLoginAt:      scanNullableTime(lastLoginAt),
	}
	if u.AllowedSurface == "" {
		u.AllowedSurface = domain.SurfaceNative
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
