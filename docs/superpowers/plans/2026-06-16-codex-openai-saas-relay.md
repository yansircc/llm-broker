# Codex OpenAI SaaS Relay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn broker into a Codex/OpenAI-only public relay with customer accounts, API keys, RMB recharge through 7pay, USD-equivalent token billing, and signup referral rewards.

**Architecture:** Keep relay as the execution kernel and put all commercial state behind billing and admission boundaries. Codex remains the only upstream provider. Balance is derived from an append-only ledger with bounded checkpoints; payment orders, request logs, gateway events, and capacity telemetry are evidence, not balance source of truth.

**Tech Stack:** Go 1.24, SQLite, SvelteKit, standard `net/http`, `golang.org/x/crypto/bcrypt`, 7pay/zpay form API, existing request-log artifact storage.

---

## Document Shape

Use one plan document for this branch.

Reason: product contraction, identity, billing, payment, referral, and relay settlement share the same invariant:

```text
admit(request) = balance(user) > 0 AND capacity.Allow(user, key) AND abuse.Allow(user, key)
settle(success) = ledger += charge(actual_usage, price_snapshot)
balance(user) = latest_checkpoint.balance + SUM(ledger rows after checkpoint)
```

Splitting these into separate documents before this invariant is implemented would create duplicate truth across docs. Execution can still be phase-gated by task.

## Stable Axis, Change Axis, Invariant

Stable axis:

- `relay` executes an authenticated request through a provider driver and extracts usage.
- `driver` owns provider protocol.
- `request_log` records operational evidence.

Change axis:

- product surface becomes Codex/OpenAI only
- external users replace admin-created API users
- billing, payment, and referral become first-class product state

Invariant:

- Provider-specific behavior terminates at `driver.Driver`.
- Commercial state enters through `billing_ledger`.
- A successful billable request creates at most one debit ledger row keyed by request id.
- A paid 7pay order creates at most one credit ledger row keyed by order id.
- Invitation bonuses are ledger credits, not user fields.
- Public traffic must pass a capacity gate; token pricing is the customer retail unit, not the upstream cost model.

## Source-of-Truth Rules

- `billing_ledger.amount_micros` is the only balance truth. Balance checkpoints are a bounded read projection over ledger sequence, not a mutable balance column.
- `payment_orders.status` is payment state, not balance.
- `request_log.cost_usd` is operational cost evidence, not customer balance.
- `model_prices` is current customer retail price truth. Each debit ledger row stores a `price_snapshot_json`.
- RMB payment amount and RMB-to-USD-equivalent rate are snapshotted on the order.
- `api_keys.token_hash` authenticates relay requests. Users do not store API token hashes. API key hashes must be deterministic SHA-256 so indexed lookup is possible; bcrypt is only for passwords.
- `events` and logs are observability only.

## Unit Economics and Capacity

The upstream cost basis is not the same as public token billing. Codex accounts are subscription seats with rate-limit windows and account-risk cost, while customers are billed per token for product clarity.

Therefore token pricing is only the retail charging function. Admission must also enforce capacity:

- global concurrent request limit
- per-user concurrent request limit
- per-API-key request-per-minute limit
- stricter reward-only limits before first successful payment
- optional admin-set minimum balance for high-cost models

Capacity settings are part of business policy, not provider protocol. Provider-specific utilization still belongs in `driver`/`pool`; admission only consumes generic availability and configured customer limits.

## Public Product Surface

Canonical relay:

- `POST /openai/responses`

Compatibility surface:

- `GET /v1/models`
- `POST /v1/responses` as an alias into the same Codex relay path
- `POST /v1/chat/completions` for basic text/chat compatibility only

Compatibility constraints:

- All compat paths must settle through the same billing path as `/openai/responses`.
- Unsupported OpenAI chat parameters are rejected at the boundary with an OpenAI-shaped JSON error.
- Compat must not reintroduce Claude or Gemini model routing.

Admin surface:

- Existing admin token remains the operator root credential.
- Admin UI keeps provider/account operations only for Codex accounts.
- Admin billing pages manage users, API keys, prices, orders, ledger adjustments, and referral settings.

Customer surface:

- Register/login/logout
- Email verification and resend
- API key management
- Balance and usage
- Recharge QR flow
- Referral link and bonus history

## Data Model

All money-like values use integers.

