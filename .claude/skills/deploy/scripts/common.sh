#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"

REMOTE="${REMOTE:-root@DEPLOY_HOST}"
SERVICE="${SERVICE:-cc-relayer}"
REMOTE_BIN="${REMOTE_BIN:-/usr/local/bin/cc-relayer}"
REMOTE_ENV="${REMOTE_ENV:-/etc/cc-relayer.env}"
REMOTE_SERVICE="${REMOTE_SERVICE:-/etc/systemd/system/${SERVICE}.service}"
SNAPSHOT_ROOT="${SNAPSHOT_ROOT:-/var/backups/cc-relayer}"
TMP_LOCAL="${TMP_LOCAL:-/tmp/cc-relayer-new}"
TMP_REMOTE="${TMP_REMOTE:-/tmp/cc-relayer-new}"
SITE="${SITE:-https://DEPLOY_HOST}"

sanitize_label() {
    local label="${1:-manual}"
    label="$(printf '%s' "$label" | tr '[:space:]/' '--' | tr -cs 'A-Za-z0-9._-' '-')"
    label="${label##-}"
    label="${label%%-}"
    if [[ -z "$label" ]]; then
        label="manual"
    fi
    printf '%s\n' "$label"
}

remote_env_value() {
    local key="$1"
    ssh "$REMOTE" env KEY="$key" REMOTE_ENV="$REMOTE_ENV" bash -s <<'EOF'
set -euo pipefail
awk -F= -v key="$KEY" '$1 == key { print substr($0, index($0, "=") + 1); exit }' "$REMOTE_ENV"
EOF
}

create_snapshot() {
    local label
    label="$(sanitize_label "${1:-manual}")"
    ssh "$REMOTE" env \
        SERVICE="$SERVICE" \
        REMOTE_BIN="$REMOTE_BIN" \
        REMOTE_ENV="$REMOTE_ENV" \
        REMOTE_SERVICE="$REMOTE_SERVICE" \
        SNAPSHOT_ROOT="$SNAPSHOT_ROOT" \
        SNAPSHOT_LABEL="$label" \
        bash -s <<'EOF'
set -euo pipefail
umask 077

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
snapshot_id="${timestamp}-${SNAPSHOT_LABEL}"
snapshot_dir="${SNAPSHOT_ROOT}/${snapshot_id}"
mkdir -p "$snapshot_dir"

db_path="$(awk -F= '$1 == "DB_PATH" { print substr($0, index($0, "=") + 1); exit }' "$REMOTE_ENV")"
if [[ -z "$db_path" ]]; then
    echo "DB_PATH not found in $REMOTE_ENV" >&2
    exit 1
fi

if [[ -f "$REMOTE_BIN" ]]; then
    cp -f "$REMOTE_BIN" "$snapshot_dir/cc-relayer"
fi
cp -f "$REMOTE_ENV" "$snapshot_dir/cc-relayer.env"
cp -f "$REMOTE_SERVICE" "$snapshot_dir/cc-relayer.service"
sqlite3 "$db_path" ".backup '$snapshot_dir/cc-relayer.db'"

{
    printf 'SNAPSHOT_ID=%s\n' "$snapshot_id"
    printf 'CREATED_AT=%s\n' "$timestamp"
    printf 'SERVICE=%s\n' "$SERVICE"
    printf 'DB_PATH=%s\n' "$db_path"
    printf 'ACTIVE=%s\n' "$(systemctl is-active "$SERVICE" 2>/dev/null || true)"
} >"$snapshot_dir/metadata.env"

sha256sum "$snapshot_dir"/cc-relayer* "$snapshot_dir/cc-relayer.db" >"$snapshot_dir/SHA256SUMS" 2>/dev/null || true
ln -sfn "$snapshot_dir" "${SNAPSHOT_ROOT}/latest"
printf '%s\n' "$snapshot_id"
EOF
}

restore_snapshot() {
    local snapshot_ref="${1:-latest}"
    ssh "$REMOTE" env \
        SERVICE="$SERVICE" \
        REMOTE_BIN="$REMOTE_BIN" \
        REMOTE_ENV="$REMOTE_ENV" \
        REMOTE_SERVICE="$REMOTE_SERVICE" \
        SNAPSHOT_ROOT="$SNAPSHOT_ROOT" \
        SNAPSHOT_REF="$snapshot_ref" \
        bash -s <<'EOF'
set -euo pipefail

resolve_snapshot_dir() {
    local ref="$1"
    if [[ -z "$ref" || "$ref" == "latest" ]]; then
        if [[ -L "${SNAPSHOT_ROOT}/latest" ]]; then
            readlink -f "${SNAPSHOT_ROOT}/latest"
            return
        fi
        find "$SNAPSHOT_ROOT" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | LC_ALL=C sort | tail -n 1 | sed "s#^#${SNAPSHOT_ROOT}/#"
        return
    fi
    if [[ "$ref" == /* ]]; then
        printf '%s\n' "$ref"
        return
    fi
    printf '%s/%s\n' "$SNAPSHOT_ROOT" "$ref"
}

snapshot_dir="$(resolve_snapshot_dir "$SNAPSHOT_REF")"
if [[ -z "$snapshot_dir" || ! -d "$snapshot_dir" ]]; then
    echo "snapshot not found: ${SNAPSHOT_REF}" >&2
    exit 1
fi

cp -f "$snapshot_dir/cc-relayer.env" "$REMOTE_ENV"
db_path="$(awk -F= '$1 == "DB_PATH" { print substr($0, index($0, "=") + 1); exit }' "$REMOTE_ENV")"
if [[ -z "$db_path" ]]; then
    echo "DB_PATH not found in restored env" >&2
    exit 1
fi

mkdir -p "$(dirname "$REMOTE_BIN")" "$(dirname "$REMOTE_ENV")" "$(dirname "$REMOTE_SERVICE")" "$(dirname "$db_path")"
systemctl stop "$SERVICE" || true

rm -f "$db_path" "${db_path}-wal" "${db_path}-shm"
cp -f "$snapshot_dir/cc-relayer.db" "$db_path"
if [[ -f "$snapshot_dir/cc-relayer" ]]; then
    cp -f "$snapshot_dir/cc-relayer" "$REMOTE_BIN"
    chmod +x "$REMOTE_BIN"
fi
cp -f "$snapshot_dir/cc-relayer.service" "$REMOTE_SERVICE"

systemctl daemon-reload
systemctl restart "$SERVICE"

status="$(systemctl is-active "$SERVICE" 2>/dev/null || true)"
if [[ "$status" != "active" ]]; then
    journalctl -u "$SERVICE" -n 20 --no-pager >&2 || true
    exit 1
fi

printf '%s\n' "$(basename "$snapshot_dir")"
EOF
}
