#!/usr/bin/env bash
set -euo pipefail

# ── Config ──────────────────────────────────────────────
REMOTE="root@DEPLOY_HOST"
REMOTE_BIN="/usr/local/bin/cc-relayer"
REMOTE_BAK="/usr/local/bin/cc-relayer.bak"
SERVICE="cc-relayer"
TMP_LOCAL="/tmp/cc-relayer-new"
TMP_REMOTE="/tmp/cc-relayer-new"

# ── Rollback mode ──────────────────────────────────────
if [[ "${1:-}" == "rollback" ]]; then
    echo "==> rolling back to previous version..."
    HAS_BAK=$(ssh "$REMOTE" "test -f $REMOTE_BAK && echo yes || echo no")
    if [[ "$HAS_BAK" != "yes" ]]; then
        echo "    FAIL: no backup found at $REMOTE_BAK"
        exit 1
    fi
    ssh "$REMOTE" "
        mv $REMOTE_BAK $REMOTE_BIN
        systemctl restart $SERVICE
    "
    sleep 2
    STATUS=$(ssh "$REMOTE" "systemctl is-active $SERVICE 2>/dev/null || true")
    if [[ "$STATUS" != "active" ]]; then
        echo "    FAIL: rollback service is $STATUS"
        ssh "$REMOTE" "journalctl -u $SERVICE -n 15 --no-pager"
        exit 1
    fi
    echo "==> rollback successful (service: $STATUS)"
    exit 0
fi

# Find repo root (works from worktrees too)
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "==> repo: $REPO_ROOT"

# ── 1. Frontend build ──────────────────────────────────
if [[ "${SKIP_FRONTEND:-}" != "1" ]]; then
    echo "==> building frontend..."
    (cd web && npm run build --silent 2>&1) | tail -1
    echo "    done"
else
    echo "==> skipping frontend build (SKIP_FRONTEND=1)"
fi

# ── 2. Go cross-compile ────────────────────────────────
echo "==> compiling linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$TMP_LOCAL" ./cmd/relay/
SIZE=$(du -h "$TMP_LOCAL" | cut -f1 | xargs)
echo "    done ($SIZE)"

# ── 3. Upload ──────────────────────────────────────────
echo "==> uploading to $REMOTE..."
scp -q "$TMP_LOCAL" "$REMOTE:$TMP_REMOTE"
echo "    done"

# ── 4. Backup + Atomic replace + restart ───────────────
echo "==> backing up current binary..."
ssh "$REMOTE" "
    if [ -f $REMOTE_BIN ]; then
        cp $REMOTE_BIN $REMOTE_BAK
        echo '    backup saved to $REMOTE_BAK'
    else
        echo '    no existing binary, skip backup'
    fi
"

echo "==> replacing binary and restarting..."
ssh "$REMOTE" "
    chmod +x $TMP_REMOTE
    mv $TMP_REMOTE $REMOTE_BIN
    systemctl restart $SERVICE
"
echo "    done"

# ── 5. Verify (auto-rollback on failure) ──────────────
echo "==> verifying..."
sleep 2
STATUS=$(ssh "$REMOTE" "systemctl is-active $SERVICE 2>/dev/null || true")
if [[ "$STATUS" != "active" ]]; then
    echo "    FAIL: service is $STATUS"
    ssh "$REMOTE" "journalctl -u $SERVICE -n 15 --no-pager"

    # Auto-rollback
    HAS_BAK=$(ssh "$REMOTE" "test -f $REMOTE_BAK && echo yes || echo no")
    if [[ "$HAS_BAK" == "yes" ]]; then
        echo ""
        echo "==> auto-rolling back to previous version..."
        ssh "$REMOTE" "
            mv $REMOTE_BAK $REMOTE_BIN
            systemctl restart $SERVICE
        "
        sleep 2
        RB_STATUS=$(ssh "$REMOTE" "systemctl is-active $SERVICE 2>/dev/null || true")
        if [[ "$RB_STATUS" == "active" ]]; then
            echo "==> auto-rollback successful, service restored"
        else
            echo "==> auto-rollback FAILED, service is $RB_STATUS — manual intervention needed"
        fi
    else
        echo "==> no backup available for auto-rollback — manual intervention needed"
    fi
    exit 1
fi

# Show restart timing
ssh "$REMOTE" "journalctl -u $SERVICE --since '2 minutes ago' --no-pager -o short-precise" \
    | grep -E '(Stopping|Stopped|Started|server starting)' || true

echo ""
echo "==> deployed successfully (backup at $REMOTE_BAK)"

# Clean up local temp
rm -f "$TMP_LOCAL"