```text
1 USD-equivalent credit = 1_000_000 credit_micros
1 RMB = 100 fen
```

### `users`

Rebuild existing `users` into customer identity.

Columns:

- `id TEXT PRIMARY KEY`
- `email TEXT NOT NULL UNIQUE`
- `name TEXT NOT NULL`
- `password_hash TEXT NOT NULL`
- `email_verified_at INTEGER`
- `status TEXT NOT NULL DEFAULT 'active'`
- `referral_code TEXT NOT NULL UNIQUE`
- `referred_by_user_id TEXT NOT NULL DEFAULT ''`
- `created_at INTEGER NOT NULL`
- `last_login_at INTEGER`

Existing admin-created users can be migrated by deriving `email` from `name` for local continuity. This branch has no external customer data yet, so the migration is allowed to be explicit and narrow.

### `api_keys`

Columns:

- `id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `name TEXT NOT NULL`
- `token_hash TEXT NOT NULL UNIQUE`
- `token_prefix TEXT NOT NULL`
- `status TEXT NOT NULL DEFAULT 'active'`
- `allowed_surface TEXT NOT NULL DEFAULT 'native'`
- `created_at INTEGER NOT NULL`
- `last_used_at INTEGER`

Indexes:

- `idx_api_keys_user ON api_keys(user_id, created_at)`

Hashing rule:

- `api_keys.token_hash = hex(SHA256(plaintext_api_key))`
- API keys are random high-entropy bearer tokens, so deterministic SHA-256 is acceptable and indexable.
- Do not use bcrypt for API keys; bcrypt is salted and cannot support equality lookup by token hash.

### `web_sessions`

Columns:

- `id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `token_hash TEXT NOT NULL UNIQUE`
- `created_at INTEGER NOT NULL`
- `last_seen_at INTEGER NOT NULL`
- `expires_at INTEGER NOT NULL`

Indexes:

- `idx_web_sessions_expires ON web_sessions(expires_at)`
- `idx_web_sessions_user ON web_sessions(user_id, expires_at)`

### `email_verifications`

Columns:

