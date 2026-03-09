#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

if [[ "${1:-}" == "rollback" ]]; then
    snapshot_ref="${2:-latest}"
    echo "==> rolling back via snapshot: $snapshot_ref"
    restored_id="$(restore_snapshot "$snapshot_ref")"
    echo "==> rollback successful: $restored_id"
    exit 0
fi

cd "$REPO_ROOT"

echo "==> repo: $REPO_ROOT"

SNAPSHOT_ID=""
RESTORE_ON_ERROR=0
RESTORING=0

on_error() {
    local exit_code=$?
    if [[ "$RESTORE_ON_ERROR" -eq 1 && "$RESTORING" -eq 0 && -n "$SNAPSHOT_ID" ]]; then
        RESTORING=1
        echo ""
        echo "==> deploy failed, auto-restoring snapshot $SNAPSHOT_ID..."
        restore_snapshot "$SNAPSHOT_ID" || true
    fi
    exit "$exit_code"
}

trap on_error ERR

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

# ── 3. Snapshot current remote state ───────────────────
echo "==> snapshotting current remote state..."
SNAPSHOT_ID="$(create_snapshot deploy)"
RESTORE_ON_ERROR=1
echo "    snapshot: $SNAPSHOT_ID"

# ── 4. Upload ──────────────────────────────────────────
echo "==> uploading to $REMOTE..."
scp -q "$TMP_LOCAL" "$REMOTE:$TMP_REMOTE"
echo "    done"

# ── 5. Stop + migrate + replace + restart ─────────────
echo "==> stopping, migrating, and restarting..."
ssh "$REMOTE" "
    systemctl stop $SERVICE || true
    set -a
    . $REMOTE_ENV
    set +a
    chmod +x $TMP_REMOTE
    $TMP_REMOTE migrate
    mv $TMP_REMOTE $REMOTE_BIN
    systemctl restart $SERVICE
"
echo "    done"

# ── 6. Verify ──────────────────────────────────────────
echo "==> verifying..."
sleep 2
STATUS=$(ssh "$REMOTE" "systemctl is-active $SERVICE 2>/dev/null || true")
if [[ "$STATUS" != "active" ]]; then
    echo "    FAIL: service is $STATUS"
    ssh "$REMOTE" "journalctl -u $SERVICE -n 15 --no-pager"
    exit 1
fi

# Show restart timing
ssh "$REMOTE" "journalctl -u $SERVICE --since '2 minutes ago' --no-pager -o short-precise" \
    | grep -E '(Stopping|Stopped|Started|server starting)' || true

# ── 7. Smoke test (HTTP endpoints) ─────────────────────
echo ""
echo "==> smoke testing endpoints..."

# Read API_TOKEN from remote EnvironmentFile
API_TOKEN="$(remote_env_value API_TOKEN)"

SMOKE_FAIL=0
smoke() {
    local label="$1" url="$2" auth="${3:-}" expect="${4:-200}"
    local args=(-s -o /dev/null -w '%{http_code}' --max-time 10)
    [[ -n "$auth" ]] && args+=(-H "Authorization: Bearer $auth")
    local code
    code=$(curl "${args[@]}" "$url" 2>/dev/null || echo "000")
    if [[ "$code" == "$expect" ]]; then
        echo "    ✓ $label ($code)"
    else
        echo "    ✗ $label (got $code, expected $expect)"
        SMOKE_FAIL=1
    fi
}

# Public endpoints
smoke "GET /health" "$SITE/health"
smoke "GET /v1/models" "$SITE/v1/models" "" 401

# Admin API (needs token)
if [[ -n "$API_TOKEN" ]]; then
    smoke "GET /v1/models (auth)" "$SITE/v1/models" "$API_TOKEN"
    smoke "GET /admin/dashboard" "$SITE/admin/dashboard" "$API_TOKEN"
    smoke "GET /admin/accounts"  "$SITE/admin/accounts"  "$API_TOKEN"
    smoke "GET /admin/users"     "$SITE/admin/users"      "$API_TOKEN"
    smoke "GET /admin/health"    "$SITE/admin/health"     "$API_TOKEN"
else
    echo "    ⚠ skipping authenticated endpoints (API_TOKEN not found on remote)"
fi

# Frontend pages (static assets, should return 200)
smoke "GET /"                 "$SITE/"
smoke "GET /dashboard"        "$SITE/dashboard"
smoke "GET /add-account/claude" "$SITE/add-account/claude"
smoke "GET /add-account"      "$SITE/add-account" "" 404
smoke "GET /ui/"              "$SITE/ui/" "" 404
smoke "GET /ui/add-account"   "$SITE/ui/add-account" "" 404

if [[ "$SMOKE_FAIL" -eq 1 ]]; then
    echo ""
    echo "==> ⚠ smoke test failures detected — restore with:"
    echo "    bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"
else
    echo "    all endpoints OK"
fi

# ── 8. Browser smoke test (Playwright) ────────────────
if [[ -d "$REPO_ROOT/web/node_modules/playwright-core" ]]; then
    echo ""
    echo "==> browser smoke test..."
    SITE="$SITE" API_TOKEN="$API_TOKEN" node "$REPO_ROOT/web/smoke.mjs"
    if [[ $? -ne 0 ]]; then
        echo "==> ⚠ browser smoke test found JS errors — check output above"
    fi
else
    echo ""
    echo "    ⚠ skipping browser smoke (run: cd web && npm i && npx playwright install chromium)"
fi

RESTORE_ON_ERROR=0
trap - ERR

echo ""
echo "==> deployed successfully"
echo "    snapshot: $SNAPSHOT_ID"
echo "    restore: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"

# Clean up local temp
rm -f "$TMP_LOCAL"
