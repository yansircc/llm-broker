# cc-relayer

Personal VPS relay for Claude Code / Codex CLI, managing 3-7 OAuth accounts.

## Core Formula

```
Relay(req) = retry(N) { a ← Pick(risk<1); resp ← Forward(mask(req,a)); Observe(a, resp) }
```

- **Pick**: `pool.go:Pick` → `isAvailable` four-way filter → priority DESC, lastUsedAt ASC
- **Forward**: `relay.go` → `EnsureValidToken` → HTTP upstream
- **mask**: `identity/transform.go` → strip billing headers, rewrite user_id, bind stainless fingerprint
- **Observe**: `pool.go:Observe` → sole state-change entry point → `applyCooldown` monotonic → `bus.Publish`

## Four Non-Negotiable Invariants

1. **Observe is sole write entry** — all account state changes go through `Pool.Observe()`
2. **applyCooldown is monotonic** — `new = max(existing, proposed)`, cooldown never shortens
3. **Pick never returns unavailable** — `isAvailable()` filters: status!=active, schedulable=false, overloadedUntil, opus rate limit
4. **Session pollution prevention** — bound + unavailable + isOldSession → reject 400, never switch accounts mid-session

## Architecture

```
main.go → store → crypto → bus → pool → tokenMgr → identity → relay → server
```

Pool is the center: all account state reads/writes are serialized through `Pool.mu`.

## Package Guide

| Package | Responsibility | Key File |
|---------|---------------|----------|
| `domain` | Pure types, zero dependencies | account.go, user.go, log.go |
| `pool` | Account state machine, single source of truth | pool.go |
| `relay` | Stateless request forwarding | relay.go |
| `oauth` | PKCE flows (Claude + Codex), token management | claude.go, codex.go, token.go |
| `identity` | Request masking, stainless binding | transform.go |
| `store` | SQLite persistence (17-method interface) | store.go, sqlite_accounts.go |
| `events` | Ring buffer event bus | bus.go |
| `crypto` | AES encryption, key derivation | crypto.go |
| `server` | HTTP handlers, admin API, probes | server.go, admin_probe.go |
| `config` | Env-based configuration | config.go |
| `auth` | Token authentication middleware | middleware.go |
| `transport` | Per-account HTTP clients (proxy support) | manager.go |

## Concurrency Model

- `Pool.mu` (RWMutex) serializes all state
- `persistLocked()` writes to SQLite synchronously inside lock
- `store.SaveAccount` is only called from within Pool
- No async goroutine persist — avoids snapshot-stale overwrites for 3-7 accounts

## Status Transitions

| Code | Action |
|------|--------|
| 200 | Update rate limit headers, set lastUsedAt |
| 401 | Short cooldown (30s) + background token refresh via `onAuthFailure` callback |
| 403 ban | status=blocked, long cooldown, auto-recovers via `RunCleanup` |
| 403 non-ban | Cooldown only |
| 429 | Cooldown (Retry-After or unified-reset), capture opus rate limit |
| 529 | Overload cooldown |

## Development

```bash
go build ./...          # compile
go test ./...           # run tests
cd web && npm run build # frontend (SvelteKit → adapter-static → internal/ui/dist)
```

Deploy: `/deploy` skill