- `id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `email TEXT NOT NULL`
- `token_hash TEXT NOT NULL UNIQUE`
- `created_at INTEGER NOT NULL`
- `expires_at INTEGER NOT NULL`
- `consumed_at INTEGER`

Rules:

- Email verification is not part of the MVP product flow.
- Customer API key creation and relay admission do not require `users.email_verified_at`.
- Signup referral credits are split by trigger: invitee credit is fulfilled at registration; inviter credit is fulfilled only after the invitee has a successful paid order.
- `email_verifications` can remain as legacy schema until a later cleanup, but it must not gate registration, API keys, admission, or referral payout.
- Verification tokens are stored as `hex(SHA256(raw_token))`; raw tokens only appear in email links.
- MVP token expiry is 1 hour.

### `billing_settings`

Key-value table for mutable commercial settings.

Required keys:

- `cny_to_usd_rate_micros`: default `1000000`, meaning `1 RMB = 1 USD-equivalent`
- `referral_new_user_bonus_micros`
- `referral_inviter_bonus_micros`

### `admission_limits`

Configurable customer capacity limits.

Columns:

- `scope TEXT NOT NULL`
- `scope_id TEXT NOT NULL DEFAULT ''`
- `max_concurrent INTEGER NOT NULL DEFAULT 0`
- `requests_per_minute INTEGER NOT NULL DEFAULT 0`
- `min_balance_micros INTEGER NOT NULL DEFAULT 1`
- `updated_at INTEGER NOT NULL`
- `PRIMARY KEY (scope, scope_id)`

Required scopes:

- `global`
- `user`
- `api_key`
- `reward_only`

Default policy:

- paid users: admin-configured user/API-key limits
- reward-only users: `max_concurrent = 1`
- unverified users: no relay admission and no API key creation

This table expresses business capacity. It does not parse Codex rate-limit headers.

### `model_prices`

Current price table.

Columns:

- `model TEXT PRIMARY KEY`
- `input_micros_per_million INTEGER NOT NULL`
- `output_micros_per_million INTEGER NOT NULL`
- `cache_read_micros_per_million INTEGER NOT NULL DEFAULT 0`
- `cache_create_micros_per_million INTEGER NOT NULL DEFAULT 0`
- `updated_at INTEGER NOT NULL`

Ledger rows snapshot the matched price, so this table can update in place without rewriting history.

### `billing_ledger`

Append-only customer balance ledger.

Columns:

- `seq INTEGER PRIMARY KEY AUTOINCREMENT`
- `id TEXT NOT NULL UNIQUE`
- `user_id TEXT NOT NULL`
- `amount_micros INTEGER NOT NULL`
- `kind TEXT NOT NULL`
- `source_type TEXT NOT NULL`
- `source_id TEXT NOT NULL`
- `idempotency_key TEXT NOT NULL UNIQUE`
- `description TEXT NOT NULL DEFAULT ''`
- `price_snapshot_json TEXT NOT NULL DEFAULT ''`
- `metadata_json TEXT NOT NULL DEFAULT '{}'`
- `created_at INTEGER NOT NULL`

Allowed `kind`:

- `payment_credit`
- `referral_signup_credit`
- `usage_debit`
- `admin_adjustment`

Indexes:

- `idx_billing_ledger_user_seq ON billing_ledger(user_id, seq)`
- `idx_billing_ledger_user_created ON billing_ledger(user_id, created_at)`
- `idx_billing_ledger_source ON billing_ledger(source_type, source_id)`

### `billing_balance_checkpoints`

Derived bounded-read checkpoints over the append-only ledger.

Columns:

- `user_id TEXT PRIMARY KEY`
- `ledger_seq INTEGER NOT NULL`
- `balance_micros INTEGER NOT NULL`
- `created_at INTEGER NOT NULL`

Balance query:

```text
latest = billing_balance_checkpoints[user_id]
balance = latest.balance_micros + SUM(billing_ledger.amount_micros WHERE user_id=? AND seq > latest.ledger_seq)
```

The checkpoint is a derived projection. If it is deleted, balance is recomputed from ledger rows.

### `payment_orders`

Columns:

- `id TEXT PRIMARY KEY`
- `out_trade_no TEXT NOT NULL UNIQUE`
- `user_id TEXT NOT NULL`
- `gateway TEXT NOT NULL DEFAULT 'zpay'`
- `status TEXT NOT NULL`
- `product_name TEXT NOT NULL`
- `amount_cny_fen INTEGER NOT NULL`
- `credit_micros INTEGER NOT NULL`
- `exchange_rate_micros INTEGER NOT NULL`
- `payment_type TEXT NOT NULL DEFAULT 'alipay'`
- `zpay_trade_no TEXT NOT NULL DEFAULT ''`
- `qrcode TEXT NOT NULL DEFAULT ''`
- `qr_image TEXT NOT NULL DEFAULT ''`
- `created_at INTEGER NOT NULL`
- `paid_at INTEGER`
- `updated_at INTEGER NOT NULL`

Allowed `status`:

- `pending`
- `paid`
- `failed`
- `expired`

### `payment_events`

Observability table for gateway callbacks and active verification results.

Columns:

- `id TEXT PRIMARY KEY`
- `order_id TEXT NOT NULL`
- `gateway TEXT NOT NULL`
- `event_type TEXT NOT NULL`
- `valid_signature INTEGER NOT NULL`
- `payload_json TEXT NOT NULL`
- `created_at INTEGER NOT NULL`

This table is evidence only. It never drives balance.

### `referrals`

Columns:

- `id TEXT PRIMARY KEY`
- `inviter_user_id TEXT NOT NULL`
- `invitee_user_id TEXT NOT NULL UNIQUE`
- `invite_code TEXT NOT NULL`
- `created_at INTEGER NOT NULL`
- `credited_at INTEGER NOT NULL`

Referral fulfillment inserts two ledger rows in the same transaction as the referral record:

- `referral:new_user:<invitee_user_id>`
- `referral:inviter:<invitee_user_id>`

### `billable_requests`

Durable billing capture for relay requests.

Columns:

- `request_id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `api_key_id TEXT NOT NULL`
- `model TEXT NOT NULL`
- `surface TEXT NOT NULL`
- `status TEXT NOT NULL`
- `input_tokens INTEGER NOT NULL DEFAULT 0`
- `output_tokens INTEGER NOT NULL DEFAULT 0`
- `cache_read_tokens INTEGER NOT NULL DEFAULT 0`
- `cache_create_tokens INTEGER NOT NULL DEFAULT 0`
- `price_snapshot_json TEXT NOT NULL DEFAULT ''`
- `ledger_id TEXT NOT NULL DEFAULT ''`
- `created_at INTEGER NOT NULL`
- `usage_observed_at INTEGER`
- `settled_at INTEGER`

