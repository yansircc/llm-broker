#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

if [[ "${1:-}" == "rollback" ]]; then
    snapshot_ref="${2:-latest}"
    echo "==> rolling back via snapshot: $snapshot_ref"
    restored_id="$(restore_snapshot "$snapshot_ref")"
    echo "==> rollback successful: $restored_id"
    exit 0
fi

strategy="${DEPLOY_STRATEGY:-auto}"
if [[ "$strategy" == "auto" ]]; then
    echo "==> detecting deploy strategy..."
    strategy="$(detect_remote_deploy_strategy)"
    echo "    strategy: $strategy"
fi

case "$strategy" in
    legacy)
        ;;
    bluegreen)
        exec bash "$SCRIPT_DIR/bluegreen_deploy.sh" "$@"
        ;;
    *)
        echo "unsupported DEPLOY_STRATEGY: $strategy" >&2
        exit 1
        ;;
esac

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

build_release_artifact

# ── 3. Snapshot current remote state ───────────────────
echo "==> snapshotting current remote state..."
SNAPSHOT_ID="$(create_snapshot deploy)"
RESTORE_ON_ERROR=1
echo "    snapshot: $SNAPSHOT_ID"

# ── 4. Upload ──────────────────────────────────────────
upload_candidate_binary

# ── 5. Stop + migrate + replace + restart ─────────────
echo "==> stopping, migrating, and restarting..."
ssh "$REMOTE" "systemctl stop $SERVICE || true"
run_uploaded_binary_migrate
ssh "$REMOTE" env TMP_REMOTE="$TMP_REMOTE" REMOTE_BIN="$REMOTE_BIN" SERVICE="$SERVICE" bash -s <<'EOF'
set -euo pipefail
mv "$TMP_REMOTE" "$REMOTE_BIN"
chmod +x "$REMOTE_BIN"
systemctl restart "$SERVICE"
EOF
echo "    done"

# ── 6. Verify ──────────────────────────────────────────
echo "==> verifying..."
sleep 2
assert_remote_service_active "$SERVICE"

echo "==> waiting for /health..."
wait_for_site_health
echo "    healthy"

verify_db_invariants

# Show restart timing
show_recent_restart_events "$SERVICE"

# ── 7. Smoke test (HTTP endpoints) ─────────────────────
run_nonfatal_smoke_suite "$SNAPSHOT_ID"

RESTORE_ON_ERROR=0
trap - ERR

echo ""
echo "==> deployed successfully"
echo "    snapshot: $SNAPSHOT_ID"
echo "    restore: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"

# Clean up local temp
rm -f "$TMP_LOCAL"
