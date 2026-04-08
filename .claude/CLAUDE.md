# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Dev Commands

```bash
make build          # Build frontend + Go binary (output: ./llm-broker)
make test           # Run all Go tests: go test ./...
make lint           # Run go vet ./...
make ui             # Build Svelte frontend only (cd web && npm install && npm run build)
make dev-ui         # Run Svelte dev server (cd web && npm run dev)
make dev-go         # Run Go backend only (go run ./cmd/relay)
make deps           # Tidy Go modules
make clean          # Remove binary and frontend dist
```

Run a single test:
```bash
go test ./internal/pool/ -run TestPickExclusion -v
```

Database migration (explicit, never auto-migrates on startup):
```bash
./llm-broker migrate
```

## Tech Stack

- **Go 1.24** — backend, single binary with embedded frontend
- **Svelte 5 + SvelteKit 2 + Vite 6** — frontend in `web/`, built to `internal/ui/dist/` and embedded
- **SQLite** (pure Go `modernc.org/sqlite`) — no CGO dependency
- **AES-256-GCM** — token encryption via `ENCRYPTION_KEY` env var
- **uTLS** — TLS fingerprint obfuscation for upstream requests

## Architecture Map

Single entry point: `cmd/relay/main.go`. All code lives in `internal/`.

```
server      HTTP handlers, routing, admin API, auth middleware
  ↓
relay       Execution pipeline: plan → pick → token → build → upstream → interpret → observe
  ↓
pool        In-memory account state machine. Loads from DB at startup.
            Pick() selects accounts. Observe() is the sole write entrance for state transitions.
  ↓
driver      Provider abstraction boundary. Each provider (Claude, Codex, Gemini) implements Driver.
            Owns: OAuth, token refresh, request building, response interpretation, model catalog.
  ↓
tokens      Token freshness manager. Refresh locks, expiry checks, encrypted storage.
  ↓
store       SQLite persistence. Schema in store/schema.sql (embedded).
  ↓
events      Ring-buffer event bus. Observability only — never source of truth.
transport   HTTP client pool with per-account proxy routing.
domain      Core types: Account, Provider, Status. No business logic.
config      All config from environment variables. No config files.
crypto      AES-256-GCM encrypt/decrypt for stored tokens.
```

The `Driver` interface is composed of role-specific sub-interfaces:
`Descriptor`, `RelayDriver`, `OAuthDriver`, `RefreshDriver`, `SchedulerDriver`, `AdminDriver`.
`ExecutionDriver = RelayDriver + SchedulerDriver`. Full `Driver = all combined`.

## Essence

The project is not a bag of Claude/Codex special cases. It is a small LLM account orchestration kernel.

The design center is:

```text
provider is the change axis
core is the stable axis
```

The core execution law is:

```text
Relay(req, drv) =
  retry(N) {
    a <- pool.Pick(drv, exclude, model, boundSession)
    t <- tokens.Ensure(a)
    u <- drv.BuildRequest(req, a, t)
    r <- upstream(u)
    e <- drv.Interpret(r)
    pool.Observe(a, e)
    surface.Write(drv, r)
  }
```

The state law is:

```text
identity(account) = (provider, subject)

available(account, model, now) =
  status == active
  AND cooldown_until <= now
  AND drv.CanServe(provider_state_json, model, now)

next_state = Observe(current_state, effect)
```

Read this literally:

- `driver` owns provider protocol.
- `pool` owns generic account state transitions.
- `relay` owns the execution pipeline.
- `tokens` owns token freshness.
- `events` are observability only, not source of truth.

This is a synchronous state-machine core with an event side channel. It is not an event-driven core.

## Non-Negotiable Invariants

1. Provider details must terminate at `driver.Driver`.
   `pool`, `relay`, `server`, and `store` should not learn provider-specific headers, body shapes, ban strings, or rate-limit parsing rules.

2. `pool.Observe()` is the single semantic write entrance for provider outcomes.
   Effects enter there; state transitions happen there.

3. Real account identity is `UNIQUE(provider, subject)`.
   `email` is display data only. Never deduplicate or bind by email.

4. Do not store duplicate state.
   If `status`, `cooldown_until`, and `driver.CanServe(...)` determine availability, do not add shadow booleans like the old `schedulable`.

5. Provider-owned state stays in `provider_state_json`.
   Do not grow public schema columns for provider-specific utilization windows or reset fields.

6. Durable provider identity metadata stays in `identity_json`.
   Keep `identity_json` and `provider_state_json` conceptually separate:
   `identity_json` answers "who is this account?"
   `provider_state_json` answers "what is this account's current provider-specific runtime state?"

7. Database migration is explicit.
   Startup should not silently rewrite schema. Use `llm-broker migrate`.

## Architectural Boundary

What belongs in a driver:

