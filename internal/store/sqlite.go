package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// SQLiteStore implements Store using SQLite for persistence.
type SQLiteStore struct {
	db            *sql.DB
	cleanupCancel context.CancelFunc
}

// New creates a SQLiteStore and initializes the current schema.
func New(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	_, cancel := context.WithCancel(context.Background())
	s := &SQLiteStore{
		db:            db,
		cleanupCancel: cancel,
	}
	if err := s.migrateAccountsTable(context.Background()); err != nil {
		db.Close()
		cancel()
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *SQLiteStore) Close() error                   { s.cleanupCancel(); return s.db.Close() }

func nullableUnix(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func scanNullableTime(v sql.NullInt64) *time.Time {
	if !v.Valid || v.Int64 == 0 {
		return nil
	}
	t := time.Unix(v.Int64, 0).UTC()
	return &t
}

var desiredAccountColumns = []string{
	"id",
	"email",
	"provider",
	"status",
	"priority",
	"priority_mode",
	"error_message",
	"refresh_token_enc",
	"access_token_enc",
	"expires_at",
	"created_at",
	"last_used_at",
	"last_refresh_at",
	"proxy_json",
	"identity_json",
	"cooldown_until",
	"subject",
	"provider_state_json",
}

func (s *SQLiteStore) migrateAccountsTable(ctx context.Context) error {
	cols, err := s.tableColumns(ctx, "accounts")
	if err != nil {
		return fmt.Errorf("inspect accounts schema: %w", err)
	}
	if slices.Equal(cols, desiredAccountColumns) {
		return nil
	}
	if !hasColumns(cols, "subject", "provider_state_json") {
		return fmt.Errorf("accounts migration: unsupported legacy schema %v", cols)
	}

	identitySource := firstPresent(cols, "identity_json", "meta_json", "ext_info_json")
	if identitySource == "" {
		return fmt.Errorf("accounts migration: missing identity column in %v", cols)
	}

	cooldownSource := "cooldown_until"
	if !slices.Contains(cols, cooldownSource) {
		cooldownSource = "overloaded_until"
	}
	if !slices.Contains(cols, cooldownSource) {
		return fmt.Errorf("accounts migration: missing cooldown source column in %v", cols)
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
			refresh_token_enc TEXT NOT NULL DEFAULT '',
			access_token_enc TEXT NOT NULL DEFAULT '',
			expires_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER,
			last_refresh_at INTEGER,
			proxy_json TEXT NOT NULL DEFAULT '',
			identity_json TEXT NOT NULL DEFAULT '',
			cooldown_until INTEGER,
			subject TEXT NOT NULL,
			provider_state_json TEXT NOT NULL DEFAULT '{}',
			UNIQUE(provider, subject)
		)
	`); err != nil {
		return fmt.Errorf("create accounts_new: %w", err)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO accounts_new (
			id, email, provider, status, priority, priority_mode, error_message,
			refresh_token_enc, access_token_enc, expires_at, created_at,
			last_used_at, last_refresh_at, proxy_json, identity_json,
			cooldown_until, subject, provider_state_json
		)
		SELECT
			id,
			email,
			provider,
			status,
			priority,
			COALESCE(NULLIF(priority_mode, ''), 'auto'),
			error_message,
			refresh_token_enc,
			access_token_enc,
			expires_at,
			created_at,
			last_used_at,
			last_refresh_at,
			proxy_json,
			%s,
			%s,
			subject,
			COALESCE(NULLIF(provider_state_json, ''), '{}')
		FROM accounts
	`, identitySource, cooldownSource)
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

func firstPresent(cols []string, names ...string) string {
	for _, name := range names {
		if slices.Contains(cols, name) {
			return name
		}
	}
	return ""
}
