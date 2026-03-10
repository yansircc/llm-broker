package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite for persistence.
type SQLiteStore struct {
	db            *sql.DB
	cleanupCancel context.CancelFunc
}

// New opens a SQLiteStore and requires the current schema to already exist.
func New(dbPath string) (*SQLiteStore, error) {
	db, err := openSQLite(dbPath)
	if err != nil {
		return nil, err
	}

	_, cancel := context.WithCancel(context.Background())
	s := &SQLiteStore{
		db:            db,
		cleanupCancel: cancel,
	}
	if err := s.validateCurrentSchema(context.Background()); err != nil {
		db.Close()
		cancel()
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *SQLiteStore) Close() error                   { s.cleanupCancel(); return s.db.Close() }

func openSQLite(dbPath string) (*sql.DB, error) {
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

	return db, nil
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
