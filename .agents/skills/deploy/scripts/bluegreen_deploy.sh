#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

cd "$REPO_ROOT"

echo "==> repo: $REPO_ROOT"
echo "==> target: $DEPLOY_TARGET_NAME ($SITE via $REMOTE)"

SNAPSHOT_ID=""
SWITCHED=0
STARTED_INACTIVE=0
DRAIN_STARTED=0
ACTIVE_SLOT_NAME=""
ACTIVE_SERVICE_NAME=""
INACTIVE_SLOT_NAME=""
INACTIVE_SERVICE_NAME=""
INACTIVE_BIN_PATH=""
INACTIVE_PORT_VALUE=""

on_error() {
    local exit_code=$?
    trap - ERR
    if [[ "$SWITCHED" -eq 1 && "$DRAIN_STARTED" -eq 1 && -n "$SNAPSHOT_ID" ]]; then
        echo ""
        echo "==> deploy failed while draining old slot."
        echo "    new active slot remains live: $INACTIVE_SLOT_NAME"
        echo "    old slot remains running/draining: $ACTIVE_SLOT_NAME"
        echo "    rollback if needed: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"
        exit "$exit_code"
    fi
    if [[ "$SWITCHED" -eq 1 && -n "$SNAPSHOT_ID" ]]; then
        echo ""
        echo "==> deploy failed after traffic switch; moving traffic back to $ACTIVE_SLOT_NAME..."
        if set_bluegreen_active_slot "$ACTIVE_SLOT_NAME" "$active_port" && remote_reload_caddy; then
            echo "    traffic restored to previous slot: $ACTIVE_SLOT_NAME"
            echo "==> draining failed new slot $INACTIVE_SLOT_NAME..."
            if start_remote_slot_drain "$INACTIVE_PORT_VALUE"; then
                if wait_for_remote_slot_drain "$INACTIVE_PORT_VALUE" "$BLUEGREEN_DRAIN_TIMEOUT"; then
                    ssh "$REMOTE" "systemctl stop $INACTIVE_SERVICE_NAME >/dev/null 2>&1 || true" || true
                    echo "    failed new slot stopped: $INACTIVE_SLOT_NAME"
                else
                    echo "    failed new slot remains running/draining: $INACTIVE_SLOT_NAME"
                fi
            else
                echo "    warning: could not start drain on failed new slot; leaving it running"
            fi
        else
            echo "    warning: traffic rollback failed; new slot remains active: $INACTIVE_SLOT_NAME"
            echo "    previous slot remains running: $ACTIVE_SLOT_NAME"
        fi
        echo "    snapshot retained: $SNAPSHOT_ID"
        echo "    restore if needed after requests drain: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"
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
wait_for_remote_local_ready "$INACTIVE_PORT_VALUE"
echo "    inactive slot ready"

echo "==> switching caddy upstream to $INACTIVE_SLOT_NAME..."
echo "    switch_started_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
set_bluegreen_active_slot "$INACTIVE_SLOT_NAME" "$INACTIVE_PORT_VALUE"
remote_reload_caddy
SWITCHED=1

echo "==> waiting for public /health..."
wait_for_site_health
echo "    healthy"
echo "==> waiting for public /ready..."
wait_for_site_ready
echo "    ready"

verify_db_invariants

show_recent_restart_events "$ACTIVE_SERVICE_NAME" "$INACTIVE_SERVICE_NAME"

run_required_smoke_suite "$SNAPSHOT_ID"

echo "==> draining previous active slot..."
echo "    drain_started_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
start_remote_slot_drain "$active_port"
DRAIN_STARTED=1
wait_for_remote_slot_drain "$active_port" "$BLUEGREEN_DRAIN_TIMEOUT"

echo "==> stopping previous active slot..."
echo "    old_slot_stop_started_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
if ssh "$REMOTE" "systemctl stop $ACTIVE_SERVICE_NAME >/dev/null 2>&1 || true"; then
    echo "    previous active slot stopped"
else
    echo "    warning: failed to stop previous active slot; continuing with new slot live"
fi

trap - ERR

echo ""
echo "==> blue-green deploy complete"
echo "    active slot: $INACTIVE_SLOT_NAME"
echo "    previous slot stopped: $ACTIVE_SLOT_NAME"
echo "    rollback: bash $SCRIPT_DIR/restore.sh $SNAPSHOT_ID"
