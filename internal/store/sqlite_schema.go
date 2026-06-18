package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"time"
)

//go:embed schema.sql
var schemaSQL string

var desiredAccountColumns = []string{
	"id",
	"email",
	"provider",
	"status",
	"priority",
	"priority_mode",
	"error_message",
	"bucket_key",
	"refresh_token_enc",
	"access_token_enc",
	"expires_at",
	"created_at",
	"last_used_at",
	"last_refresh_at",
	"proxy_json",
	"cell_id",
	"identity_json",
	"subject",
}

var desiredEgressCellColumns = []string{
	"id",
	"name",
	"status",
	"proxy_json",
	"labels_json",
	"cooldown_until",
	"state_json",
	"created_at",
	"updated_at",
}

var desiredUserColumns = []string{
	"id",
	"email",
	"name",
	"password_hash",
	"email_verified_at",
	"status",
	"allowed_surface",
	"bound_account_id",
	"referral_code",
	"referred_by_user_id",
	"created_at",
	"last_login_at",
}

var desiredAPIKeyColumns = []string{
	"id",
	"user_id",
	"name",
	"token_hash",
	"token_prefix",
	"status",
	"allowed_surface",
	"created_at",
	"last_used_at",
}

var desiredWebSessionColumns = []string{
	"id",
	"user_id",
	"token_hash",
	"created_at",
	"last_seen_at",
	"expires_at",
}

var desiredEmailVerificationColumns = []string{
	"id",
	"user_id",
	"email",
	"token_hash",
	"purpose",
	"created_at",
	"expires_at",
	"consumed_at",
}

var desiredBillingSettingColumns = []string{
	"key",
	"value",
	"updated_at",
}

var desiredAdmissionLimitColumns = []string{
	"scope",
	"scope_id",
	"max_concurrent",
	"requests_per_minute",
	"min_balance_micros",
	"updated_at",
}

var desiredModelPriceColumns = []string{
	"model",
	"input_micros_per_million",
	"output_micros_per_million",
	"cache_read_micros_per_million",
	"cache_create_micros_per_million",
	"updated_at",
}

var desiredBillingLedgerColumns = []string{
	"seq",
	"id",
	"user_id",
	"amount_micros",
	"kind",
	"source_type",
	"source_id",
	"idempotency_key",
	"description",
	"price_snapshot_json",
	"metadata_json",
	"created_at",
}

var desiredBillingBalanceCheckpointColumns = []string{
	"user_id",
	"ledger_seq",
	"balance_micros",
	"created_at",
}

var desiredPaymentOrderColumns = []string{
	"id",
	"out_trade_no",
	"user_id",
	"gateway",
	"status",
	"product_name",
	"amount_cny_fen",
	"credit_micros",
	"exchange_rate_micros",
	"payment_type",
	"zpay_trade_no",
	"qrcode",
	"qr_image",
	"created_at",
	"paid_at",
	"updated_at",
}

var desiredPaymentEventColumns = []string{
	"id",
	"order_id",
	"gateway",
	"event_type",
	"valid_signature",
	"payload_json",
	"created_at",
}

var desiredReferralColumns = []string{
	"id",
	"inviter_user_id",
	"invitee_user_id",
	"invite_code",
	"created_at",
	"credited_at",
}

var desiredBillableRequestColumns = []string{
	"request_id",
	"user_id",
	"api_key_id",
	"model",
	"surface",
	"status",
	"input_tokens",
	"output_tokens",
	"cache_read_tokens",
	"cache_create_tokens",
	"price_snapshot_json",
	"ledger_id",
	"error",
	"created_at",
	"usage_observed_at",
	"settled_at",
}

var desiredRequestLogColumns = []string{
	"id",
	"user_id",
	"request_id",
	"api_key_id",
	"account_id",
	"provider",
	"surface",
	"model",
	"cell_id",
	"input_tokens",
	"output_tokens",
	"cache_read_tokens",
	"cache_create_tokens",
	"cost_usd",
	"status",
	"effect_kind",
	"upstream_status",
	"upstream_error_type",
	"duration_ms",
	"created_at",
}

