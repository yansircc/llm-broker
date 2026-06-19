# Zero-Downtime Blue-Green Deploy Plan

## Boundary

Before this change, the `cdx` deploy target used `DEPLOY_STRATEGY=legacy`, so it
stopped the single `llm-broker` service, ran migration, replaced the binary, and
restarted. That path can interrupt active relay requests.

The local `cdx` target is now configured for `DEPLOY_STRATEGY=bluegreen`. A host
still needs a one-time `bluegreen_setup.sh` bootstrap before regular blue-green
deploys can run there.

The target state is blue-green deploy with request draining:

```text
build new binary
start inactive slot
verify inactive /health and /ready
switch Caddy upstream to inactive slot
mark old slot draining
wait until old slot active_requests == 0
stop old slot
```

The invariant is:

```text
Accepted relay requests are not actively terminated by deploy.
```

This does not mean an infinite stream can block cleanup forever. The product
must define a maximum request/stream duration. The old slot may only be stopped
after all accepted requests finish, or after they exceed that declared maximum.

## Required Runtime Changes

1. Split liveness from readiness.
   - `/health`: process is alive and store ping works.
   - `/ready`: process is accepting new business requests.
   - A draining slot returns non-2xx from `/ready`, but keeps `/health` healthy.

2. Add a drain control surface.
   - `POST /admin/drain`: mark this process as draining.
   - `GET /admin/drain-status`: return `draining`, `active_requests`,
     oldest request age, and compact active request metadata.
   - Only admin-authenticated callers can use these endpoints.

3. Gate business entry points on readiness.
   - New relay/customer/admin business requests should reject once draining.
   - Existing handlers must continue to completion.
   - Operational endpoints (`/health`, `/ready`, drain status) stay available.

4. Make graceful shutdown duration configurable.
   - Current shutdown timeout is 30 seconds.
   - Add `GRACEFUL_SHUTDOWN_TIMEOUT`, defaulting to a product-defined value.
   - systemd `TimeoutStopSec` must be longer than the Go shutdown timeout.

5. Keep request accounting observable.
   - The implemented middleware-level tracker records request id, method, path,
     remote address, start time, and age.
   - User id and streaming state belong at the authenticated relay boundary, not
     in the outer request middleware. Add them there later if deploy diagnostics
     need that detail.
   - Do not log tokens, API keys, cookies, or request bodies.

## Required Deploy Script Changes

1. Enable real blue-green layout for `cdx`.
   - Bootstrap blue-green once.
   - Set `DEPLOY_STRATEGY=bluegreen` for `.agents/skills/deploy/targets/cdx.env`.

2. Change blue-green deploy stop order.
   - Current script stops the previous active slot immediately after switching
     Caddy upstream.
   - Replace that with:

```text
POST old_slot /admin/drain
poll old_slot /admin/drain-status
if active_requests == 0: stop old slot
if max drain timeout exceeded: fail deploy or force stop according to policy
```

3. Keep rollback valid.
   - Before traffic switch, failure should stop only the inactive slot.
   - After traffic switch but before old-slot drain, failure switches Caddy back
     to the previous slot, starts drain on the failed new slot, and only stops it
     after `active_requests == 0`.
   - After old-slot drain has started, failure leaves both slots running so no
     already accepted request is actively terminated.

4. Add deploy log evidence.
   - active slot before deploy
   - inactive slot port
   - switch timestamp
   - old slot drain start timestamp
   - active request count while draining
   - old slot stop timestamp

## Database Migration Rule

Blue-green means old and new binaries can run against the same SQLite database
at the same time. Migrations must therefore be expand/contract:

1. Expand deploy: only add compatible columns, tables, indexes, and nullable
   fields.
2. Code deploy: new binary can read both old and new shapes.
3. Drain old slot.
4. Contract migration: remove old fields or incompatible behavior only after no
   old binary is running.

Heavy SQLite migrations should not run during traffic switch. They need a
separate maintenance step or a copy/swap strategy.

## Verification

The feature is not done until there is an executable verification loop:

1. Start a long streaming `/openai/v1/responses` or equivalent relay request.
2. Start deploy while the stream is active.
3. Assert the stream completes without disconnect.
4. During drain, send a new request and assert it reaches the new slot.
5. Assert old slot remains running while `active_requests > 0`.
6. Assert old slot stops only after `active_requests == 0`.
7. Assert rollback after traffic switch does not interrupt accepted requests on
   either slot.

## References

- Go `net/http.Server.Shutdown`: gracefully shuts down without interrupting
  active connections until the shutdown context expires.
  https://pkg.go.dev/net/http#Server.Shutdown
- Caddy `reverse_proxy`: supports health checks and upstream switching through
  config reload.
  https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
- systemd service stop behavior: stop timeout and final kill behavior must be
  configured to exceed app graceful shutdown duration.
  https://www.freedesktop.org/software/systemd/man/systemd.service.html
