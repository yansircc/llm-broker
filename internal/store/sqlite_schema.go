package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"slices"
	"strings"
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
	"name",
	"token_hash",
	"token_prefix",
	"status",
	"created_at",
	"last_active_at",
}

var desiredRequestLogColumns = []string{
	"id",
	"user_id",
	"account_id",
	"model",
	"input_tokens",
	"output_tokens",
	"cache_read_tokens",
	"cache_create_tokens",
	"cost_usd",
	"status",
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
		{table: "request_log", want: desiredRequestLogColumns},
		{table: "quota_buckets", want: desiredQuotaBucketColumns},
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

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM quota_buckets").Scan(&count); err != nil {
		return fmt.Errorf("count quota_buckets: %w", err)
	}
	if count > 0 {
		return nil
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

	seedSQL := fmt.Sprintf(`
		INSERT INTO quota_buckets (
			bucket_key, provider, cooldown_until, state_json, updated_at
		)
		SELECT
			%s,
			provider,
			%s,
			%s,
			strftime('%%s', 'now')
		FROM accounts
		`, bucketKeyExpr, cooldownExpr, stateExpr)
	if _, err := s.db.ExecContext(ctx, seedSQL); err != nil {
		return fmt.Errorf("seed quota_buckets from accounts: %w", err)
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
