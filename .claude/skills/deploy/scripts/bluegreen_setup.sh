#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

cd "$REPO_ROOT"

remote_strategy="$(detect_remote_deploy_strategy)"
if [[ "$remote_strategy" == "bluegreen" && "${FORCE_BOOTSTRAP:-}" != "1" ]]; then
    echo "blue-green already configured on $REMOTE; use deploy.sh or set FORCE_BOOTSTRAP=1" >&2
    exit 1
fi

snapshot_id=""
restoring=0

on_error() {
    local exit_code=$?
    if [[ "$restoring" -eq 0 && -n "$snapshot_id" ]]; then
        restoring=1
        echo ""
        echo "==> bootstrap failed, auto-restoring snapshot $snapshot_id..."
        restore_snapshot "$snapshot_id" || true
    fi
    exit "$exit_code"
}

trap on_error ERR

legacy_port="$(remote_env_value PORT)"
legacy_port="${legacy_port:-3000}"
blue_port="${BLUEGREEN_BLUE_PORT:-$((legacy_port + 1))}"
green_port="${BLUEGREEN_GREEN_PORT:-$((legacy_port + 2))}"

echo "==> repo: $REPO_ROOT"
echo "==> blue-green bootstrap on $REMOTE"
echo "    legacy port: $legacy_port"
echo "    blue port:   $blue_port"
echo "    green port:  $green_port"

echo "==> snapshotting current remote state..."
snapshot_id="$(create_snapshot bluegreen-bootstrap)"
echo "    snapshot: $snapshot_id"

provision_bluegreen_layout "$legacy_port" "$blue_port" "$green_port"
load_bluegreen_layout
set_bluegreen_active_slot "$BLUE_SLOT" "$BLUE_PORT"

echo "==> reloading caddy..."
remote_reload_caddy

echo "==> waiting for public /health..."
wait_for_site_health
echo "    healthy"

verify_db_invariants
show_recent_restart_events "$BLUE_SERVICE" "$GREEN_SERVICE"
run_nonfatal_smoke_suite "$snapshot_id"

echo "==> stopping legacy service..."
ssh "$REMOTE" "systemctl stop $SERVICE >/dev/null 2>&1 || true"

echo "==> disabling legacy service on boot..."
ssh "$REMOTE" "systemctl disable $SERVICE >/dev/null 2>&1 || true"

trap - ERR

echo ""
echo "==> blue-green bootstrap complete"
echo "    active slot: $BLUE_SLOT"
echo "    rollback: bash $SCRIPT_DIR/restore.sh $snapshot_id"
