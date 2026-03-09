package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestNew_MigratesLegacyAccountsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	t.Cleanup(func() { legacyDB.Close() })

	if _, err := legacyDB.Exec(`
		CREATE TABLE accounts (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			provider TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'created',
			schedulable INTEGER NOT NULL DEFAULT 1,
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
			ext_info_json TEXT NOT NULL DEFAULT '',
			five_hour_status TEXT NOT NULL DEFAULT '',
			five_hour_util REAL NOT NULL DEFAULT 0,
			five_hour_reset INTEGER NOT NULL DEFAULT 0,
			seven_day_util REAL NOT NULL DEFAULT 0,
			seven_day_reset INTEGER NOT NULL DEFAULT 0,
			opus_rate_limit_end_at INTEGER,
			overloaded_at INTEGER,
			overloaded_until INTEGER,
			rate_limited_at INTEGER,
			codex_primary_util REAL NOT NULL DEFAULT 0,
			codex_primary_reset INTEGER NOT NULL DEFAULT 0,
			codex_secondary_util REAL NOT NULL DEFAULT 0,
			codex_secondary_reset INTEGER NOT NULL DEFAULT 0,
			subject TEXT NOT NULL DEFAULT '',
			provider_state_json TEXT NOT NULL DEFAULT '{}'
		)
	`); err != nil {
		t.Fatalf("create legacy accounts: %v", err)
	}

	cooldownUntil := time.Now().Add(15 * time.Minute).Unix()
	if _, err := legacyDB.Exec(`
		INSERT INTO accounts (
			id, email, provider, status, schedulable, priority, priority_mode, error_message,
			refresh_token_enc, access_token_enc, expires_at, created_at,
			last_used_at, last_refresh_at, proxy_json, ext_info_json,
			five_hour_status, five_hour_util, five_hour_reset, seven_day_util, seven_day_reset,
			opus_rate_limit_end_at, overloaded_at, overloaded_until, rate_limited_at,
			codex_primary_util, codex_primary_reset, codex_secondary_util, codex_secondary_reset,
			subject, provider_state_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"acct-1", "acct@example.com", "claude", "active", 1, 80, "manual", "",
		"refresh", "access", time.Now().Add(time.Hour).UnixMilli(), time.Now().Add(-time.Hour).Unix(),
		nil, nil, "", `{"orgUUID":"org-1"}`,
		"", 0.4, 1_700_000_000, 0.1, 1_800_000_000,
		nil, nil, cooldownUntil, nil,
		0, 0, 0, 0,
		"org-1", `{"five_hour_util":0.4,"five_hour_reset":1700000000}`,
	); err != nil {
		t.Fatalf("insert legacy account: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() after legacy schema: %v", err)
	}
	defer store.Close()

	cols, err := store.tableColumns(context.Background(), "accounts")
	if err != nil {
		t.Fatalf("tableColumns(accounts): %v", err)
	}
	if !hasColumns(cols, "cooldown_until", "subject", "provider_state_json") {
		t.Fatalf("migrated columns missing expected fields: %v", cols)
	}
	if !hasColumns(cols, "identity_json") {
		t.Fatalf("migrated columns missing identity_json: %v", cols)
	}
	if hasColumns(cols, "schedulable", "overloaded_until", "five_hour_util", "codex_primary_util", "ext_info_json", "meta_json") {
		t.Fatalf("legacy columns still present after migration: %v", cols)
	}

	acct, err := store.GetAccount(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("GetAccount(): %v", err)
	}
	if acct == nil {
		t.Fatal("GetAccount() returned nil")
	}
	if acct.Subject != "org-1" {
		t.Fatalf("Subject = %q, want org-1", acct.Subject)
	}
	if acct.CooldownUntil == nil || acct.CooldownUntil.Unix() != cooldownUntil {
		t.Fatalf("CooldownUntil = %v, want unix %d", acct.CooldownUntil, cooldownUntil)
	}
	if acct.ProviderStateJSON == "" || acct.ProviderStateJSON == "{}" {
		t.Fatalf("ProviderStateJSON = %q, want preserved state", acct.ProviderStateJSON)
	}
}