Allowed `status`:

- `in_progress`
- `usage_observed`
- `settled`
- `aborted_no_usage`
- `settlement_failed`

Rules:

- Insert `in_progress` before the upstream request starts.
- Persist final usage before forwarding the final stream completion event to the client.
- Settlement is idempotent by `request_id`.
- Startup reconciliation settles `usage_observed` rows that have no `ledger_id`.

### `request_log`

Keep the existing request log as operational evidence and add correlation fields:

- `request_id TEXT NOT NULL DEFAULT ''`
- `api_key_id TEXT NOT NULL DEFAULT ''`

Do not add balance fields to `request_log`.

## Packages and File Boundaries

Create:

- `internal/billing/types.go`: ledger, price, order-independent billing types
- `internal/billing/pricing.go`: integer cost calculation from usage and price snapshot
- `internal/billing/service.go`: balance, checkpoints, settlement, referral credits, admin adjustments
- `internal/billing/service_test.go`: ledger invariants and idempotency tests
- `internal/admission/service.go`: balance/capacity/abuse admission checks and runtime counters
- `internal/admission/service_test.go`: capacity, reward-only, and limit tests
- `internal/email/sender.go`: verification email sender interface, SMTP implementation, and stdout dev sender
- `internal/email/sender_test.go`: email rendering and secret-safe logging tests
- `internal/payments/zpay/sign.go`: 7pay MD5 signing and signature verification
- `internal/payments/zpay/client.go`: `mapi.php` create order and `api.php?act=order` active query
- `internal/payments/zpay/sign_test.go`: canonical sign/verify tests
- `internal/server/customer_auth.go`: register, login, logout, session auth
- `internal/server/customer_billing.go`: balance, orders, recharge, polling
- `internal/server/customer_keys.go`: customer API key CRUD
- `internal/server/admin_billing.go`: admin prices, settings, orders, ledger adjustments
- `internal/domain/api_key.go`
- `internal/domain/billing.go`
- `internal/domain/admission.go`
- `internal/domain/payment.go`
- `internal/domain/session.go`
- `internal/store/sqlite_api_keys.go`
- `internal/store/sqlite_admission.go`
- `internal/store/sqlite_billing.go`
- `internal/store/sqlite_payments.go`
- `internal/store/sqlite_sessions.go`
- `web/src/routes/app/*`: customer portal
- `web/src/routes/admin-billing/*`: admin billing pages

Modify:

- `cmd/relay/main.go`: register only Codex driver
- `internal/config/config.go`: add 7pay and session settings
- `internal/auth/auth.go`: authenticate admin token or API key rows
- `internal/relay/relay.go`: accept billing service dependency
- `internal/relay/relay_flow.go`: admit before upstream execution
- `internal/relay/relay_attempt.go`: settle successful usage once
- `internal/server/server.go`: wire billing and payment services
- `internal/server/server_routes.go`: public auth, customer, payment, and admin billing routes
- `internal/server/compat_openai_chat.go`: Codex-only compat or explicit unsupported errors
- `internal/store/schema.sql`: explicit schema
- `internal/store/sqlite_schema.go`: explicit migration
- `web/src/routes/users/*`: point admin user views at customer/API key model
- `web/src/routes/accounts/*`: Codex-only wording and filters

Delete or retire after compile is green:

- Claude/Gemini provider registration
- Claude/Gemini onboarding UI paths
- Claude/Gemini compat routing
- Claude/Gemini-specific tests that assert product support

Provider driver files can remain temporarily if tests still need compile scaffolding, but no runtime route, model catalog, or UI entry may expose them.

## Implementation Tasks

### Task 1: Lock Codex-Only Runtime Boundary

**Files:**

- Modify: `cmd/relay/main.go`
- Modify: `internal/server/server_routes.go`
- Modify: `internal/server/driver_views.go`
- Modify: `internal/server/compat_openai_chat.go`
- Modify: `internal/server/contract_test.go`
- Modify: `internal/server/compat_openai_chat_test.go`

