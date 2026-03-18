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

func TestMigrate_LegacyUsersTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-users.db")

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
			cost_usd REAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		);
		CREATE TABLE quota_buckets (
			bucket_key TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			cooldown_until INTEGER,
			state_json TEXT NOT NULL DEFAULT '{}',
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE session_bindings (
			session_uuid TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE stainless_bindings (
			account_id TEXT PRIMARY KEY,
			headers_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE oauth_sessions (
			session_id TEXT PRIMARY KEY,
			data_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE refresh_locks (
			account_id TEXT PRIMARY KEY,
			lock_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)
	`); err != nil {
		t.Fatalf("create legacy users schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO users (id, name, token_hash, token_prefix, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "u-1", "legacy-user", "hash", "tk_legacy_abcd...", "active", time.Now().Add(-time.Hour).Unix()); err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}

	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer store.Close()

	cols, err := store.tableColumns(context.Background(), "users")
	if err != nil {
		t.Fatalf("tableColumns(users): %v", err)
	}
	if !hasColumns(cols, "allowed_surface", "bound_account_id") {
		t.Fatalf("migrated users columns missing policy fields: %v", cols)
	}

	users, err := store.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers(): %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("len(users) = %d, want 1", len(users))
	}
	if users[0].AllowedSurface != domain.SurfaceNative {
		t.Fatalf("AllowedSurface = %q, want native", users[0].AllowedSurface)
	}
	if users[0].BoundAccountID != "" {
		t.Fatalf("BoundAccountID = %q, want empty", users[0].BoundAccountID)
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
			allowed_surface TEXT NOT NULL DEFAULT 'native',
			bound_account_id TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			last_active_at INTEGER
		);
		CREATE TABLE request_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT '',
			surface TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			path TEXT NOT NULL DEFAULT '',
			cell_id TEXT NOT NULL DEFAULT '',
			bucket_key TEXT NOT NULL DEFAULT '',
			request_meta_json TEXT NOT NULL DEFAULT '{}',
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			upstream_headers_json TEXT NOT NULL DEFAULT '{}',
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create_tokens INTEGER NOT NULL DEFAULT 0,
			upstream_error_type TEXT NOT NULL DEFAULT '',
			session_uuid TEXT NOT NULL DEFAULT '',
			request_bytes INTEGER NOT NULL DEFAULT 0,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			client_headers_json TEXT NOT NULL DEFAULT '{}',
			upstream_request_id TEXT NOT NULL DEFAULT '',
			upstream_status INTEGER NOT NULL DEFAULT 0,
			binding_source TEXT NOT NULL DEFAULT '',
			upstream_error_message TEXT NOT NULL DEFAULT '',
			effect_kind TEXT NOT NULL DEFAULT '',
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
		);
		CREATE TABLE session_bindings (
			session_uuid TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE stainless_bindings (
			account_id TEXT PRIMARY KEY,
			headers_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE oauth_sessions (
			session_id TEXT PRIMARY KEY,
			data_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE refresh_locks (
			account_id TEXT PRIMARY KEY,
			lock_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
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

func TestSessionBindings_RoundTripAndPurge(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session-bindings.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	active := &domain.SessionBinding{
		SessionUUID: "sess-active",
		AccountID:   "acct-1",
		CreatedAt:   now.Add(-2 * time.Minute),
		LastUsedAt:  now.Add(-1 * time.Minute),
		ExpiresAt:   now.Add(10 * time.Minute),
	}
	expired := &domain.SessionBinding{
		SessionUUID: "sess-expired",
		AccountID:   "acct-1",
		CreatedAt:   now.Add(-20 * time.Minute),
		LastUsedAt:  now.Add(-15 * time.Minute),
		ExpiresAt:   now.Add(-1 * time.Minute),
	}

	if err := store.SaveSessionBinding(context.Background(), active); err != nil {
		t.Fatalf("SaveSessionBinding(active): %v", err)
	}
	if err := store.SaveSessionBinding(context.Background(), expired); err != nil {
		t.Fatalf("SaveSessionBinding(expired): %v", err)
	}

	got, err := store.GetSessionBinding(context.Background(), active.SessionUUID)
	if err != nil {
		t.Fatalf("GetSessionBinding(active): %v", err)
	}
	if got == nil || got.AccountID != active.AccountID {
		t.Fatalf("GetSessionBinding(active) = %#v, want account %q", got, active.AccountID)
	}

	gotExpired, err := store.GetSessionBinding(context.Background(), expired.SessionUUID)
	if err != nil {
		t.Fatalf("GetSessionBinding(expired): %v", err)
	}
	if gotExpired != nil {
		t.Fatalf("GetSessionBinding(expired) = %#v, want nil", gotExpired)
	}

	list, err := store.ListSessionBindingsByAccount(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("ListSessionBindingsByAccount(): %v", err)
	}
	if len(list) != 1 || list[0].SessionUUID != active.SessionUUID {
		t.Fatalf("ListSessionBindingsByAccount() = %#v, want only %q", list, active.SessionUUID)
	}

	purged, err := store.PurgeExpiredSessionBindings(context.Background(), now)
	if err != nil {
		t.Fatalf("PurgeExpiredSessionBindings(): %v", err)
	}
	if purged != 1 {
		t.Fatalf("PurgeExpiredSessionBindings() purged %d, want 1", purged)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	store, err = New(dbPath)
	if err != nil {
		t.Fatalf("New() after reopen: %v", err)
	}
	defer store.Close()

	got, err = store.GetSessionBinding(context.Background(), active.SessionUUID)
	if err != nil {
		t.Fatalf("GetSessionBinding(active) after reopen: %v", err)
	}
	if got == nil || got.SessionUUID != active.SessionUUID {
		t.Fatalf("GetSessionBinding(active) after reopen = %#v, want %q", got, active.SessionUUID)
	}

	if err := store.DeleteSessionBinding(context.Background(), active.SessionUUID); err != nil {
		t.Fatalf("DeleteSessionBinding(): %v", err)
	}
	got, err = store.GetSessionBinding(context.Background(), active.SessionUUID)
	if err != nil {
		t.Fatalf("GetSessionBinding(active) after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("GetSessionBinding(active) after delete = %#v, want nil", got)
	}
}

func TestStainlessBindings_SetNXAndPurge(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stainless-bindings.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	first := &domain.StainlessBinding{
		AccountID:   "acct-1",
		HeadersJSON: `{"x-stainless-os":"MacOS"}`,
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	second := &domain.StainlessBinding{
		AccountID:   "acct-1",
		HeadersJSON: `{"x-stainless-os":"Linux"}`,
		CreatedAt:   now.Add(1 * time.Minute),
		ExpiresAt:   now.Add(24*time.Hour + time.Minute),
	}
	expiredReplacement := &domain.StainlessBinding{
		AccountID:   "acct-1",
		HeadersJSON: `{"x-stainless-os":"Linux"}`,
		CreatedAt:   now.Add(2 * time.Hour),
		ExpiresAt:   now.Add(26 * time.Hour),
	}

	ok, err := store.SetStainlessBindingNX(context.Background(), first)
	if err != nil {
		t.Fatalf("SetStainlessBindingNX(first): %v", err)
	}
	if !ok {
		t.Fatal("SetStainlessBindingNX(first) = false, want true")
	}

	ok, err = store.SetStainlessBindingNX(context.Background(), second)
	if err != nil {
		t.Fatalf("SetStainlessBindingNX(second): %v", err)
	}
	if ok {
		t.Fatal("SetStainlessBindingNX(second) = true, want false while binding is active")
	}

	got, err := store.GetStainlessBinding(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("GetStainlessBinding(active): %v", err)
	}
	if got == nil || got.HeadersJSON != first.HeadersJSON {
		t.Fatalf("GetStainlessBinding(active) = %#v, want %q", got, first.HeadersJSON)
	}

	purged, err := store.PurgeExpiredStainlessBindings(context.Background(), now.Add(25*time.Hour))
	if err != nil {
		t.Fatalf("PurgeExpiredStainlessBindings(): %v", err)
	}
	if purged != 1 {
		t.Fatalf("PurgeExpiredStainlessBindings() purged %d, want 1", purged)
	}

	ok, err = store.SetStainlessBindingNX(context.Background(), expiredReplacement)
	if err != nil {
		t.Fatalf("SetStainlessBindingNX(expiredReplacement): %v", err)
	}
	if !ok {
		t.Fatal("SetStainlessBindingNX(expiredReplacement) = false, want true after expiry")
	}

	got, err = store.GetStainlessBinding(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("GetStainlessBinding(replaced): %v", err)
	}
	if got == nil || got.HeadersJSON != expiredReplacement.HeadersJSON {
		t.Fatalf("GetStainlessBinding(replaced) = %#v, want %q", got, expiredReplacement.HeadersJSON)
	}

	if err := store.DeleteStainlessBinding(context.Background(), "acct-1"); err != nil {
		t.Fatalf("DeleteStainlessBinding(): %v", err)
	}
	got, err = store.GetStainlessBinding(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("GetStainlessBinding(after delete): %v", err)
	}
	if got != nil {
		t.Fatalf("GetStainlessBinding(after delete) = %#v, want nil", got)
	}
}

func TestOAuthSessions_GetDeleteAndPurge(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "oauth-sessions.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	active := &domain.OAuthSessionState{
		SessionID: "sess-active",
		DataJSON:  `{"provider":"claude"}`,
		CreatedAt: now,
		ExpiresAt: now.Add(10 * time.Minute),
	}
	expired := &domain.OAuthSessionState{
		SessionID: "sess-expired",
		DataJSON:  `{"provider":"gemini"}`,
		CreatedAt: now.Add(-20 * time.Minute),
		ExpiresAt: now.Add(-10 * time.Minute),
	}

	if err := store.SaveOAuthSession(context.Background(), active); err != nil {
		t.Fatalf("SaveOAuthSession(active): %v", err)
	}
	if err := store.SaveOAuthSession(context.Background(), expired); err != nil {
		t.Fatalf("SaveOAuthSession(expired): %v", err)
	}

	got, err := store.GetAndDeleteOAuthSession(context.Background(), active.SessionID)
	if err != nil {
		t.Fatalf("GetAndDeleteOAuthSession(active): %v", err)
	}
	if got == nil || got.DataJSON != active.DataJSON {
		t.Fatalf("GetAndDeleteOAuthSession(active) = %#v, want %q", got, active.DataJSON)
	}

	got, err = store.GetAndDeleteOAuthSession(context.Background(), active.SessionID)
	if err != nil {
		t.Fatalf("GetAndDeleteOAuthSession(active second read): %v", err)
	}
	if got != nil {
		t.Fatalf("GetAndDeleteOAuthSession(active second read) = %#v, want nil", got)
	}

	got, err = store.GetAndDeleteOAuthSession(context.Background(), expired.SessionID)
	if err != nil {
		t.Fatalf("GetAndDeleteOAuthSession(expired): %v", err)
	}
	if got != nil {
		t.Fatalf("GetAndDeleteOAuthSession(expired) = %#v, want nil", got)
	}

	purged, err := store.PurgeExpiredOAuthSessions(context.Background(), now)
	if err != nil {
		t.Fatalf("PurgeExpiredOAuthSessions(): %v", err)
	}
	if purged != 1 {
		t.Fatalf("PurgeExpiredOAuthSessions() purged %d, want 1", purged)
	}
}

func TestRefreshLocks_AcquireReleaseAndPurge(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "refresh-locks.db")
	if err := Migrate(dbPath); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	first := &domain.RefreshLock{
		AccountID: "acct-1",
		LockID:    "lock-1",
		CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Second),
	}
	second := &domain.RefreshLock{
		AccountID: "acct-1",
		LockID:    "lock-2",
		CreatedAt: now.Add(1 * time.Second),
		ExpiresAt: now.Add(31 * time.Second),
	}
	replacement := &domain.RefreshLock{
		AccountID: "acct-1",
		LockID:    "lock-3",
		CreatedAt: now.Add(1 * time.Minute),
		ExpiresAt: now.Add(90 * time.Second),
	}

	ok, err := store.AcquireRefreshLock(context.Background(), first)
	if err != nil {
		t.Fatalf("AcquireRefreshLock(first): %v", err)
	}
	if !ok {
		t.Fatal("AcquireRefreshLock(first) = false, want true")
	}

	ok, err = store.AcquireRefreshLock(context.Background(), second)
	if err != nil {
		t.Fatalf("AcquireRefreshLock(second): %v", err)
	}
	if ok {
		t.Fatal("AcquireRefreshLock(second) = true, want false while first lock is active")
	}

	if err := store.ReleaseRefreshLock(context.Background(), "acct-1", "wrong-lock"); err != nil {
		t.Fatalf("ReleaseRefreshLock(wrong-lock): %v", err)
	}

	ok, err = store.AcquireRefreshLock(context.Background(), second)
	if err != nil {
		t.Fatalf("AcquireRefreshLock(second after wrong release): %v", err)
	}
	if ok {
		t.Fatal("AcquireRefreshLock(second after wrong release) = true, want false")
	}

	if err := store.ReleaseRefreshLock(context.Background(), "acct-1", "lock-1"); err != nil {
		t.Fatalf("ReleaseRefreshLock(lock-1): %v", err)
	}

	ok, err = store.AcquireRefreshLock(context.Background(), second)
	if err != nil {
		t.Fatalf("AcquireRefreshLock(second after release): %v", err)
	}
	if !ok {
		t.Fatal("AcquireRefreshLock(second after release) = false, want true")
	}

	purged, err := store.PurgeExpiredRefreshLocks(context.Background(), now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("PurgeExpiredRefreshLocks(): %v", err)
	}
	if purged != 1 {
		t.Fatalf("PurgeExpiredRefreshLocks() purged %d, want 1", purged)
	}

	ok, err = store.AcquireRefreshLock(context.Background(), replacement)
	if err != nil {
		t.Fatalf("AcquireRefreshLock(replacement): %v", err)
	}
	if !ok {
		t.Fatal("AcquireRefreshLock(replacement) = false, want true after purge")
	}
}
