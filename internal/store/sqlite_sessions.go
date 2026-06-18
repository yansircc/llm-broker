package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) CreateWebSession(ctx context.Context, session *domain.WebSession) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO web_sessions (id, user_id, token_hash, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.TokenHash, session.CreatedAt.Unix(), session.LastSeenAt.Unix(), session.ExpiresAt.Unix())
	return err
}

func (s *SQLiteStore) GetWebSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.WebSession, *domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			s.id, s.user_id, s.token_hash, s.created_at, s.last_seen_at, s.expires_at,
			u.id, u.email, u.name, u.password_hash, u.email_verified_at, u.status,
			u.allowed_surface, u.bound_account_id, u.referral_code, u.referred_by_user_id,
			u.created_at, u.last_login_at
		FROM web_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?
	`, tokenHash, time.Now().Unix())
	return scanWebSessionAndUser(row)
}

func (s *SQLiteStore) DeleteWebSessionByTokenHash(ctx context.Context, tokenHash string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM web_sessions WHERE token_hash = ?", tokenHash)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) TouchWebSession(ctx context.Context, id string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, "UPDATE web_sessions SET last_seen_at = ? WHERE id = ?", now.Unix(), id)
	return err
}

func (s *SQLiteStore) CreateEmailVerification(ctx context.Context, ev *domain.EmailVerification) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO email_verifications (
			id, user_id, email, token_hash, purpose, created_at, expires_at, consumed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, ev.ID, ev.UserID, ev.Email, ev.TokenHash, ev.Purpose, ev.CreatedAt.Unix(), ev.ExpiresAt.Unix(), nullableUnix(ev.ConsumedAt))
	return err
}

func (s *SQLiteStore) GetEmailVerificationByTokenHash(ctx context.Context, tokenHash string) (*domain.EmailVerification, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, email, token_hash, purpose, created_at, expires_at, consumed_at
		FROM email_verifications WHERE token_hash = ?
	`, tokenHash)
	return scanEmailVerification(row)
}

func (s *SQLiteStore) ConsumeEmailVerification(ctx context.Context, id string, consumedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE email_verifications SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL
	`, consumedAt.Unix(), id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) DeletePendingEmailVerifications(ctx context.Context, userID, purpose string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM email_verifications WHERE user_id = ? AND purpose = ? AND consumed_at IS NULL
	`, userID, purpose)
	return err
}

func (s *SQLiteStore) CountEmailVerificationsSince(ctx context.Context, userID, purpose string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM email_verifications WHERE user_id = ? AND purpose = ? AND created_at >= ?
	`, userID, purpose, since.Unix()).Scan(&count)
	return count, err
}

func (s *SQLiteStore) LastEmailVerification(ctx context.Context, userID, purpose string) (*domain.EmailVerification, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, email, token_hash, purpose, created_at, expires_at, consumed_at
		FROM email_verifications WHERE user_id = ? AND purpose = ?
		ORDER BY created_at DESC LIMIT 1
	`, userID, purpose)
	return scanEmailVerification(row)
}

func scanWebSessionAndUser(scanner interface{ Scan(...any) error }) (*domain.WebSession, *domain.User, error) {
	var (
		sessionID, sessionUserID, tokenHash     string
		sessionCreatedAt, lastSeenAt, expiresAt int64

		userID, email, userName, passwordHash, userStatus, userSurface, boundAccountID string
		referralCode, referredByUserID                                                 string
		userCreatedAt                                                                  int64
		emailVerifiedAt, lastLoginAt                                                   sql.NullInt64
	)
	err := scanner.Scan(
		&sessionID, &sessionUserID, &tokenHash, &sessionCreatedAt, &lastSeenAt, &expiresAt,
		&userID, &email, &userName, &passwordHash, &emailVerifiedAt, &userStatus,
		&userSurface, &boundAccountID, &referralCode, &referredByUserID,
		&userCreatedAt, &lastLoginAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	session := &domain.WebSession{
		ID:         sessionID,
		UserID:     sessionUserID,
		TokenHash:  tokenHash,
		CreatedAt:  time.Unix(sessionCreatedAt, 0).UTC(),
		LastSeenAt: time.Unix(lastSeenAt, 0).UTC(),
		ExpiresAt:  time.Unix(expiresAt, 0).UTC(),
	}
	user := &domain.User{
		ID:               userID,
		Email:            email,
		Name:             userName,
		PasswordHash:     passwordHash,
		Status:           userStatus,
		AllowedSurface:   domain.NormalizeSurface(userSurface),
		BoundAccountID:   boundAccountID,
		ReferralCode:     referralCode,
		ReferredByUserID: referredByUserID,
		CreatedAt:        time.Unix(userCreatedAt, 0).UTC(),
		EmailVerifiedAt:  scanNullableTime(emailVerifiedAt),
		LastLoginAt:      scanNullableTime(lastLoginAt),
	}
	if user.AllowedSurface == "" {
		user.AllowedSurface = domain.SurfaceNative
	}
	return session, user, nil
}

func scanEmailVerification(scanner interface{ Scan(...any) error }) (*domain.EmailVerification, error) {
	var (
		id, userID, email, tokenHash, purpose string
		createdAt, expiresAt                  int64
		consumedAt                            sql.NullInt64
	)
	err := scanner.Scan(&id, &userID, &email, &tokenHash, &purpose, &createdAt, &expiresAt, &consumedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &domain.EmailVerification{
		ID:         id,
		UserID:     userID,
		Email:      email,
		TokenHash:  tokenHash,
		Purpose:    purpose,
		CreatedAt:  time.Unix(createdAt, 0).UTC(),
		ExpiresAt:  time.Unix(expiresAt, 0).UTC(),
		ConsumedAt: scanNullableTime(consumedAt),
	}, nil
}
