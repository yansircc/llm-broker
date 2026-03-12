---
name: deploy
description: |
  Build and deploy broker (`llm-broker`) to production.
  Triggers: deploy, 部署, push to production, 发布, ship it, 上线.
  Handles: frontend build, Go cross-compile, remote snapshot, upload, migrate, restart, verification, rollback.
---

## Deploy broker

Run `scripts/deploy.sh` from the repo root (or any worktree). The script handles the full pipeline:

1. Build SvelteKit frontend (`web/` → `internal/ui/dist`)
2. Cross-compile Go binary for linux/amd64
3. Create a remote snapshot of binary, env, service unit, and SQLite DB
4. Upload to remote via scp
5. Stop service, run `migrate`, replace binary, restart
6. Verify service health — if restart fails, auto-restore the snapshot
7. Smoke test HTTP endpoints — `/health`, admin API, frontend pages

Before starting any deploy, tell the human the rollback command first so they can recover immediately if the release looks wrong. At minimum print `bash .claude/skills/deploy/scripts/restore.sh latest`, and after the snapshot is created, surface the exact snapshot-specific rollback command emitted by the script.

```bash
bash .claude/skills/deploy/scripts/deploy.sh
```

`deploy.sh` now supports strategy selection:

- `DEPLOY_STRATEGY=auto` (default) — if the remote has blue-green layout, use blue-green deploy; otherwise use legacy single-instance deploy
- `DEPLOY_STRATEGY=legacy` — force the old stop-replace-restart path
- `DEPLOY_STRATEGY=bluegreen` — force blue-green slot deploy

### Blue-green bootstrap

Bootstrap a host into blue-green mode once:

```bash
bash .claude/skills/deploy/scripts/bluegreen_setup.sh
```

If the host is already blue-green enabled, the script refuses to re-run unless forced:

```bash
FORCE_BOOTSTRAP=1 bash .claude/skills/deploy/scripts/bluegreen_setup.sh
```

After bootstrap, regular deploys can keep using:

```bash
bash .claude/skills/deploy/scripts/deploy.sh
```

### Rollback

Manually rollback to the most recent snapshot:

```bash
bash .claude/skills/deploy/scripts/restore.sh latest
```

Or restore a specific snapshot:

```bash
bash .claude/skills/deploy/scripts/restore.sh 20260309T211816Z-deploy
```

`deploy.sh rollback` is also available as a shortcut:

```bash
bash .claude/skills/deploy/scripts/deploy.sh rollback
```

### Partial deploy

To skip frontend build for a Go-only change:

```bash
SKIP_FRONTEND=1 bash .claude/skills/deploy/scripts/deploy.sh
```

### Failure handling

- Service fails after deploy — script auto-restores the snapshot it just took
- Blue-green bootstrap fails — script auto-restores the snapshot it just took
- Manual rollback — use `restore.sh` or the `rollback` shortcut
- No snapshot exists — first deploy can only fail forward; nothing remote is deleted before upload succeeds
- npm/go/scp fails — exits before touching the remote runtime