- [ ] Add failing tests proving `/admin/providers`, `/v1/models`, `/openai/responses`, and compat routes expose only Codex/OpenAI models.
- [ ] Add failing tests proving Claude/Gemini add-account routes and compat model prefixes are rejected.
- [ ] Remove Claude/Gemini driver registration from `cmd/relay/main.go`.
- [ ] Make `handleListModels` return only Codex models.
- [ ] Replace Claude/Gemini compat resolver with Codex-only resolver.
- [ ] Run `go test ./internal/server ./cmd/relay -count=1`.
- [ ] Commit: `git commit -m "refactor: constrain relay runtime to codex"`

### Task 2: Split Customer Identity from API Keys

**Files:**

- Create: `internal/domain/api_key.go`
- Create: `internal/domain/session.go`
- Create: `internal/email/sender.go`
- Create: `internal/email/sender_test.go`
- Create: `internal/store/sqlite_api_keys.go`
- Create: `internal/store/sqlite_sessions.go`
- Create: `internal/server/customer_auth.go`
- Create: `internal/server/customer_keys.go`
- Modify: `internal/domain/user.go`
- Modify: `internal/auth/auth.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite_users.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`
- Modify: `internal/server/server_routes.go`
- Test: `internal/store/sqlite_migration_test.go`

- [ ] Add migration tests for new `users`, `api_keys`, `web_sessions`, and `email_verifications` schema.
- [ ] Add auth tests proving API key lookup returns `user_id`, `api_key_id`, surface, and status using deterministic SHA-256 token hashes.
- [ ] Add customer auth handler tests for register, login, logout, and session lookup.
- [ ] Implement bcrypt password hashing for customer login.
- [ ] Implement `POST /api/auth/register`, creating the user and fulfilling invitee-side referral signup credit when an invite code is present.
- [ ] Move relay token hashes from `users` into `api_keys`.
- [ ] Keep admin `API_TOKEN` validation unchanged for operator routes.
- [ ] Generate API keys as one-time plaintext values with SHA-256 hash and prefix stored.
- [ ] Allow API key creation for active logged-in customers without email verification.
- [ ] Run `go test ./internal/auth ./internal/server ./internal/store -count=1`.
- [ ] Commit: `git commit -m "feat: split customers and api keys"`

### Task 3: Add Billing Ledger and Pricing

**Files:**

- Create: `internal/domain/billing.go`
- Create: `internal/billing/types.go`
- Create: `internal/billing/pricing.go`
- Create: `internal/billing/service.go`
- Create: `internal/billing/service_test.go`
- Create: `internal/store/sqlite_billing.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`
- Test: `internal/store/sqlite_migration_test.go`

- [ ] Add tests proving balance is latest checkpoint plus ledger rows after the checkpoint.
- [ ] Add tests proving deleting a checkpoint recomputes the same balance from ledger rows.
- [ ] Add tests proving settlement is idempotent by request id.
- [ ] Add tests proving price updates do not change existing debit snapshot JSON.
- [ ] Implement integer usage pricing from `driver.Usage`.
- [ ] Implement `Credit`, `DebitUsage`, `Balance`, `WriteCheckpoint`, and `AdminAdjust`.
- [ ] Add `billable_requests` persistence helpers for `in_progress`, `usage_observed`, `settled`, and `settlement_failed`.
- [ ] Seed default Codex model prices in migration or explicit admin bootstrap.
- [ ] Run `go test ./internal/billing ./internal/store -count=1`.
- [ ] Commit: `git commit -m "feat: add billing ledger"`

### Task 4: Add Capacity and Abuse Admission Gates

**Files:**

- Create: `internal/domain/admission.go`
- Create: `internal/admission/service.go`
- Create: `internal/admission/service_test.go`
- Create: `internal/store/sqlite_admission.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`

- [ ] Add tests proving admission requires positive balance, configured capacity, and verified email.
- [ ] Add tests proving reward-only users are limited to one concurrent request.
- [ ] Add tests proving paid users use admin-configured per-user and per-key limits.
- [ ] Add tests proving request-per-minute limits reject excess calls before upstream account selection.
- [ ] Implement `admission_limits` storage and default rows.
- [ ] Implement runtime counters keyed by user and API key.
- [ ] Log admission rejects with `user_id`, `api_key_id`, `reason`, `balance_micros`, and limit fields.
- [ ] Run `go test ./internal/admission ./internal/server ./internal/store -count=1`.
- [ ] Commit: `git commit -m "feat: add relay admission limits"`

