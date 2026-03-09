package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/yansir/cc-relayer/internal/domain"
)

const accountCols = `id, email, provider, status, schedulable, priority, priority_mode, error_message,
	refresh_token_enc, access_token_enc, expires_at, created_at,
	last_used_at, last_refresh_at, proxy_json, ext_info_json,
	five_hour_status, five_hour_util, five_hour_reset, seven_day_util, seven_day_reset,
	opus_rate_limit_end_at, overloaded_at, overloaded_until, rate_limited_at,
	codex_primary_util, codex_primary_reset, codex_secondary_util, codex_secondary_reset,
	subject, provider_state_json`

func scanAccount(scanner interface{ Scan(...any) error }) (*domain.Account, error) {
	var (
		id, email, provider, status, priMode, errMsg string
		refreshEnc, accessEnc                        string
		proxyJSON, extInfoJSON, fhStatus             string
		sched, prio                                  int
		expiresAt, createdAt                         int64
		lastUsedAt, lastRefreshAt                    sql.NullInt64
		fhUtil, sdUtil                               float64
		fhReset, sdReset                             int64
		opusEnd, olAt, olUntil, rlAt                 sql.NullInt64
		cpUtil, csUtil                               float64
		cpReset, csReset                             int64
		subject, providerStateJSON                   string
	)
	err := scanner.Scan(
		&id, &email, &provider, &status, &sched, &prio, &priMode, &errMsg,
		&refreshEnc, &accessEnc, &expiresAt, &createdAt,
		&lastUsedAt, &lastRefreshAt, &proxyJSON, &extInfoJSON,
		&fhStatus, &fhUtil, &fhReset, &sdUtil, &sdReset,
		&opusEnd, &olAt, &olUntil, &rlAt,
		&cpUtil, &cpReset, &csUtil, &csReset,
		&subject, &providerStateJSON,
	)
	if err != nil {
		return nil, err
	}

	if priMode == "" {
		priMode = "auto"
	}
	if provider == "" {
		provider = "claude"
	}

	a := &domain.Account{
		ID:              id,
		Email:           email,
		Provider:        domain.Provider(provider),
		Status:          domain.Status(status),
		Schedulable:     sched != 0,
		Priority:        prio,
		PriorityMode:    priMode,
		ErrorMessage:    errMsg,
		RefreshTokenEnc: refreshEnc,
		AccessTokenEnc:  accessEnc,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Unix(createdAt, 0).UTC(),
		LastUsedAt:      scanNullableTime(lastUsedAt),
		LastRefreshAt:   scanNullableTime(lastRefreshAt),
		ProxyJSON:       proxyJSON,
		ExtInfoJSON:     extInfoJSON,
		FiveHourStatus:  fhStatus,
		FiveHourUtil:    fhUtil,
		FiveHourReset:   fhReset,
		SevenDayUtil:    sdUtil,
		SevenDayReset:   sdReset,
		OpusRateLimitEndAt: scanNullableTime(opusEnd),
		OverloadedAt:       scanNullableTime(olAt),
		OverloadedUntil:    scanNullableTime(olUntil),
		RateLimitedAt:      scanNullableTime(rlAt),
		CodexPrimaryUtil:    cpUtil,
		CodexPrimaryReset:   cpReset,
		CodexSecondaryUtil:  csUtil,
		CodexSecondaryReset: csReset,
		Subject:             subject,
		ProviderStateJSON:   providerStateJSON,
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
			id, email, provider, status, schedulable, priority, priority_mode, error_message,
			refresh_token_enc, access_token_enc, expires_at, created_at,
			last_used_at, last_refresh_at, proxy_json, ext_info_json,
			five_hour_status, five_hour_util, five_hour_reset, seven_day_util, seven_day_reset,
			opus_rate_limit_end_at, overloaded_at, overloaded_until, rate_limited_at,
			codex_primary_util, codex_primary_reset, codex_secondary_util, codex_secondary_reset,
			subject, provider_state_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email=excluded.email, provider=excluded.provider, status=excluded.status,
			schedulable=excluded.schedulable, priority=excluded.priority, priority_mode=excluded.priority_mode,
			error_message=excluded.error_message,
			refresh_token_enc=excluded.refresh_token_enc, access_token_enc=excluded.access_token_enc,
			expires_at=excluded.expires_at,
			last_used_at=excluded.last_used_at, last_refresh_at=excluded.last_refresh_at,
			proxy_json=excluded.proxy_json, ext_info_json=excluded.ext_info_json,
			five_hour_status=excluded.five_hour_status,
			five_hour_util=excluded.five_hour_util, five_hour_reset=excluded.five_hour_reset,
			seven_day_util=excluded.seven_day_util, seven_day_reset=excluded.seven_day_reset,
			opus_rate_limit_end_at=excluded.opus_rate_limit_end_at,
			overloaded_at=excluded.overloaded_at, overloaded_until=excluded.overloaded_until,
			rate_limited_at=excluded.rate_limited_at,
			codex_primary_util=excluded.codex_primary_util, codex_primary_reset=excluded.codex_primary_reset,
			codex_secondary_util=excluded.codex_secondary_util, codex_secondary_reset=excluded.codex_secondary_reset,
			subject=excluded.subject, provider_state_json=excluded.provider_state_json`,
		acct.ID, acct.Email, string(acct.Provider), string(acct.Status),
		boolInt(acct.Schedulable), acct.Priority, acct.PriorityMode, acct.ErrorMessage,
		acct.RefreshTokenEnc, acct.AccessTokenEnc, acct.ExpiresAt, acct.CreatedAt.Unix(),
		nullableUnix(acct.LastUsedAt), nullableUnix(acct.LastRefreshAt),
		acct.ProxyJSON, acct.ExtInfoJSON,
		acct.FiveHourStatus, acct.FiveHourUtil, acct.FiveHourReset,
		acct.SevenDayUtil, acct.SevenDayReset,
		nullableUnix(acct.OpusRateLimitEndAt), nullableUnix(acct.OverloadedAt),
		nullableUnix(acct.OverloadedUntil), nullableUnix(acct.RateLimitedAt),
		acct.CodexPrimaryUtil, acct.CodexPrimaryReset,
		acct.CodexSecondaryUtil, acct.CodexSecondaryReset,
		acct.Subject, acct.ProviderStateJSON,
	)
	return err
}

func (s *SQLiteStore) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", id)
	return err
}
