package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/yansircc/llm-broker/internal/domain"

	_ "modernc.org/sqlite"
)

func TestMigrate_LegacyAccountsTable(t *testing.T) {
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

	if _, err := New(dbPath); err == nil {
		t.Fatal("New() on legacy schema = nil, want explicit migrate failure")
	}

	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate() after legacy schema: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() after Migrate(): %v", err)
	}
	defer store.Close()

	cols, err := store.tableColumns(context.Background(), "accounts")
	if err != nil {
		t.Fatalf("tableColumns(accounts): %v", err)
	}
	if !hasColumns(cols, "bucket_key", "subject") {
		t.Fatalf("migrated columns missing expected fields: %v", cols)
	}
	if !hasColumns(cols, "identity_json", "cell_id") {
		t.Fatalf("migrated columns missing identity_json/cell_id: %v", cols)
	}
	if hasColumns(cols, "schedulable", "overloaded_until", "five_hour_util", "codex_primary_util", "ext_info_json", "meta_json", "cooldown_until", "provider_state_json") {
		t.Fatalf("legacy columns still present after migration: %v", cols)
	}

	cellCols, err := store.tableColumns(context.Background(), "egress_cells")
	if err != nil {
		t.Fatalf("tableColumns(egress_cells): %v", err)
	}
	if !hasColumns(cellCols, "proxy_json", "labels_json", "state_json") {
		t.Fatalf("egress_cells columns missing expected fields: %v", cellCols)
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
	if acct.BucketKey != "claude:org-1" {
		t.Fatalf("BucketKey = %q, want claude:org-1", acct.BucketKey)
	}

	bucket, err := store.GetQuotaBucket(context.Background(), "claude:org-1")
	if err != nil {
		t.Fatalf("GetQuotaBucket(): %v", err)
	}
	if bucket == nil {
		t.Fatal("GetQuotaBucket() returned nil")
	}
	if bucket.CooldownUntil == nil || bucket.CooldownUntil.Unix() != cooldownUntil {
		t.Fatalf("bucket CooldownUntil = %v, want unix %d", bucket.CooldownUntil, cooldownUntil)
	}
	if bucket.StateJSON == "" || bucket.StateJSON == "{}" {
		t.Fatalf("bucket StateJSON = %q, want preserved state", bucket.StateJSON)
	}
}

func TestNew_AllowsRequestLogColumnOrderDrift(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "order-drift.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE accounts (
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
			subject TEXT NOT NULL
		);
		CREATE TABLE egress_cells (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			proxy_json TEXT NOT NULL DEFAULT '',
			labels_json TEXT NOT NULL DEFAULT '',
			cooldown_until INTEGER,
			state_json TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			token_hash TEXT NOT NULL UNIQUE,
			token_prefix TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at INTEGER NOT NULL,
			last_active_at INTEGER
		);
		CREATE TABLE request_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			model TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create_tokens INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			cost_usd REAL NOT NULL DEFAULT 0
		);
		CREATE TABLE quota_buckets (
			bucket_key TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			cooldown_until INTEGER,
			state_json TEXT NOT NULL DEFAULT '{}',
			updated_at INTEGER NOT NULL
		)
	`); err != nil {
		t.Fatalf("create schema with request_log order drift: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() with request_log order drift: %v", err)
	}
	defer store.Close()
}

func TestSaveAccount_WritesCurrentSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "save-account.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer store.Close()

	acct := &domain.Account{
		ID:              "acct-1",
		Email:           "acct@example.com",
		Provider:        domain.ProviderClaude,
		Subject:         "org-1",
		Status:          domain.StatusActive,
		Priority:        50,
		PriorityMode:    "auto",
		RefreshTokenEnc: "refresh",
		AccessTokenEnc:  "access",
		ExpiresAt:       time.Now().Add(time.Hour).UnixMilli(),
		CreatedAt:       time.Now().UTC(),
		CellID:          "cell-fr-par-1",
	}

	if err := store.SaveAccount(context.Background(), acct); err != nil {
		t.Fatalf("SaveAccount(): %v", err)
	}

	saved, err := store.GetAccount(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("GetAccount(): %v", err)
	}
	if saved == nil || saved.CellID != "cell-fr-par-1" {
		t.Fatalf("saved CellID = %q, want cell-fr-par-1", saved.CellID)
	}
}

func TestSaveEgressCell_RoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "save-cell.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	cell := &domain.EgressCell{
		ID:        "cell-fr-par-1",
		Name:      "France / mark",
		Status:    domain.EgressCellActive,
		Proxy:     &domain.ProxyConfig{Type: "socks5", Host: "10.10.0.2", Port: 11081},
		Labels:    map[string]string{"country": "FR", "city": "Paris"},
		StateJSON: "{}",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.SaveEgressCell(context.Background(), cell); err != nil {
		t.Fatalf("SaveEgressCell(): %v", err)
	}

	saved, err := store.GetEgressCell(context.Background(), cell.ID)
	if err != nil {
		t.Fatalf("GetEgressCell(): %v", err)
	}
	if saved == nil {
		t.Fatal("GetEgressCell() returned nil")
	}
	if saved.Proxy == nil || saved.Proxy.Host != "10.10.0.2" {
		t.Fatalf("saved Proxy = %#v, want hydrated proxy", saved.Proxy)
	}
	if saved.Labels["city"] != "Paris" {
		t.Fatalf("saved Labels = %#v, want city=Paris", saved.Labels)
	}
}