Admission law:

```text
allow = balance > min_balance AND under_global_limit AND under_user_limit AND under_key_limit
```

### Task 5: Wire Billing into Relay Admission and Settlement

**Files:**

- Modify: `internal/relay/relay.go`
- Modify: `internal/relay/relay_flow.go`
- Modify: `internal/relay/relay_attempt.go`
- Modify: `internal/relay/relay_attempt_test.go`
- Modify: `internal/admission/service.go`
- Modify: `internal/billing/service.go`
- Modify: `internal/requestlog/artifact.go`
- Modify: `internal/domain/log.go`
- Modify: `internal/store/sqlite_logs.go`

- [ ] Add failing relay tests for admission rejection when balance or capacity fails.
- [ ] Add failing relay tests for balance `> 0` and capacity available allowing a request and debiting actual usage.
- [ ] Add failing relay tests proving two settlements for the same request id create one ledger debit.
- [ ] Add failing stream tests proving final usage is persisted before forwarding the final completion event.
- [ ] Add failing disconnect tests proving a stream abort with observed usage is still settled and a stream abort without usage is marked `aborted_no_usage`.
- [ ] Add failing startup reconciliation tests proving `usage_observed` rows with no `ledger_id` are settled.
- [ ] Add request-log correlation fields `request_id` and `api_key_id`.
- [ ] Insert `billable_requests.in_progress` after admission and before the upstream request starts.
- [ ] Check admission after authentication and before account selection.
- [ ] Settle non-stream requests after usage is parsed and before writing the response body.
- [ ] Settle stream requests when final usage is observed; persist usage before forwarding final completion to the client.
- [ ] On settlement failure, persist `settlement_failed` with the causal error and log `request_id`, `user_id`, `api_key_id`, `model`, and usage.
- [ ] Run `go test ./internal/relay ./internal/admission ./internal/billing ./internal/requestlog ./internal/store -count=1`.
- [ ] Commit: `git commit -m "feat: bill relay usage"`

Accepted failure model for MVP:

- If a user has positive balance and fires concurrent requests, all admitted requests may settle and push the account further negative. This is intentionally accepted for the first release.
- Reward-only users are constrained to one concurrent request, so the accepted concurrent overrun applies to verified users under admin-configured paid limits.
- If a stream disconnects before any final usage is observed, the system records `aborted_no_usage`; it does not invent a token estimate in MVP.
- If SQLite settlement fails after usage is observed, durable `billable_requests` state allows startup reconciliation.

Removal condition for `aborted_no_usage` manual review:

- Add provider-specific prompt-token estimation only after real abort-without-usage volume makes manual review unacceptable.

### Task 6: Add 7pay Recharge

**Files:**

- Create: `internal/domain/payment.go`
- Create: `internal/payments/zpay/sign.go`
- Create: `internal/payments/zpay/client.go`
- Create: `internal/payments/zpay/sign_test.go`
- Create: `internal/store/sqlite_payments.go`
- Create: `internal/server/customer_billing.go`
- Modify: `internal/config/config.go`
- Modify: `internal/server/server_routes.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`

- [ ] Add sign tests for filtering `sign`, `sign_type`, empty values, ASCII key sorting, raw value concatenation, and lowercase MD5.
- [ ] Add order creation test proving local pending order is inserted before calling zpay.
- [ ] Add webhook test proving bad signature returns `fail`.
- [ ] Add webhook test proving non-`TRADE_SUCCESS` returns `success` without fulfillment.
- [ ] Add webhook test proving amount mismatch returns `fail` and does not credit ledger.
- [ ] Add webhook idempotency test proving duplicate callbacks do not double-credit.
- [ ] Add active-query polling test proving pending orders can be fulfilled from `api.php?act=order`.
- [ ] Add config keys `ZPAY_PID`, `ZPAY_KEY`, `ZPAY_CID`, and `SITE_URL`.
- [ ] Implement `POST /api/payments/orders`.
- [ ] Implement `GET /api/payments/orders/{out_trade_no}` with throttled active query.
- [ ] Implement `GET|POST /api/payments/zpay/notify`.
- [ ] Redact `ZPAY_KEY` from every log and stored event payload.
- [ ] Run `go test ./internal/payments/zpay ./internal/server ./internal/store -count=1`.
- [ ] Commit: `git commit -m "feat: add zpay recharge"`

