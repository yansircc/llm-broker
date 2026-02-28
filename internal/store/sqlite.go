package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
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

// New creates a SQLiteStore, initializes the schema, and runs migrations.
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

	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		cancel()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// migrate adds columns that may not exist in older databases.
func (s *SQLiteStore) migrate(ctx context.Context) error {
	migrations := []struct {
		table, column, ddl string
	}{
		{"request_log", "cost_usd", "ALTER TABLE request_log ADD COLUMN cost_usd REAL NOT NULL DEFAULT 0"},
		{"accounts", "priority_mode", "ALTER TABLE accounts ADD COLUMN priority_mode TEXT NOT NULL DEFAULT 'auto'"},
		{"accounts", "email", "ALTER TABLE accounts RENAME COLUMN name TO email"},
		{"accounts", "five_hour_util", "ALTER TABLE accounts ADD COLUMN five_hour_util REAL NOT NULL DEFAULT 0"},
		{"accounts", "five_hour_reset", "ALTER TABLE accounts ADD COLUMN five_hour_reset INTEGER NOT NULL DEFAULT 0"},
		{"accounts", "seven_day_util", "ALTER TABLE accounts ADD COLUMN seven_day_util REAL NOT NULL DEFAULT 0"},
		{"accounts", "seven_day_reset", "ALTER TABLE accounts ADD COLUMN seven_day_reset INTEGER NOT NULL DEFAULT 0"},
		{"accounts", "provider", "ALTER TABLE accounts ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude'"},
		{"accounts", "codex_primary_util", "ALTER TABLE accounts ADD COLUMN codex_primary_util REAL NOT NULL DEFAULT 0"},
		{"accounts", "codex_primary_reset", "ALTER TABLE accounts ADD COLUMN codex_primary_reset INTEGER NOT NULL DEFAULT 0"},
		{"accounts", "codex_secondary_util", "ALTER TABLE accounts ADD COLUMN codex_secondary_util REAL NOT NULL DEFAULT 0"},
		{"accounts", "codex_secondary_reset", "ALTER TABLE accounts ADD COLUMN codex_secondary_reset INTEGER NOT NULL DEFAULT 0"},
	}
	for _, m := range migrations {
		if !s.columnExists(ctx, m.table, m.column) {
			if _, err := s.db.ExecContext(ctx, m.ddl); err != nil {
				return fmt.Errorf("add %s.%s: %w", m.table, m.column, err)
			}
		}
	}
	return nil
}

func (s *SQLiteStore) columnExists(ctx context.Context, table, column string) bool {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func (s *SQLiteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *SQLiteStore) Close() error                    { s.cleanupCancel(); return s.db.Close() }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

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
