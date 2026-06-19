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
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    allowed_surface TEXT NOT NULL DEFAULT 'native',
    daily_budget_micros INTEGER NOT NULL DEFAULT 0,
    monthly_budget_micros INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id, created_at);

CREATE TABLE IF NOT EXISTS web_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_web_sessions_expires ON web_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_web_sessions_user ON web_sessions(user_id, expires_at);

CREATE TABLE IF NOT EXISTS email_verifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    purpose TEXT NOT NULL DEFAULT 'signup',
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_email_verifications_user ON email_verifications(user_id, purpose, created_at);

CREATE TABLE IF NOT EXISTS security_events (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    ip_hash TEXT NOT NULL,
    email_hash TEXT NOT NULL DEFAULT '',
    success INTEGER NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_security_events_kind_ip_created ON security_events(kind, ip_hash, created_at);
CREATE INDEX IF NOT EXISTS idx_security_events_kind_email_created ON security_events(kind, email_hash, created_at);

CREATE TABLE IF NOT EXISTS billing_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admission_limits (
    scope TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    max_concurrent INTEGER NOT NULL DEFAULT 0,
    requests_per_minute INTEGER NOT NULL DEFAULT 0,
    min_balance_micros INTEGER NOT NULL DEFAULT 1,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (scope, scope_id)
);

CREATE TABLE IF NOT EXISTS model_prices (
    model TEXT PRIMARY KEY,
    input_micros_per_million INTEGER NOT NULL,
    output_micros_per_million INTEGER NOT NULL,
    cache_read_micros_per_million INTEGER NOT NULL DEFAULT 0,
    cache_create_micros_per_million INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS billing_ledger (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    amount_micros INTEGER NOT NULL,
    kind TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    price_snapshot_json TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_billing_ledger_user_seq ON billing_ledger(user_id, seq);
CREATE INDEX IF NOT EXISTS idx_billing_ledger_user_created ON billing_ledger(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_billing_ledger_source ON billing_ledger(source_type, source_id);

CREATE TABLE IF NOT EXISTS billing_balance_checkpoints (
    user_id TEXT PRIMARY KEY,
    ledger_seq INTEGER NOT NULL,
    balance_micros INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS payment_orders (
    id TEXT PRIMARY KEY,
    out_trade_no TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    gateway TEXT NOT NULL DEFAULT 'zpay',
    status TEXT NOT NULL,
    product_name TEXT NOT NULL,
    amount_cny_fen INTEGER NOT NULL,
    credit_micros INTEGER NOT NULL,
    exchange_rate_micros INTEGER NOT NULL,
    payment_type TEXT NOT NULL DEFAULT 'alipay',
    zpay_trade_no TEXT NOT NULL DEFAULT '',
    qrcode TEXT NOT NULL DEFAULT '',
    qr_image TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    paid_at INTEGER,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_payment_orders_user_created ON payment_orders(user_id, created_at);

CREATE TABLE IF NOT EXISTS payment_events (
    id TEXT PRIMARY KEY,
    order_id TEXT NOT NULL,
    gateway TEXT NOT NULL,
    event_type TEXT NOT NULL,
    valid_signature INTEGER NOT NULL,
    payload_json TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS referrals (
    id TEXT PRIMARY KEY,
    inviter_user_id TEXT NOT NULL,
    invitee_user_id TEXT NOT NULL UNIQUE,
    invite_code TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    credited_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS billable_requests (
    request_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    api_key_id TEXT NOT NULL,
    model TEXT NOT NULL,
    surface TEXT NOT NULL,
    status TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_create_tokens INTEGER NOT NULL DEFAULT 0,
    price_snapshot_json TEXT NOT NULL DEFAULT '',
    ledger_id TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    usage_observed_at INTEGER,
    settled_at INTEGER
);

CREATE TABLE IF NOT EXISTS request_log (
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

CREATE TABLE IF NOT EXISTS user_route_bindings (
    user_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    surface TEXT NOT NULL,
    account_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, provider, surface)
);

CREATE INDEX IF NOT EXISTS idx_user_route_bindings_account ON user_route_bindings(account_id, last_used_at DESC);

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
