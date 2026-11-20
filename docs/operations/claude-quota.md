# Claude quota scope and recovery

## Runtime contract

Claude response headers are parsed only by the Claude driver. Shared `5h` / `7d`
rejections cool the quota bucket; a Fable request whose `7d_oi` window is rejected
and whose representative claim is `seven_day_overage_included` restricts only
Fable, unless a shared window explicitly rejects the request. Missing shared
headers are not interpreted as either allowed or rejected. An unclassified 429
retains a bucket cooldown with `quota_scope=account`, `quota_key=unknown`.

An explicit Retry-After (seconds or HTTP date) takes precedence over unified reset
(Unix seconds or RFC3339), then the configured 429 pause (60 seconds by default).
Expired model retry deadlines permit a fresh attempt; stale percentages do not
prove either renewed allowance or permanent exhaustion. Existing retry-account
limits apply, and models are never automatically substituted.

Extra usage follows upstream settings: subscription utilization alone does not
block paid requests. An actual upstream rejection always applies its retry delay.
This feature does not enable extra usage, buy credits, redeem resets, or change
subscriptions. Successful ordinary-model probes never clear Fable-only state.

## State and concurrency

`Effect.UpdatedState` is an opaque observation reduced through
`SchedulerDriver.MergeState` inside `pool.Observe`, against the latest bucket
state. Claude merges only present, valid fields per window, ordered by local
response-observation time. An older observation cannot replace a newer window;
missing fields retain their last observation, and explicit zero utilization is
retained. This does not infer upstream processing order from network responses.
Codex and Gemini retain full-snapshot replacement behavior.

The existing public utilization/reset JSON fields remain valid. New metadata,
Fable, and usage-credit observations live in the existing bucket JSON. There is
no SQL schema change or startup rewrite. No log replay is used as runtime state.

A successful probe clears a shared cooldown only when subject, bucket, provider
state and cooldown still match its starting snapshot. The check, clear, and
observation happen under the same pool lock. A concurrent newer Claude response
changes the persisted observation stamp and prevents that clear.

These guarantees are for one process. There is no database-level multi-process
compare-and-swap, and snapshot comparison does not claim to solve arbitrary ABA.
During rollout/recovery, retain one production traffic writer and drain the old
slot before accepting requests on the new one.

## API and UI

Existing account `windows` gain a stable `key`, `scope`, `status`, `reason`,
`observed_at` (Unix seconds), and `raw_pct`. `pct` is nullable for unknown or stale
usage. Public `/api/status` exposes the same semantics with nullable
`remaining_pct`; it does not expose account identities or raw request data.
Percentages displayed to users are clamped to 0–100; the raw percentage remains
available in account details. Reset-expired values are shown as pending update.

Account-list columns and sorting use window keys, not array offsets. Public
cooldown and model-only restrictions are distinct. The list shows compact
readings, with reasons, raw usage and observation time in tooltips; details and
public status show reasons inline. Usage-credit rejection describes billing
availability, not ordinary-model eligibility.

`POST /admin/accounts/{id}/test` accepts an optional JSON body:

```json
{"model":"claude-opus-5-5"}
```

No body retains the driver's default probe. Unknown options/models return 400
before token I/O. The probe has a 30-second context timeout. Claude uses a
one-output-token request through the same request builder and beta URL as normal
inference. Refreshing credentials preserves all quota cooldowns; authentication
freshness does not prove quota recovery. Gemini's existing quota probe rejects model selection.

Semantic events and error observation files include quota key/scope, retry time,
and upstream request ID. These are diagnostics, never scheduling authority.

## Verification

```sh
go test ./...
go test -race ./...
go vet ./...
(cd web && npm run build)
```

The allowlisted production-header fixture is
`internal/driver/testdata/claude-fable-quota-429.json`. Tests retain its statuses,
66%/101% ratios and absent 5h headers while shifting deadlines relative to test
time. Integration tests exercise real Claude request transformation, native
Fable/Opus, compatible Sonnet/Fable, rejected extra-usage requests, persistence,
concurrent sparse observations and stale-probe protection. No real account is
used by these tests.

For browser verification of the built UI with real server projections:

```sh
BROKER_QUOTA_BROWSER=1 go test ./internal/server \
  -run '^TestClaudeQuotaBrowserFixture$' -count=1 -v
```

Open the printed loopback `QUOTA_BROWSER_URL`. Public `/status` must show 34%
shared remaining, 0% Fable remaining, and pending update for an expired account.
Log in through `/login` with the synthetic token `quota-local-test`; inspect
`/accounts` and `/accounts/fable-limited`. Fable's 0% must not appear in an
unrelated account's 5h column. Finish by POSTing `/fixture/finish` on that loopback
origin; the test closes its listener. It otherwise expires after ten minutes.
Close only browser resources created for this verification.

## Deployment and existing production cooldown

Implementation alone does not deploy or clear production cooldowns. A separately
authorized rollout uses the existing blue-green deploy skill, committing exact
source and built assets first. Record the local commit, binary checksum, snapshot,
active slot and verification result; publish the snapshot-specific restore
command before changing traffic. Drain the old writer before enabling new-slot
traffic, because driver observations differ from old full snapshots.

After rollout, recover only the intended Claude `(provider, subject)`:

1. GET account details and record subject, current shared cooldown, and windows.
   Confirm there is one production traffic writer and no concurrent maintenance.
2. POST the probe with `{"model":"claude-fable-5-2"}`. Read account details again.
   Proceed only if it succeeds, or records a Fable-only restriction while leaving
   the prior shared cooldown unchanged. Stop on auth/transport/shared/unknown
   rejection. This obtains fresh provider evidence; do not reconstruct state from
   the historical error log.
3. POST the probe with `{"model":"claude-opus-5-5"}`. It must succeed, and a GET
   must confirm that the unchanged old shared cooldown was cleared. A newer
   observation must prevent clearing; investigate instead of forcing a clear.
4. Verify one ordinary native Opus request through the normal relay and a Fable
   request consistent with its current quota. Preserve the chosen model. Bind
   evidence to the target account using request logs; if several accounts exist,
   use an already-authorized bound user key rather than modifying user policy.
5. Confirm API, UI and persisted state agree. Capture receipts and commit the
   deployment verification separately. Do not mark this complete from health
   checks or synthetic tests alone.

If any step is uncertain, preserve the account state and reconcile the response
before retrying. Rollback uses the recorded snapshot; it is separate from clearing
quota state or changing subscription settings.
