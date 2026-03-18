CREATE TABLE IF NOT EXISTS accounts (
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
);

CREATE TABLE IF NOT EXISTS egress_cells (
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

CREATE TABLE IF NOT EXISTS users (
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

CREATE TABLE IF NOT EXISTS request_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    surface TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    cell_id TEXT NOT NULL DEFAULT '',
    bucket_key TEXT NOT NULL DEFAULT '',
    session_uuid TEXT NOT NULL DEFAULT '',
    binding_source TEXT NOT NULL DEFAULT '',
    client_headers_json TEXT NOT NULL DEFAULT '{}',
    client_body_excerpt TEXT NOT NULL DEFAULT '',
    request_meta_json TEXT NOT NULL DEFAULT '{}',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_create_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    effect_kind TEXT NOT NULL DEFAULT '',
    upstream_status INTEGER NOT NULL DEFAULT 0,
    upstream_url TEXT NOT NULL DEFAULT '',
    upstream_request_headers_json TEXT NOT NULL DEFAULT '{}',
    upstream_request_meta_json TEXT NOT NULL DEFAULT '{}',
    upstream_request_body_excerpt TEXT NOT NULL DEFAULT '',
    upstream_request_id TEXT NOT NULL DEFAULT '',
    upstream_headers_json TEXT NOT NULL DEFAULT '{}',
    upstream_response_meta_json TEXT NOT NULL DEFAULT '{}',
    upstream_response_body_excerpt TEXT NOT NULL DEFAULT '',
    upstream_error_type TEXT NOT NULL DEFAULT '',
    upstream_error_message TEXT NOT NULL DEFAULT '',
    request_bytes INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_log_created ON request_log(created_at);
CREATE INDEX IF NOT EXISTS idx_request_log_user ON request_log(user_id, created_at);

CREATE TABLE IF NOT EXISTS quota_buckets (
    bucket_key TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    cooldown_until INTEGER,
    state_json TEXT NOT NULL DEFAULT '{}',
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS session_bindings (
    session_uuid TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_bindings_account ON session_bindings(account_id, last_used_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_bindings_expires ON session_bindings(expires_at);

CREATE TABLE IF NOT EXISTS stainless_bindings (
    account_id TEXT PRIMARY KEY,
    headers_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_stainless_bindings_expires ON stainless_bindings(expires_at);

CREATE TABLE IF NOT EXISTS oauth_sessions (
    session_id TEXT PRIMARY KEY,
    data_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oauth_sessions_expires ON oauth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS refresh_locks (
    account_id TEXT PRIMARY KEY,
    lock_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_refresh_locks_expires ON refresh_locks(expires_at);