- OAuth generation and code exchange
- token refresh request semantics
- upstream request construction
- response interpretation into `driver.Effect`
- streaming and non-stream response handling
- probe semantics
- model catalog
- provider-specific utilization math
- provider-specific account presentation fields

What does not belong in core:

- `if provider == ...` branches for protocol behavior
- provider-specific rate-limit headers in `pool`
- provider-specific JSON parsing in `server`
- provider-specific model catalogs hardcoded in `server`
- provider-specific schema columns in `domain.Account`

If a provider change requires touching many core packages, the boundary is regressing.

## The Correct Mental Model

Think in compiler terms:

```text
provider protocol -> driver.Interpret -> Effect -> pool.Observe -> account state
```

`driver.Effect` is the IR between unstable upstream behavior and stable core semantics.

The core should only care about things like:

- success
- cooldown
- overload
- block
- auth failure
- updated provider state

The core should not care how any provider expressed those facts.

## Schema Aesthetic

The schema should encode the fewest truths necessary.

Good:

- `id`
- `provider`
- `subject`
- `email`
- `status`
- `priority`
- `priority_mode`
- `cooldown_until`
- encrypted tokens
- timestamps
- `identity_json`
- `provider_state_json`

Bad:

- provider-specific columns for rate limits
- duplicate booleans derivable from existing state
- compatibility debris kept after migration is over

The project prefers fewer states over more defensive code.

## Route Truths

These are intentional and should not drift casually:

- UI lives at `/` and `/dashboard`
- onboarding lives at `/add-account/{provider}`
- `/add-account` without a provider should 404
- `/ui/*` should 404
- `GET /v1/models` is authenticated
- relay paths are registered from driver metadata, not hardcoded server constants

## Extension Rule

Adding a provider should look roughly like:

1. implement `driver.Driver`
2. register it in `cmd/relay/main.go`
3. expose provider metadata through `Driver.Info()` and `Driver.Models()`
4. reuse existing `pool`, `relay`, `server`, `tokens`, and `store`

If you find yourself editing `pool` because a provider has a different header name, stop. The driver abstraction is being violated.

## Review Heuristics

Reject or challenge changes that:

- reintroduce provider conditionals into core packages
- deduplicate accounts by email or mutable metadata
- add fallback layers that duplicate current truth
- rebuild provider state from old legacy fields after migration is complete
- make events/logs authoritative for runtime state
- add UI knobs for redundant state that should not exist

Prefer changes that:

- delete invalid states
- tighten the provider boundary
- make `driver` more complete and core more ignorant
- reduce cross-package knowledge
- improve rollback safety without preserving dead runtime compatibility

## Operational Truths

- The project relies on VPS snapshots and `restore.sh` for rollback safety.
- A failed deploy should be recoverable with `bash .claude/skills/deploy/scripts/restore.sh latest`.
- That safety net exists so the code can stay clean; it is not permission to leave permanent compatibility clutter in the runtime path.

## Client Anti-Bypass Setup

Clients must only reach upstream through the broker. Server-side rewriting alone is insufficient if clients can bypass the gateway.

**Required client environment:**

```bash
export ANTHROPIC_BASE_URL="https://<broker-host>"
export ANTHROPIC_API_KEY="<broker-token>"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
export CLAUDE_CODE_ATTRIBUTION_HEADER=false
```

**Optional network-level blocking (Clash / ACL rules):**

```yaml
# Block direct connections to Anthropic from client machines
- DOMAIN-SUFFIX,anthropic.com,REJECT
- DOMAIN-SUFFIX,claude.com,REJECT
- DOMAIN-SUFFIX,claude.ai,REJECT
```

**Server-side opt-in:**

```bash
# Enable prompt environment masking (normalize platform/shell/OS/paths)
export PROMPT_ENV_HOME=/Users/user
```

**Verification:** `GET /v1/models` with a valid broker token returns the model list (proves auth + relay wiring). Direct `curl https://api.anthropic.com` from client machines is blocked by network rules.

## Short Checklist Before You Edit

Ask:

1. Is this change about stable orchestration semantics or provider-specific protocol?
2. If provider-specific, can it live entirely in `driver`?
3. Am I introducing duplicate state?
4. Am I violating `identity(account) = (provider, subject)`?
5. Am I making core understand something it should merely consume as `Effect`?

If the answer to 2 is "no" or to 3-5 is "yes", rethink the change.

## Operational Safety — Account Binding

**NEVER unbind, rebind, or change an account's cell_id without explicit user confirmation.**

Account-to-cell binding determines the egress IP. Changing it can expose the account to a different IP, triggering provider bans. This is an irreversible, destructive operation.

- Do not unbind a cell "to fix" a routing issue — ask first.
- Do not assume "no cell" (direct connect) is a safe fallback.
- If an account's cell is misconfigured, disable the cell or cooldown — do not touch the binding.
