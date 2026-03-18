package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const userRouteBindingCols = `user_id, provider, surface, account_id, created_at, last_used_at`

func scanUserRouteBinding(scanner interface{ Scan(...any) error }) (*domain.UserRouteBinding, error) {
	var (
		userID     string
		provider   string
		surface    string
		accountID  string
		createdAt  int64
		lastUsedAt int64
	)
	if err := scanner.Scan(&userID, &provider, &surface, &accountID, &createdAt, &lastUsedAt); err != nil {
		return nil, err
	}
	return &domain.UserRouteBinding{
		UserID:     userID,
		Provider:   domain.Provider(provider),
		Surface:    domain.NormalizeSurface(surface),
		AccountID:  accountID,
		CreatedAt:  time.Unix(createdAt, 0).UTC(),
		LastUsedAt: time.Unix(lastUsedAt, 0).UTC(),
	}, nil
}

func (s *SQLiteStore) GetUserRouteBinding(ctx context.Context, userID string, provider domain.Provider, surface domain.Surface) (*domain.UserRouteBinding, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+userRouteBindingCols+" FROM user_route_bindings WHERE user_id = ? AND provider = ? AND surface = ?",
		userID, string(provider), string(domain.NormalizeSurface(string(surface))),
	)
	binding, err := scanUserRouteBinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return binding, err
}

func (s *SQLiteStore) SaveUserRouteBinding(ctx context.Context, binding *domain.UserRouteBinding) error {
	createdAt := binding.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	lastUsedAt := binding.LastUsedAt.UTC()
	if lastUsedAt.IsZero() {
		lastUsedAt = createdAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_route_bindings (
			user_id, provider, surface, account_id, created_at, last_used_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, provider, surface) DO UPDATE SET
			account_id=excluded.account_id,
			created_at=excluded.created_at,
			last_used_at=excluded.last_used_at
	`, binding.UserID, string(binding.Provider), string(domain.NormalizeSurface(string(binding.Surface))), binding.AccountID, createdAt.Unix(), lastUsedAt.Unix())
	return err
}

func (s *SQLiteStore) DeleteUserRouteBindingsByUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM user_route_bindings WHERE user_id = ?", userID)
	return err
}