7pay fulfillment law:

```text
payment_order(pending) + valid paid gateway evidence + amount match
  -> payment_order(paid)
  -> billing_ledger(payment_credit, source_id=out_trade_no)
```

### Task 7: Add Verified Referral Signup Credits

**Files:**

- Modify: `internal/domain/billing.go`
- Modify: `internal/billing/service.go`
- Modify: `internal/billing/service_test.go`
- Modify: `internal/server/customer_auth.go`
- Modify: `internal/store/sqlite_billing.go`
- Modify: `internal/store/sqlite_users.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`

- [ ] Add tests proving registration with a valid invite code credits the invitee immediately.
- [ ] Add tests proving the inviter is credited only after the invitee has a successful paid order.
- [ ] Add tests proving invalid invite code rejects registration.
- [ ] Add tests proving duplicate signup/retry cannot duplicate referral credits.
- [ ] Add `referrals` table and referral settings.
- [ ] Generate stable referral codes for every user.
- [ ] Insert invitee referral credit in the registration fulfillment path.
- [ ] Insert inviter referral credit from the payment fulfillment path using an invitee-scoped idempotency key.
- [ ] Expose referral summary in customer `/api/me`.
- [ ] Run `go test ./internal/billing ./internal/server ./internal/store -count=1`.
- [ ] Commit: `git commit -m "feat: add referral signup credits"`

Referral law:

```text
register(user_with_invite_code)
  -> users.referred_by_user_id = inviter
  -> referrals(inviter, invitee)
  -> ledger += invitee signup bonus

paid_order(invitee)
  -> ledger += inviter paid referral bonus
```

### Task 8: Add Customer Portal

**Files:**

- Create: `web/src/routes/app/+layout.svelte`
- Create: `web/src/routes/app/login/+page.svelte`
- Create: `web/src/routes/app/register/+page.svelte`
- Create: `web/src/routes/app/dashboard/+page.svelte`
- Create: `web/src/routes/app/keys/+page.svelte`
- Create: `web/src/routes/app/billing/+page.svelte`
- Create: `web/src/routes/app/referrals/+page.svelte`
- Modify: `web/src/routes/+page.ts`
- Modify: `web/src/routes/+layout.svelte`
- Modify: `web/src/lib/admin-types.ts`

- [ ] Build customer login and registration forms.
- [ ] Build balance panel using ledger-derived balance.
- [ ] Build recharge form that creates a 7pay QR order and polls order status every 3 seconds.
- [ ] Build API key list/create/revoke UI.
- [ ] Build usage table from request logs and ledger debits.
- [ ] Build referral link and bonus history.
- [ ] Keep admin token login visually and route-wise separate from customer login.
- [ ] Run `cd web && npm run build`.
- [ ] Commit: `git commit -m "feat: add customer portal"`

UI rule:

- Customer pages do not expose upstream account identities, proxy cells, or provider account management.
- Admin pages do not ask for customer passwords.

### Task 9: Add Admin Billing Controls

**Files:**

- Create: `internal/server/admin_billing.go`
- Create: `web/src/routes/admin-billing/+page.svelte`
- Create: `web/src/routes/admin-billing/orders/+page.svelte`
- Create: `web/src/routes/admin-billing/users/[id]/+page.svelte`
- Modify: `internal/server/server_routes.go`
- Modify: `web/src/routes/+layout.svelte`
- Modify: `web/src/routes/users/+page.svelte`
- Modify: `web/src/routes/users/[id]/+page.svelte`

- [ ] Add admin endpoints for billing settings.
- [ ] Add admin endpoints for admission limits.
- [ ] Add admin endpoints for model prices.
- [ ] Add admin endpoints for payment order lookup.
- [ ] Add admin endpoints for user ledger and manual adjustments.
- [ ] Add tests proving manual adjustments are ledger rows with idempotency keys.
- [ ] Add tests proving price changes affect future debits only.
- [ ] Build admin pages for settings, admission limits, prices, orders, user balance, and ledger.
- [ ] Run `go test ./internal/server ./internal/billing -count=1`.
- [ ] Run `cd web && npm run build`.
- [ ] Commit: `git commit -m "feat: add admin billing controls"`

