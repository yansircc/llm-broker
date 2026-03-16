# broker

This file is for LLM agents working in this repo.

## Essence

The project is not a bag of Codex/Codex special cases. It is a small LLM account orchestration kernel.

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
- A failed deploy should be recoverable with `bash .Codex/skills/deploy/scripts/restore.sh latest`.
- That safety net exists so the code can stay clean; it is not permission to leave permanent compatibility clutter in the runtime path.

## Short Checklist Before You Edit

Ask:

1. Is this change about stable orchestration semantics or provider-specific protocol?
2. If provider-specific, can it live entirely in `driver`?
3. Am I introducing duplicate state?
4. Am I violating `identity(account) = (provider, subject)`?
5. Am I making core understand something it should merely consume as `Effect`?

If the answer to 2 is "no" or to 3-5 is "yes", rethink the change.
