#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

cd "$REPO_ROOT"

echo "==> repo: $REPO_ROOT"

SNAPSHOT_ID=""
RESTORING=0
SWITCHED=0
STARTED_INACTIVE=0
ACTIVE_SLOT_NAME=""
ACTIVE_SERVICE_NAME=""
INACTIVE_SLOT_NAME=""
INACTIVE_SERVICE_NAME=""
INACTIVE_BIN_PATH=""
INACTIVE_PORT_VALUE=""

on_error() {
    local exit_code=$?
    if [[ "$RESTORING" -eq 0 && "$SWITCHED" -eq 1 && -n "$SNAPSHOT_ID" ]]; then
        RESTORING=1
        echo ""
        echo "==> deploy failed after traffic switch, auto-restoring snapshot $SNAPSHOT_ID..."
        restore_snapshot "$SNAPSHOT_ID" || true
        exit "$exit_code"
    fi
    if [[ "$STARTED_INACTIVE" -eq 1 && -n "$INACTIVE_SERVICE_NAME" ]]; then
        echo ""
        echo "==> deploy failed before traffic switch; stopping inactive slot $INACTIVE_SLOT_NAME"
        ssh "$REMOTE" "systemctl stop $INACTIVE_SERVICE_NAME >/dev/null 2>&1 || true" || true
    fi
    if [[ -n "$SNAPSHOT_ID" ]]; then
        echo "==> rollback: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"
    fi
    exit "$exit_code"
}

trap on_error ERR

echo "==> loading blue-green layout..."
load_bluegreen_layout

active_slot="${ACTIVE_SLOT:-$BLUE_SLOT}"
case "$active_slot" in
    "$BLUE_SLOT")
        ACTIVE_SLOT_NAME="$BLUE_SLOT"
        ACTIVE_SERVICE_NAME="$BLUE_SERVICE"
        active_port="$BLUE_PORT"
        INACTIVE_SLOT_NAME="$GREEN_SLOT"
        INACTIVE_SERVICE_NAME="$GREEN_SERVICE"
        INACTIVE_BIN_PATH="$GREEN_BIN"
        INACTIVE_PORT_VALUE="$GREEN_PORT"
        ;;
    "$GREEN_SLOT")
        ACTIVE_SLOT_NAME="$GREEN_SLOT"
        ACTIVE_SERVICE_NAME="$GREEN_SERVICE"
        active_port="$GREEN_PORT"
        INACTIVE_SLOT_NAME="$BLUE_SLOT"
        INACTIVE_SERVICE_NAME="$BLUE_SERVICE"
        INACTIVE_BIN_PATH="$BLUE_BIN"
        INACTIVE_PORT_VALUE="$BLUE_PORT"
        ;;
    *)
        echo "invalid active slot: $active_slot" >&2
        exit 1
        ;;
esac

echo "    active slot:   $ACTIVE_SLOT_NAME ($ACTIVE_SERVICE_NAME :$active_port)"
echo "    inactive slot: $INACTIVE_SLOT_NAME ($INACTIVE_SERVICE_NAME :$INACTIVE_PORT_VALUE)"

build_release_artifact

echo "==> snapshotting current remote state..."
SNAPSHOT_ID="$(create_snapshot bluegreen-deploy)"
echo "    snapshot: $SNAPSHOT_ID"

upload_candidate_binary

run_uploaded_binary_migrate

echo "==> updating inactive slot..."
ssh "$REMOTE" env \
    INACTIVE_SERVICE_NAME="$INACTIVE_SERVICE_NAME" \
    INACTIVE_BIN_PATH="$INACTIVE_BIN_PATH" \
    TMP_REMOTE="$TMP_REMOTE" \
    bash -s <<'EOF'
set -euo pipefail
systemctl stop "$INACTIVE_SERVICE_NAME" >/dev/null 2>&1 || true
install -m 755 "$TMP_REMOTE" "$INACTIVE_BIN_PATH"
rm -f "$TMP_REMOTE"
systemctl restart "$INACTIVE_SERVICE_NAME"
EOF
STARTED_INACTIVE=1
wait_for_remote_local_health "$INACTIVE_PORT_VALUE"
echo "    inactive slot healthy"

echo "==> switching caddy upstream to $INACTIVE_SLOT_NAME..."
set_bluegreen_active_slot "$INACTIVE_SLOT_NAME" "$INACTIVE_PORT_VALUE"
remote_reload_caddy
SWITCHED=1

echo "==> waiting for public /health..."
wait_for_site_health
echo "    healthy"

verify_db_invariants

show_recent_restart_events "$ACTIVE_SERVICE_NAME" "$INACTIVE_SERVICE_NAME"

echo "==> stopping previous active slot..."
if ssh "$REMOTE" "systemctl stop $ACTIVE_SERVICE_NAME >/dev/null 2>&1 || true"; then
    echo "    previous active slot stopped"
else
    echo "    warning: failed to stop previous active slot; continuing with new slot live"
fi

run_nonfatal_smoke_suite "$SNAPSHOT_ID"

trap - ERR

echo ""
echo "==> blue-green deploy complete"
echo "    active slot: $INACTIVE_SLOT_NAME"
echo "    previous slot stopped: $ACTIVE_SLOT_NAME"
echo "    rollback: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"
