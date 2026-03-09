#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"

# Load .env from repo root (gitignored, holds REMOTE/SITE)
if [[ -f "$REPO_ROOT/.env" ]]; then
    set -a; source "$REPO_ROOT/.env"; set +a
fi

REMOTE="${REMOTE:?Set REMOTE in .env or environment (e.g. user@host)}"
SITE="${SITE:?Set SITE in .env or environment (e.g. https://example.com)}"

detect_service() {
    if [[ -n "${SERVICE:-}" ]]; then
        printf '%s\n' "$SERVICE"
        return
    fi
    if ssh "$REMOTE" "systemctl cat llm-broker >/dev/null 2>&1 || test -f /etc/llm-broker.env || test -f /usr/local/bin/llm-broker"; then
        printf 'llm-broker\n'
        return
    fi
    if ssh "$REMOTE" "systemctl cat cc-relayer >/dev/null 2>&1 || test -f /etc/cc-relayer.env || test -f /usr/local/bin/cc-relayer"; then
        printf 'cc-relayer\n'
        return
    fi
    printf 'llm-broker\n'
}

SERVICE="$(detect_service)"
REMOTE_BIN="${REMOTE_BIN:-/usr/local/bin/${SERVICE}}"
REMOTE_ENV="${REMOTE_ENV:-/etc/${SERVICE}.env}"
REMOTE_SERVICE="${REMOTE_SERVICE:-/etc/systemd/system/${SERVICE}.service}"
REMOTE_DB_PATH="${REMOTE_DB_PATH:-/var/lib/${SERVICE}/${SERVICE}.db}"
SNAPSHOT_ROOT="${SNAPSHOT_ROOT:-/var/backups/${SERVICE}}"
TMP_LOCAL="${TMP_LOCAL:-/tmp/${SERVICE}-new}"
TMP_REMOTE="${TMP_REMOTE:-/tmp/${SERVICE}-new}"
ARTIFACT_PREFIX="${ARTIFACT_PREFIX:-${SERVICE}}"
ALT_ARTIFACT_PREFIX="${ALT_ARTIFACT_PREFIX:-}"

if [[ -z "$ALT_ARTIFACT_PREFIX" ]]; then
    if [[ "$ARTIFACT_PREFIX" == "llm-broker" ]]; then
        ALT_ARTIFACT_PREFIX="cc-relayer"
    else
        ALT_ARTIFACT_PREFIX="llm-broker"
    fi
fi

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
        ARTIFACT_PREFIX="$ARTIFACT_PREFIX" \
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
    cp -f "$REMOTE_BIN" "$snapshot_dir/$ARTIFACT_PREFIX"
fi
cp -f "$REMOTE_ENV" "$snapshot_dir/$ARTIFACT_PREFIX.env"
cp -f "$REMOTE_SERVICE" "$snapshot_dir/$ARTIFACT_PREFIX.service"
sqlite3 "$db_path" ".backup '$snapshot_dir/$ARTIFACT_PREFIX.db'"

{
    printf 'SNAPSHOT_ID=%s\n' "$snapshot_id"
    printf 'CREATED_AT=%s\n' "$timestamp"
    printf 'SERVICE=%s\n' "$SERVICE"
    printf 'DB_PATH=%s\n' "$db_path"
    printf 'ACTIVE=%s\n' "$(systemctl is-active "$SERVICE" 2>/dev/null || true)"
} >"$snapshot_dir/metadata.env"

sha256sum "$snapshot_dir"/"$ARTIFACT_PREFIX"* "$snapshot_dir/$ARTIFACT_PREFIX.db" >"$snapshot_dir/SHA256SUMS" 2>/dev/null || true
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
        REMOTE_DB_PATH="$REMOTE_DB_PATH" \
        SNAPSHOT_ROOT="$SNAPSHOT_ROOT" \
        ARTIFACT_PREFIX="$ARTIFACT_PREFIX" \
        ALT_ARTIFACT_PREFIX="$ALT_ARTIFACT_PREFIX" \
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

snapshot_file() {
    local dir="$1"
    local suffix="$2"
    local path
    for prefix in "$ARTIFACT_PREFIX" "$ALT_ARTIFACT_PREFIX"; do
        path="${dir}/${prefix}${suffix}"
        if [[ -f "$path" ]]; then
            printf '%s\n' "$path"
            return 0
        fi
    done
    return 1
}

snapshot_dir="$(resolve_snapshot_dir "$SNAPSHOT_REF")"
if [[ -z "$snapshot_dir" || ! -d "$snapshot_dir" ]]; then
    echo "snapshot not found: ${SNAPSHOT_REF}" >&2
    exit 1
fi

env_snapshot="$(snapshot_file "$snapshot_dir" ".env")"
db_snapshot="$(snapshot_file "$snapshot_dir" ".db")"
binary_snapshot="$(snapshot_file "$snapshot_dir" "" || true)"

mkdir -p "$(dirname "$REMOTE_BIN")" "$(dirname "$REMOTE_ENV")" "$(dirname "$REMOTE_SERVICE")" "$(dirname "$REMOTE_DB_PATH")"

awk -F= -v db_path="$REMOTE_DB_PATH" '
    BEGIN { wrote = 0 }
    $1 == "DB_PATH" { print "DB_PATH=" db_path; wrote = 1; next }
    { print }
    END { if (!wrote) print "DB_PATH=" db_path }
' "$env_snapshot" > "$REMOTE_ENV"

systemctl stop "$SERVICE" || true

rm -f "$REMOTE_DB_PATH" "${REMOTE_DB_PATH}-wal" "${REMOTE_DB_PATH}-shm"
cp -f "$db_snapshot" "$REMOTE_DB_PATH"
if [[ -n "$binary_snapshot" ]]; then
    cp -f "$binary_snapshot" "$REMOTE_BIN"
    chmod +x "$REMOTE_BIN"
fi

cat > "$REMOTE_SERVICE" <<UNIT
[Unit]
Description=broker service
After=network.target

[Service]
Type=simple
EnvironmentFile=$REMOTE_ENV
ExecStart=$REMOTE_BIN
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT

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