### Task 10: Retire Claude/Gemini Product Debris

**Files:**

- Modify or delete: `internal/driver/claude*.go`
- Modify or delete: `internal/driver/gemini*.go`
- Modify: `internal/domain/account.go`
- Modify: `internal/server/admin_oauth*.go`
- Modify: `internal/server/compat_openai_*`
- Modify: `web/src/routes/add-account/[provider]/+page.svelte`
- Modify: `README.md`

- [ ] Remove public docs that tell users to configure Claude Code or Gemini.
- [ ] Remove UI provider choices except Codex.
- [ ] Remove tests whose only assertion is that Claude/Gemini are supported.
- [ ] Keep generic provider abstractions only if Codex still uses them and they do not expose non-Codex products.
- [ ] Run `go test ./...`.
- [ ] Run `cd web && npm run build`.
- [ ] Commit: `git commit -m "refactor: retire non-codex product paths"`

Boundary:

- It is acceptable for `driver.Driver` to remain generic.
- It is not acceptable for server routes, model catalogs, customer UI, or docs to advertise Claude/Gemini.

### Task 11: End-to-End Verification and Release Notes

**Files:**

- Create: `scripts/smoke-billing.sh`
- Create: `docs/codex-openai-saas-relay.md`
- Modify: `README.md`

- [ ] Add smoke script that creates a user, grants credit, creates an API key, calls `/openai/responses` against a fake upstream, and verifies one debit ledger row.
- [ ] Add smoke script mode for balance `<= 0` rejection.
- [ ] Add smoke script mode for reward-only concurrent request rejection.
- [ ] Add smoke script mode proving checkpointed balance equals full ledger recomputation.
- [ ] Add payment smoke notes for live 7pay because gateway credentials are environment-specific.
- [ ] Document environment variables: `API_TOKEN`, `ENCRYPTION_KEY`, `ZPAY_PID`, `ZPAY_KEY`, `ZPAY_CID`, `SITE_URL`, optional SMTP settings, and session settings.
- [ ] Document public client setup for Codex/OpenAI-compatible callers.
- [ ] Run `go test ./...`.
- [ ] Run `cd web && npm run build`.
- [ ] Run `bash scripts/smoke-billing.sh`.
- [ ] Commit: `git commit -m "docs: document codex saas relay"`

## Verification Matrix

Required before merging this branch:

- `go test ./...`
- `cd web && npm run build`
- `bash scripts/smoke-billing.sh`
- 7pay sign tests pass with deterministic fixtures
- webhook duplicate delivery test proves one ledger credit
- relay duplicate settlement test proves one ledger debit
- relay stream usage-observed reconciliation test passes
- reward-only concurrent request rejection test passes
- checkpointed balance equals full ledger recomputation
- balance `<= 0` rejection test passes
- concurrent positive-balance requests are documented as accepted MVP behavior

## Explicit Non-Goals

- No monthly subscriptions in this phase.
- No prepaid reservation or per-request hold in this phase.
- No strict "only one last negative request" guarantee for paid users in this phase; paid-user overrun is bounded by configured admission limits.
- No multi-provider public product surface.
- No provider-specific billing logic inside `pool`, `relay`, or `server` shared core.
- No mutable `users.balance` column. Bounded balance checkpoints are allowed as a derived projection.
- No automated refund flow in this phase; admin adjustment ledger rows are the manual reversal path.

## Open Decisions Fixed by This Plan

- Keep one plan document.
- Keep `/openai/responses` as canonical.
- Keep a small OpenAI-compatible surface as a projection, not a second execution path.
- Use USD-equivalent integer credits internally.
- Snapshot RMB-to-USD-equivalent rate at payment order creation.
- Admission includes balance, capacity, and abuse gates.
- API keys use deterministic SHA-256 hashes; customer passwords use bcrypt.
- Balance reads use ledger checkpoints to avoid unbounded hot-path SUM queries.
- Stream billing persists usage before final stream completion is forwarded, and startup reconciliation settles observed-but-unsettled usage.
- Allow postpaid settlement to push balance negative after an admitted request.
- Accept paid-user concurrent overrun within configured admission limits for MVP.
- Credit invitee referral rewards at signup; credit inviter referral rewards after the invitee's first successful paid order.
