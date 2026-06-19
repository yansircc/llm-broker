package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

func (s *SQLiteStore) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	allowedSurface := key.AllowedSurface
	if allowedSurface == "" {
		allowedSurface = domain.SurfaceNative
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_keys (
			id, user_id, name, token_hash, token_prefix, status,
			allowed_surface, daily_budget_micros, monthly_budget_micros,
			created_at, last_used_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		key.ID, key.UserID, key.Name, key.TokenHash, key.TokenPrefix, key.Status,
		string(allowedSurface), key.DailyBudgetMicros, key.MonthlyBudgetMicros,
		key.CreatedAt.Unix(), nullableUnix(key.LastUsedAt))
	return err
}

func (s *SQLiteStore) GetAPIKey(ctx context.Context, id string) (*domain.APIKey, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, token_hash, token_prefix, status,
			allowed_surface, daily_budget_micros, monthly_budget_micros,
			created_at, last_used_at
		FROM api_keys WHERE id = ?
	`, id)
	return scanAPIKey(row)
}

func (s *SQLiteStore) GetAPIKeyByTokenHash(ctx context.Context, tokenHash string) (*domain.APIKey, *domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			k.id, k.user_id, k.name, k.token_hash, k.token_prefix, k.status,
			k.allowed_surface, k.daily_budget_micros, k.monthly_budget_micros,
			k.created_at, k.last_used_at,
			u.id, u.email, u.name, u.password_hash, u.email_verified_at, u.status,
			u.allowed_surface, u.bound_account_id, u.referral_code, u.referred_by_user_id,
			u.created_at, u.last_login_at
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.token_hash = ?
	`, tokenHash)
	return scanAPIKeyAndUser(row)
}

func (s *SQLiteStore) ListAPIKeysByUser(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, token_hash, token_prefix, status,
			allowed_surface, daily_budget_micros, monthly_budget_micros,
			created_at, last_used_at
		FROM api_keys WHERE user_id = ? ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []*domain.APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *SQLiteStore) DeleteAPIKey(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM api_keys WHERE id = ?", id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) UpdateAPIKey(ctx context.Context, key *domain.APIKey) error {
	if key == nil {
		return nil
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_keys
		SET name = ?, status = ?, allowed_surface = ?,
			daily_budget_micros = ?, monthly_budget_micros = ?
		WHERE id = ? AND user_id = ?
	`,
		key.Name, key.Status, string(key.AllowedSurface), key.DailyBudgetMicros, key.MonthlyBudgetMicros,
		key.ID, key.UserID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) UpdateAPIKeyStatus(ctx context.Context, id, status string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE api_keys SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (s *SQLiteStore) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE api_keys SET last_used_at = ? WHERE id = ?", time.Now().Unix(), id)
	return err
}

func scanAPIKey(scanner interface{ Scan(...any) error }) (*domain.APIKey, error) {
	var (
		id, userID, name, tokenHash, tokenPrefix, status, allowedSurface string
		dailyBudget, monthlyBudget                                       int64
		createdAt                                                        int64
		lastUsedAt                                                       sql.NullInt64
	)
	err := scanner.Scan(
		&id, &userID, &name, &tokenHash, &tokenPrefix, &status,
		&allowedSurface, &dailyBudget, &monthlyBudget, &createdAt, &lastUsedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	key := &domain.APIKey{
		ID:                  id,
		UserID:              userID,
		Name:                name,
		TokenHash:           tokenHash,
		TokenPrefix:         tokenPrefix,
		Status:              status,
		AllowedSurface:      domain.NormalizeSurface(allowedSurface),
		DailyBudgetMicros:   dailyBudget,
		MonthlyBudgetMicros: monthlyBudget,
		CreatedAt:           time.Unix(createdAt, 0).UTC(),
		LastUsedAt:          scanNullableTime(lastUsedAt),
	}
	if key.AllowedSurface == "" {
		key.AllowedSurface = domain.SurfaceNative
	}
	return key, nil
}

func scanAPIKeyAndUser(scanner interface{ Scan(...any) error }) (*domain.APIKey, *domain.User, error) {
	var (
		keyID, keyUserID, keyName, tokenHash, tokenPrefix, keyStatus, keySurface string
		keyCreatedAt                                                             int64
		keyDailyBudget, keyMonthlyBudget                                         int64
		keyLastUsedAt                                                            sql.NullInt64

		userID, email, userName, passwordHash, userStatus, userSurface, boundAccountID string
		referralCode, referredByUserID                                                 string
		userCreatedAt                                                                  int64
		emailVerifiedAt, lastLoginAt                                                   sql.NullInt64
	)
	err := scanner.Scan(
		&keyID, &keyUserID, &keyName, &tokenHash, &tokenPrefix, &keyStatus,
		&keySurface, &keyDailyBudget, &keyMonthlyBudget, &keyCreatedAt, &keyLastUsedAt,
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
	key := &domain.APIKey{
		ID:                  keyID,
		UserID:              keyUserID,
		Name:                keyName,
		TokenHash:           tokenHash,
		TokenPrefix:         tokenPrefix,
		Status:              keyStatus,
		AllowedSurface:      domain.NormalizeSurface(keySurface),
		DailyBudgetMicros:   keyDailyBudget,
		MonthlyBudgetMicros: keyMonthlyBudget,
		CreatedAt:           time.Unix(keyCreatedAt, 0).UTC(),
		LastUsedAt:          scanNullableTime(keyLastUsedAt),
	}
	if key.AllowedSurface == "" {
		key.AllowedSurface = domain.SurfaceNative
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
	return key, user, nil
}
