package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"
)

const accountCols = `id, email, provider, status, priority, priority_mode, error_message,
	refresh_token_enc, access_token_enc, expires_at, created_at,
	last_used_at, last_refresh_at, proxy_json, identity_json,
	cooldown_until, subject, provider_state_json`

func scanAccount(scanner interface{ Scan(...any) error }) (*domain.Account, error) {
	var (
		id, email, provider, status, priMode, errMsg string
		refreshEnc, accessEnc                        string
		proxyJSON, identityJSON                      string
		prio                                         int
		expiresAt, createdAt                         int64
		lastUsedAt, lastRefreshAt                    sql.NullInt64
		cooldownUntil                                sql.NullInt64
		subject, providerStateJSON                   string
	)
	err := scanner.Scan(
		&id, &email, &provider, &status, &prio, &priMode, &errMsg,
		&refreshEnc, &accessEnc, &expiresAt, &createdAt,
		&lastUsedAt, &lastRefreshAt, &proxyJSON, &identityJSON,
		&cooldownUntil, &subject, &providerStateJSON,
	)
	if err != nil {
		return nil, err
	}

	if priMode == "" {
		priMode = "auto"
	}
	if provider == "" {
		return nil, fmt.Errorf("account %s missing provider", id)
	}

	a := &domain.Account{
		ID:                id,
		Email:             email,
		Provider:          domain.Provider(provider),
		Status:            domain.Status(status),
		Priority:          prio,
		PriorityMode:      priMode,
		ErrorMessage:      errMsg,
		RefreshTokenEnc:   refreshEnc,
		AccessTokenEnc:    accessEnc,
		ExpiresAt:         expiresAt,
		CreatedAt:         time.Unix(createdAt, 0).UTC(),
		LastUsedAt:        scanNullableTime(lastUsedAt),
		LastRefreshAt:     scanNullableTime(lastRefreshAt),
		ProxyJSON:         proxyJSON,
		IdentityJSON:      identityJSON,
		CooldownUntil:     scanNullableTime(cooldownUntil),
		Subject:           subject,
		ProviderStateJSON: providerStateJSON,
	}
	a.HydrateRuntime()
	return a, nil
}

func (s *SQLiteStore) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE id = ?", id)
	a, err := scanAccount(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (s *SQLiteStore) ListAccounts(ctx context.Context) ([]*domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+accountCols+" FROM accounts ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []*domain.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// SaveAccount performs an UPSERT of the entire Account struct.
func (s *SQLiteStore) SaveAccount(ctx context.Context, acct *domain.Account) error {
	acct.PersistRuntime()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO accounts (
			id, email, provider, status, priority, priority_mode, error_message,
			refresh_token_enc, access_token_enc, expires_at, created_at,
			last_used_at, last_refresh_at, proxy_json, identity_json,
			cooldown_until, subject, provider_state_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email=excluded.email, provider=excluded.provider, status=excluded.status,
			priority=excluded.priority, priority_mode=excluded.priority_mode,
			error_message=excluded.error_message,
			refresh_token_enc=excluded.refresh_token_enc, access_token_enc=excluded.access_token_enc,
			expires_at=excluded.expires_at,
			last_used_at=excluded.last_used_at, last_refresh_at=excluded.last_refresh_at,
			proxy_json=excluded.proxy_json, identity_json=excluded.identity_json,
			cooldown_until=excluded.cooldown_until,
			subject=excluded.subject, provider_state_json=excluded.provider_state_json`,
		acct.ID, acct.Email, string(acct.Provider), string(acct.Status),
		acct.Priority, acct.PriorityMode, acct.ErrorMessage,
		acct.RefreshTokenEnc, acct.AccessTokenEnc, acct.ExpiresAt, acct.CreatedAt.Unix(),
		nullableUnix(acct.LastUsedAt), nullableUnix(acct.LastRefreshAt),
		acct.ProxyJSON, acct.IdentityJSON,
		nullableUnix(acct.CooldownUntil),
		acct.Subject, acct.ProviderStateJSON,
	)
	return err
}

func (s *SQLiteStore) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", id)
	return err
}
