---
name: deploy
description: |
  Build and deploy cc-relayer to production (cc.210k.cc).
  Triggers: deploy, 部署, push to production, 发布, ship it, 上线.
  Handles: frontend build, Go cross-compile, upload, atomic binary replace, systemctl restart, verification, rollback.
---

## Deploy cc-relayer

Run `scripts/deploy.sh` from the repo root (or any worktree). The script handles the full pipeline:

1. Build SvelteKit frontend (`web/` → `internal/ui/dist`)
2. Cross-compile Go binary for linux/amd64
3. Upload to remote via scp
4. **Backup current binary** to `cc-relayer.bak`
5. Atomic replace (`mv`) + `systemctl restart`
6. Verify service health — **if service fails, auto-rollback to backup**

```bash
bash .claude/skills/deploy/scripts/deploy.sh
```

### Rollback

Manually rollback to the previous version (e.g. logic bug but process still alive):

```bash
bash .claude/skills/deploy/scripts/deploy.sh rollback
```

### Partial deploy

To skip frontend build (Go-only change):

```bash
SKIP_FRONTEND=1 bash .claude/skills/deploy/scripts/deploy.sh
```

### Failure handling

- **Service fails after deploy** — script auto-rollbacks to backup and restores service
- **Manual rollback** — use `rollback` subcommand when service is running but behaving wrong
- **No backup exists** — first-ever deploy has no backup; script warns but can't auto-recover
- **npm/go/scp fails** — exits before touching remote binary, no risk
