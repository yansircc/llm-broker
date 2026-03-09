# cc-relayer

[![CI](https://github.com/yansircc/cc-relayer/actions/workflows/ci.yml/badge.svg)](https://github.com/yansircc/cc-relayer/actions/workflows/ci.yml)

`cc-relayer` is a personal VPS relay for Claude Code and Codex CLI. In the current architecture it is closer to an LLM account orchestration kernel than a thin proxy: it schedules a small pool of OAuth accounts, keeps identity boundaries intact, manages token refresh, and exposes one stable relay surface.

## Architecture

The system is built around one rule: provider is the change axis.

Core code stays provider-agnostic. Provider behavior lives behind `driver.Driver`.

```text
Relay(req, drv) =
  retry(N) {
    a <- pool.Pick(drv, model, boundSession)
    t <- tokens.Ensure(a)
    u <- drv.BuildRequest(req, a, t)
    r <- upstream(u)
    e <- drv.Interpret(r)
    pool.Observe(a, e)
    surface.Write(drv, r)
  }
```

Useful mental model:

- `driver` translates provider protocol into stable core semantics.
- `pool` is a synchronous state machine, not a provider parser.
- `tokens` owns access-token freshness.
- `relay` is a provider-neutral execution pipeline.
- `events` are observational side effects, not source of truth.

Two formulas matter:

```text
identity(account) = (provider, subject)

available(account, model, now) =
  status == active
  AND cooldown_until <= now
  AND driver.CanServe(provider_state_json, model, now)
```

That is why:

- `email` is display data, not account identity.
- `(provider, subject)` is the durable uniqueness boundary.
- provider-specific rate-limit and health state lives in `provider_state_json`, not public schema columns.

## Current Surface

- UI: `/` and `/dashboard`
- Add account: `/add-account/{provider}`
- Relay metadata: `GET /v1/models` (authenticated)
- Claude relay paths: exposed from the Claude driver
- Codex relay paths: exposed from the Codex driver
- Dead routes by design: `/ui/*`, `/add-account`

## Features

- Multi-account scheduling for a small OAuth account pool
- Per-provider drivers with explicit boundaries
- Session binding for providers that need conversation stickiness
- Per-account proxy support via a shared transport pool
- Token refresh via `internal/tokens`
- Built-in dashboard, account admin, user management, and usage views
- Explicit DB migration command
- Snapshot-friendly deployment and restore workflow

## Quick Start

### 1. Build

```bash
git clone https://github.com/yansircc/cc-relayer.git
cd cc-relayer
cd web && npm ci && npm run build && cd ..
go build -o cc-relayer ./cmd/relay
```

Requires:

- Go 1.24+
- Node 22+

### 2. Create the schema

```bash
./cc-relayer migrate
```

Schema migration is explicit. Startup does not mutate the database.

### 3. Start the server

```bash
export ENCRYPTION_KEY=$(openssl rand -hex 16)
export API_TOKEN=$(openssl rand -hex 16)

./cc-relayer
```

Defaults:

- listen: `0.0.0.0:3000`
- SQLite: `./cc-relayer.db`

### 4. Add accounts

Open the UI:

- `http://YOUR_SERVER:3000/`

Or use the admin API:

```bash
curl -X POST "http://YOUR_SERVER:3000/admin/accounts/generate-auth-url?provider=claude" \
  -H "Authorization: Bearer $API_TOKEN"
```

Then exchange the callback:

```bash
curl -X POST "http://YOUR_SERVER:3000/admin/accounts/exchange-code" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "claude",
    "session_id": "SESSION_ID",
    "callback_url": "https://..."
  }'
```

### 5. Point clients at the relay

Claude Code:

```bash
export ANTHROPIC_BASE_URL="http://YOUR_SERVER:3000"
export ANTHROPIC_API_KEY="$API_TOKEN"
claude
```

Codex CLI:

```bash
export OPENAI_BASE_URL="http://YOUR_SERVER:3000/openai"
export OPENAI_API_KEY="$API_TOKEN"
codex
```

### 6. Verify a key

Use a copy-paste-safe shell form:

```bash
BASE_URL="https://ccc.210k.cc"
API_KEY="tk_..."

curl -fsS "$BASE_URL/v1/models" \
  -H "Authorization: Bearer $API_KEY" \
  >/dev/null && echo "key ok"
```

`/v1/models` is authenticated on purpose, so this is a meaningful smoke test.

## Data Model

`accounts` keeps only core fields plus two provider-owned JSON pockets:

- `identity_json`: durable provider identity metadata for display and admin flows
- `provider_state_json`: mutable provider runtime state such as utilization windows or cooldown signals

Core account columns:

- `id`
- `provider`
- `subject`
- `email`
- `status`
- `priority`
- `priority_mode`
- `cooldown_until`
- token material and timestamps

Important invariant:

```text
UNIQUE(provider, subject)
```

This is the real account identity. Never deduplicate by email.

## Package Guide

```text
cmd/relay/              binary entrypoint + explicit migrate command
internal/
  auth/                 API key authentication middleware
  config/               environment config
  crypto/               token encryption
  domain/               stable core types
  driver/               provider boundary
  events/               observational event bus
  identity/             request masking and session fingerprinting
  pool/                 account state machine and scheduler
  relay/                provider-neutral execution pipeline
  server/               HTTP surface, admin API, UI
  store/                SQLite persistence + migration
  tokens/               access-token refresh manager
  transport/            shared transport pool keyed by proxy shape
  ui/                   embedded built frontend
web/                    Svelte source
```

## Admin API

All authenticated endpoints accept either:

- `Authorization: Bearer $API_TOKEN`
- `x-api-key: $API_TOKEN`

Main endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/v1/models` | authenticated relay model catalog |
| `GET` | `/admin/providers` | provider catalog for UI/onboarding |
| `POST` | `/admin/accounts/generate-auth-url` | start OAuth |
| `POST` | `/admin/accounts/exchange-code` | finish OAuth |
| `GET` | `/admin/accounts` | list accounts |
| `GET` | `/admin/accounts/{id}` | account detail |
| `POST` | `/admin/accounts/{id}/refresh` | refresh token |
| `POST` | `/admin/accounts/{id}/test` | probe account |
| `GET` | `/admin/dashboard` | dashboard data |
| `GET` | `/admin/users` | list users |
| `POST` | `/admin/users` | create user key |
| `GET` | `/admin/health` | authenticated health |
| `GET` | `/health` | unauthenticated process/store health |

## Development

```bash
go build ./...
go test ./...
go vet ./...
cd web && npm run build
```

## Operations

Deploy:

```bash
bash .claude/skills/deploy/scripts/deploy.sh
```

Restore latest snapshot:

```bash
bash .claude/skills/deploy/scripts/restore.sh latest
```

This project prefers explicit rollback over long-lived compatibility clutter.

## Adding a Provider

If the architecture is healthy, a new provider should mostly mean:

1. Implement a new `driver.Driver`.
2. Register it in `cmd/relay/main.go`.
3. Expose its relay paths and OAuth metadata through `Driver.Info()`.
4. Let existing core code keep working unchanged.

If a new provider requires edits scattered across `pool`, `relay`, `server`, and `store`, the boundary is regressing.

## Design Standard

The codebase prefers fewer states over more fallback code.

Bad direction:

- provider-specific branches in core
- duplicate availability flags
- schema columns for each provider's rate-limit model
- email-based deduplication
- dead compatibility layers that outlive migration

Good direction:

- one provider boundary: `driver`
- one state transition entrance: `pool.Observe`
- one real identity: `(provider, subject)`
- one mutable provider pocket: `provider_state_json`

## License

MIT