var desiredQuotaBucketColumns = []string{
	"bucket_key",
	"provider",
	"cooldown_until",
	"state_json",
	"updated_at",
}

var desiredSessionBindingColumns = []string{
	"session_uuid",
	"account_id",
	"created_at",
	"last_used_at",
	"expires_at",
}

var desiredUserRouteBindingColumns = []string{
	"user_id",
	"provider",
	"surface",
	"account_id",
	"created_at",
	"last_used_at",
}

var desiredStainlessBindingColumns = []string{
	"account_id",
	"headers_json",
	"created_at",
	"expires_at",
}

var desiredOAuthSessionColumns = []string{
	"session_id",
	"data_json",
	"created_at",
	"expires_at",
}

var desiredRefreshLockColumns = []string{
	"account_id",
	"lock_id",
	"created_at",
	"expires_at",
}

// Migrate creates or upgrades the database to the current schema.
func Migrate(dbPath string) error {
	db, err := openSQLite(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrateQuotaBucketsTable(context.Background()); err != nil {
		return err
	}
	if err := s.migrateAccountsTable(context.Background()); err != nil {
		return err
	}
	if err := s.migrateUsersTable(context.Background()); err != nil {
		return err
	}
	if err := s.migrateRequestLogTable(context.Background()); err != nil {
		return err
	}
	if err := s.ensureRequestLogIndexes(context.Background()); err != nil {
		return err
	}
	if err := s.seedCommercialDefaults(context.Background()); err != nil {
		return err
	}
	if err := s.validateCurrentSchema(context.Background()); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) validateCurrentSchema(ctx context.Context) error {
	checks := []struct {
		table string
		want  []string
	}{
		{table: "accounts", want: desiredAccountColumns},
		{table: "egress_cells", want: desiredEgressCellColumns},
		{table: "users", want: desiredUserColumns},
		{table: "api_keys", want: desiredAPIKeyColumns},
		{table: "web_sessions", want: desiredWebSessionColumns},
		{table: "email_verifications", want: desiredEmailVerificationColumns},
		{table: "billing_settings", want: desiredBillingSettingColumns},
		{table: "admission_limits", want: desiredAdmissionLimitColumns},
		{table: "model_prices", want: desiredModelPriceColumns},
		{table: "billing_ledger", want: desiredBillingLedgerColumns},
		{table: "billing_balance_checkpoints", want: desiredBillingBalanceCheckpointColumns},
		{table: "payment_orders", want: desiredPaymentOrderColumns},
		{table: "payment_events", want: desiredPaymentEventColumns},
		{table: "referrals", want: desiredReferralColumns},
		{table: "billable_requests", want: desiredBillableRequestColumns},
		{table: "request_log", want: desiredRequestLogColumns},
		{table: "quota_buckets", want: desiredQuotaBucketColumns},
		{table: "session_bindings", want: desiredSessionBindingColumns},
		{table: "user_route_bindings", want: desiredUserRouteBindingColumns},
		{table: "stainless_bindings", want: desiredStainlessBindingColumns},
		{table: "oauth_sessions", want: desiredOAuthSessionColumns},
		{table: "refresh_locks", want: desiredRefreshLockColumns},
	}

	for _, check := range checks {
		cols, err := s.tableColumns(ctx, check.table)
		if err != nil {
			return fmt.Errorf("inspect %s schema: %w", check.table, err)
		}
		if sameColumns(cols, check.want) {
			continue
		}
		if len(cols) == 0 {
			return fmt.Errorf("database schema missing table %q; run `llm-broker migrate`", check.table)
		}
		return fmt.Errorf("database schema for %q is not current; run `llm-broker migrate`", check.table)
	}

	return nil
}

func (s *SQLiteStore) migrateAccountsTable(ctx context.Context) error {
	cols, err := s.tableColumns(ctx, "accounts")
	if err != nil {
		return fmt.Errorf("inspect accounts schema: %w", err)
	}
	if sameColumns(cols, desiredAccountColumns) {
		return nil
	}
	if !hasColumns(cols, "subject") {
		return fmt.Errorf("accounts migration: missing subject column in %v", cols)
	}

	identitySource := firstPresent(cols, "identity_json", "meta_json", "ext_info_json")
	if identitySource == "" {
		return fmt.Errorf("accounts migration: missing identity column in %v", cols)
	}
	cellIDExpr := "''"
	if slices.Contains(cols, "cell_id") {
		cellIDExpr = "cell_id"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin accounts migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE accounts_new (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			provider TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'created',
			priority INTEGER NOT NULL DEFAULT 50,
			priority_mode TEXT NOT NULL DEFAULT 'auto',
			error_message TEXT NOT NULL DEFAULT '',
			bucket_key TEXT NOT NULL DEFAULT '',
			refresh_token_enc TEXT NOT NULL DEFAULT '',
			access_token_enc TEXT NOT NULL DEFAULT '',
			expires_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER,
			last_refresh_at INTEGER,
			proxy_json TEXT NOT NULL DEFAULT '',
			cell_id TEXT NOT NULL DEFAULT '',
			identity_json TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL,
			UNIQUE(provider, subject)
		)
	`); err != nil {
		return fmt.Errorf("create accounts_new: %w", err)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO accounts_new (
			id, email, provider, status, priority, priority_mode, error_message,
			bucket_key,
			refresh_token_enc, access_token_enc, expires_at, created_at,
			last_used_at, last_refresh_at, proxy_json, cell_id, identity_json,
			subject
		)
		SELECT
			id,
			email,
			provider,
			status,
			priority,
			COALESCE(NULLIF(priority_mode, ''), 'auto'),
			error_message,
			CASE
				WHEN subject != '' THEN provider || ':' || subject
				ELSE provider || ':' || id
			END,
			refresh_token_enc,
			access_token_enc,
			expires_at,
			created_at,
			last_used_at,
			last_refresh_at,
			proxy_json,
			%s,
			%s,
			subject
		FROM accounts
	`, cellIDExpr, identitySource)
	if _, err := tx.ExecContext(ctx, insertSQL); err != nil {
		return fmt.Errorf("copy accounts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE accounts`); err != nil {
		return fmt.Errorf("drop old accounts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE accounts_new RENAME TO accounts`); err != nil {
		return fmt.Errorf("rename accounts_new: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit accounts migration: %w", err)
	}
	return nil
}

func (s *SQLiteStore) migrateQuotaBucketsTable(ctx context.Context) error {
	cols, err := s.tableColumns(ctx, "quota_buckets")
	if err != nil {
		return fmt.Errorf("inspect quota_buckets schema: %w", err)
	}
	if len(cols) == 0 {
		return fmt.Errorf("quota_buckets table missing after schema creation")
	}
	if !sameColumns(cols, desiredQuotaBucketColumns) {
		return fmt.Errorf("quota_buckets migration: unsupported schema %v", cols)
	}

	accountCols, err := s.tableColumns(ctx, "accounts")
	if err != nil {
		return fmt.Errorf("inspect accounts schema for bucket seed: %w", err)
	}

	cooldownExpr := "NULL"
	switch {
	case slices.Contains(accountCols, "cooldown_until"):
		cooldownExpr = "cooldown_until"
	case slices.Contains(accountCols, "overloaded_until"):
		cooldownExpr = "overloaded_until"
	}

	stateExpr := "'{}'"
	if slices.Contains(accountCols, "provider_state_json") {
		stateExpr = "COALESCE(NULLIF(provider_state_json, ''), '{}')"
	}

	bucketKeyExpr := "CASE WHEN subject != '' THEN provider || ':' || subject ELSE provider || ':' || id END"
	if slices.Contains(accountCols, "bucket_key") {
		bucketKeyExpr = "CASE WHEN bucket_key != '' THEN bucket_key WHEN subject != '' THEN provider || ':' || subject ELSE provider || ':' || id END"
	}

	syncSQL := fmt.Sprintf(`
		INSERT OR IGNORE INTO quota_buckets (
			bucket_key, provider, cooldown_until, state_json, updated_at
		)
		SELECT
			DISTINCT
			%s,
			provider,
			%s,
			%s,
			strftime('%%s', 'now')
		FROM accounts
		`, bucketKeyExpr, cooldownExpr, stateExpr)
	if _, err := s.db.ExecContext(ctx, syncSQL); err != nil {
		return fmt.Errorf("sync quota_buckets from accounts: %w", err)
	}

	pruneSQL := fmt.Sprintf(`
		DELETE FROM quota_buckets
		WHERE bucket_key NOT IN (
			SELECT DISTINCT %s FROM accounts
		)
	`, bucketKeyExpr)
	if _, err := s.db.ExecContext(ctx, pruneSQL); err != nil {
		return fmt.Errorf("delete orphan quota_buckets: %w", err)
	}
	return nil
}

func (s *SQLiteStore) migrateUsersTable(ctx context.Context) error {
	cols, err := s.tableColumns(ctx, "users")
	if err != nil {
		return fmt.Errorf("inspect users schema: %w", err)
	}
	if sameColumns(cols, desiredUserColumns) {
		return nil
	}
	if hasColumns(cols, "email", "password_hash", "referral_code") {
		return fmt.Errorf("users migration: unsupported schema %v", cols)
	}
	if !hasColumns(cols, "id", "name", "token_hash", "token_prefix", "status", "created_at") {
		return fmt.Errorf("users migration: unsupported schema %v", cols)
	}

	allowedSurfaceExpr := "'native'"
	if slices.Contains(cols, "allowed_surface") {
		allowedSurfaceExpr = "COALESCE(NULLIF(allowed_surface, ''), 'native')"
	}
	boundAccountExpr := "''"
	if slices.Contains(cols, "bound_account_id") {
		boundAccountExpr = "bound_account_id"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin users migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE users_new (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			email_verified_at INTEGER,
			status TEXT NOT NULL DEFAULT 'active',
			allowed_surface TEXT NOT NULL DEFAULT 'native',
			bound_account_id TEXT NOT NULL DEFAULT '',
			referral_code TEXT NOT NULL UNIQUE,
			referred_by_user_id TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			last_login_at INTEGER
		)
	`); err != nil {
		return fmt.Errorf("create users_new: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE api_keys_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			token_prefix TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			allowed_surface TEXT NOT NULL DEFAULT 'native',
			created_at INTEGER NOT NULL,
			last_used_at INTEGER
		)
	`); err != nil {
		return fmt.Errorf("create api_keys_new: %w", err)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO users_new (
			id, email, name, password_hash, email_verified_at, status, allowed_surface,
			bound_account_id, referral_code, referred_by_user_id, created_at, last_login_at
		)
		SELECT
			id,
			lower(replace(name, ' ', '_')) || '@local.invalid',
			name,
			'',
			created_at,
			status,
			%s,
			%s,
			'ref_' || replace(id, '-', ''),
			'',
			created_at,
			%s
		FROM users
	`, allowedSurfaceExpr, boundAccountExpr, firstPresentOr(cols, "last_active_at", "NULL"))
	if _, err := tx.ExecContext(ctx, insertSQL); err != nil {
		return fmt.Errorf("copy users: %w", err)
	}
	keySQL := fmt.Sprintf(`
		INSERT INTO api_keys_new (
			id, user_id, name, token_hash, token_prefix, status, allowed_surface, created_at, last_used_at
		)
		SELECT
			'key_' || replace(id, '-', ''),
			id,
			'legacy',
			token_hash,
			token_prefix,
			status,
			%s,
			created_at,
			%s
		FROM users
	`, allowedSurfaceExpr, firstPresentOr(cols, "last_active_at", "NULL"))
	if _, err := tx.ExecContext(ctx, keySQL); err != nil {
		return fmt.Errorf("copy api keys: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE users`); err != nil {
		return fmt.Errorf("drop old users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE users_new RENAME TO users`); err != nil {
		return fmt.Errorf("rename users_new: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS api_keys`); err != nil {
		return fmt.Errorf("drop old api_keys: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE api_keys_new RENAME TO api_keys`); err != nil {
		return fmt.Errorf("rename api_keys_new: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX idx_api_keys_user ON api_keys(user_id, created_at)`); err != nil {
		return fmt.Errorf("create idx_api_keys_user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit users migration: %w", err)
	}
	return nil
}

func (s *SQLiteStore) migrateRequestLogTable(ctx context.Context) error {
	cols, err := s.tableColumns(ctx, "request_log")
	if err != nil {
		return fmt.Errorf("inspect request_log schema: %w", err)
	}
	if sameColumns(cols, desiredRequestLogColumns) {
		return nil
	}
	if !hasColumns(cols, "id", "user_id", "account_id", "model", "status", "created_at") {
		return fmt.Errorf("request_log migration: unsupported schema %v", cols)
	}

	copyExpr := func(name, fallback string) string {
		if slices.Contains(cols, name) {
			return name
		}
		return fallback
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin request_log migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE request_log_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			request_id TEXT NOT NULL DEFAULT '',
			api_key_id TEXT NOT NULL DEFAULT '',
			account_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT '',
			surface TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			cell_id TEXT NOT NULL DEFAULT '',
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create_tokens INTEGER NOT NULL DEFAULT 0,
			cost_usd REAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			effect_kind TEXT NOT NULL DEFAULT '',
			upstream_status INTEGER NOT NULL DEFAULT 0,
			upstream_error_type TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create request_log_new: %w", err)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO request_log_new (
			id, user_id, request_id, api_key_id, account_id, provider, surface, model, cell_id,
			input_tokens, output_tokens, cache_read_tokens, cache_create_tokens, cost_usd,
			status, effect_kind, upstream_status, upstream_error_type,
			duration_ms, created_at
		)
		SELECT
			id,
			user_id,
			%s,
			%s,
			account_id,
			%s,
			%s,
			model,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			status,
			%s,
			%s,
			%s,
			%s,
			created_at
		FROM request_log
	`,
		copyExpr("request_id", "''"),
		copyExpr("api_key_id", "''"),
		copyExpr("provider", "''"),
		copyExpr("surface", "''"),
		copyExpr("cell_id", "''"),
		copyExpr("input_tokens", "0"),
		copyExpr("output_tokens", "0"),
		copyExpr("cache_read_tokens", "0"),
		copyExpr("cache_create_tokens", "0"),
		copyExpr("cost_usd", "0"),
		copyExpr("effect_kind", "''"),
		copyExpr("upstream_status", "0"),
		copyExpr("upstream_error_type", "''"),
		copyExpr("duration_ms", "0"),
	)
	if _, err := tx.ExecContext(ctx, insertSQL); err != nil {
		return fmt.Errorf("copy request_log: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE request_log`); err != nil {
		return fmt.Errorf("drop old request_log: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE request_log_new RENAME TO request_log`); err != nil {
		return fmt.Errorf("rename request_log_new: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX idx_request_log_created ON request_log(created_at)`); err != nil {
		return fmt.Errorf("create idx_request_log_created: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX idx_request_log_user ON request_log(user_id, created_at)`); err != nil {
		return fmt.Errorf("create idx_request_log_user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX idx_request_log_status ON request_log(status, created_at)`); err != nil {
		return fmt.Errorf("create idx_request_log_status: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX idx_request_log_cell ON request_log(cell_id, created_at)`); err != nil {
		return fmt.Errorf("create idx_request_log_cell: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit request_log migration: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ensureRequestLogIndexes(ctx context.Context) error {
	for name, stmt := range map[string]string{
		"idx_request_log_created": "CREATE INDEX IF NOT EXISTS idx_request_log_created ON request_log(created_at)",
		"idx_request_log_user":    "CREATE INDEX IF NOT EXISTS idx_request_log_user ON request_log(user_id, created_at)",
		"idx_request_log_status":  "CREATE INDEX IF NOT EXISTS idx_request_log_status ON request_log(status, created_at)",
		"idx_request_log_cell":    "CREATE INDEX IF NOT EXISTS idx_request_log_cell ON request_log(cell_id, created_at)",
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure %s: %w", name, err)
		}
	}
	return nil
}

func (s *SQLiteStore) seedCommercialDefaults(ctx context.Context) error {
	now := time.Now().Unix()
	settings := map[string]string{
		"cny_to_usd_rate_micros":         "1000000",
		"referral_new_user_bonus_micros": "0",
		"referral_inviter_bonus_micros":  "0",
	}
	for key, value := range settings {
		if _, err := s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO billing_settings (key, value, updated_at) VALUES (?, ?, ?)
		`, key, value, now); err != nil {
			return fmt.Errorf("seed billing setting %s: %w", key, err)
		}
	}

	limits := []struct {
		scope             string
		maxConcurrent     int
		requestsPerMinute int
		minBalanceMicros  int64
	}{
		{scope: "global", maxConcurrent: 0, requestsPerMinute: 0, minBalanceMicros: 1},
		{scope: "user", maxConcurrent: 0, requestsPerMinute: 0, minBalanceMicros: 1},
		{scope: "api_key", maxConcurrent: 0, requestsPerMinute: 0, minBalanceMicros: 1},
		{scope: "reward_only", maxConcurrent: 1, requestsPerMinute: 0, minBalanceMicros: 1},
	}
	for _, limit := range limits {
		if _, err := s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO admission_limits (
				scope, scope_id, max_concurrent, requests_per_minute, min_balance_micros, updated_at
			) VALUES (?, '', ?, ?, ?, ?)
		`, limit.scope, limit.maxConcurrent, limit.requestsPerMinute, limit.minBalanceMicros, now); err != nil {
			return fmt.Errorf("seed admission limit %s: %w", limit.scope, err)
		}
	}

	models := []string{
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
		"gpt-5.3-codex",
		"gpt-5.3-codex-spark",
		"gpt-5.2-codex",
		"gpt-5.2",
		"gpt-5.1-codex-max",
		"gpt-5.1-codex",
		"gpt-5.1-codex-mini",
		"gpt-5.1",
		"gpt-5-codex",
		"gpt-5-codex-mini",
		"gpt-5",
		"codex-1",
	}
	for _, model := range models {
		if _, err := s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO model_prices (
				model, input_micros_per_million, output_micros_per_million,
				cache_read_micros_per_million, cache_create_micros_per_million, updated_at
			) VALUES (?, 1000000, 5000000, 250000, 1000000, ?)
		`, model, now); err != nil {
			return fmt.Errorf("seed model price %s: %w", model, err)
		}
	}
	return nil
}

func (s *SQLiteStore) tableColumns(ctx context.Context, table string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, strings.ToLower(name))
	}
	return cols, rows.Err()
}

func hasColumns(cols []string, want ...string) bool {
	for _, col := range want {
		if !slices.Contains(cols, col) {
			return false
		}
	}
	return true
}

func sameColumns(cols, want []string) bool {
	return len(cols) == len(want) && hasColumns(cols, want...)
}

func firstPresent(cols []string, names ...string) string {
	for _, name := range names {
		if slices.Contains(cols, name) {
			return name
		}
	}
	return ""
}

func firstPresentOr(cols []string, name, fallback string) string {
	if slices.Contains(cols, name) {
		return name
	}
	return fallback
}
