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

BLUEGREEN_STATE_DIR="${BLUEGREEN_STATE_DIR:-/var/lib/${SERVICE}/bluegreen}"
BLUEGREEN_LAYOUT_FILE="${BLUEGREEN_LAYOUT_FILE:-${BLUEGREEN_STATE_DIR}/layout.env}"
BLUEGREEN_ACTIVE_SLOT_FILE="${BLUEGREEN_ACTIVE_SLOT_FILE:-${BLUEGREEN_STATE_DIR}/active-slot}"
BLUEGREEN_UPSTREAM_FILE="${BLUEGREEN_UPSTREAM_FILE:-/etc/caddy/${SERVICE}.upstream}"
CADDYFILE_PATH="${CADDYFILE_PATH:-/etc/caddy/Caddyfile}"

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

load_bluegreen_layout() {
    local tmp_file
    tmp_file="$(mktemp)"
    ssh "$REMOTE" env \
        BLUEGREEN_LAYOUT_FILE="$BLUEGREEN_LAYOUT_FILE" \
        BLUEGREEN_ACTIVE_SLOT_FILE="$BLUEGREEN_ACTIVE_SLOT_FILE" \
        bash -s <<'EOF' >"$tmp_file"
set -euo pipefail
if [[ ! -f "$BLUEGREEN_LAYOUT_FILE" ]]; then
    echo "missing blue-green layout: $BLUEGREEN_LAYOUT_FILE" >&2
    exit 1
fi
cat "$BLUEGREEN_LAYOUT_FILE"
if [[ -f "$BLUEGREEN_ACTIVE_SLOT_FILE" ]]; then
    printf 'ACTIVE_SLOT=%s\n' "$(tr -d '\n' < "$BLUEGREEN_ACTIVE_SLOT_FILE")"
fi
EOF
    set -a
    # shellcheck disable=SC1090
    source "$tmp_file"
    set +a
    rm -f "$tmp_file"
}

wait_for_remote_local_health() {
    local port="$1"
    local attempts="${2:-30}"
    ssh "$REMOTE" env PORT="$port" ATTEMPTS="$attempts" bash -s <<'EOF'
set -euo pipefail
for ((i = 1; i <= ATTEMPTS; i++)); do
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:${PORT}/health" 2>/dev/null || echo "000")"
    if [[ "$code" == "200" ]]; then
        exit 0
    fi
    sleep 1
done
echo "local /health on port ${PORT} did not return 200" >&2
exit 1
EOF
}

remote_reload_caddy() {
    ssh "$REMOTE" env CADDYFILE_PATH="$CADDYFILE_PATH" bash -s <<'EOF'
set -euo pipefail
if systemctl reload caddy >/dev/null 2>&1; then
    exit 0
fi
if command -v caddy >/dev/null 2>&1; then
    caddy reload --config "$CADDYFILE_PATH"
    exit 0
fi
echo "unable to reload caddy" >&2
exit 1
EOF
}

detect_remote_deploy_strategy() {
    ssh "$REMOTE" env BLUEGREEN_LAYOUT_FILE="$BLUEGREEN_LAYOUT_FILE" bash -s <<'EOF'
set -euo pipefail
if [[ -f "$BLUEGREEN_LAYOUT_FILE" ]]; then
    printf 'bluegreen\n'
else
    printf 'legacy\n'
fi
EOF
}

build_release_artifact() {
    cd "$REPO_ROOT"

    if [[ "${SKIP_FRONTEND:-}" != "1" ]]; then
        echo "==> building frontend..."
        (cd web && npm run build --silent 2>&1) | tail -1
        echo "    done"
    else
        echo "==> skipping frontend build (SKIP_FRONTEND=1)"
    fi

    echo "==> compiling linux/amd64..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$TMP_LOCAL" ./cmd/relay/
    local size
    size="$(du -h "$TMP_LOCAL" | cut -f1 | xargs)"
    echo "    done ($size)"
}

upload_candidate_binary() {
    echo "==> uploading to $REMOTE..."
    scp -q "$TMP_LOCAL" "$REMOTE:$TMP_REMOTE"
    echo "    done"
}

run_uploaded_binary_migrate() {
    echo "==> running migrate..."
    ssh "$REMOTE" env TMP_REMOTE="$TMP_REMOTE" REMOTE_ENV="$REMOTE_ENV" bash -s <<'EOF'
set -euo pipefail
chmod +x "$TMP_REMOTE"
set -a
. "$REMOTE_ENV"
set +a
"$TMP_REMOTE" migrate
EOF
    echo "    done"
}

wait_for_site_health() {
    local attempts="${1:-30}"
    local code="000"
    for ((i = 1; i <= attempts; i++)); do
        code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$SITE/health" 2>/dev/null || echo "000")"
        if [[ "$code" == "200" ]]; then
            return 0
        fi
        sleep 1
    done
    echo "    FAIL: /health did not return 200 (last=$code)"
    return 1
}

query_db_invariants() {
    ssh "$REMOTE" env REMOTE_ENV="$REMOTE_ENV" bash -s <<'EOF'
set -euo pipefail
db_path="$(awk -F= '$1 == "DB_PATH" { print substr($0, index($0, "=") + 1); exit }' "$REMOTE_ENV")"
if [[ -z "$db_path" || ! -f "$db_path" ]]; then
    echo "missing|0|0|0|0"
    exit 0
fi
sqlite3 "$db_path" <<'SQL'
.mode list
.separator |
SELECT
    'ok',
    (SELECT COUNT(*) FROM accounts),
    (SELECT COUNT(*) FROM quota_buckets),
    (SELECT COUNT(*) FROM accounts WHERE subject = ''),
    (SELECT COUNT(DISTINCT CASE
        WHEN bucket_key != '' THEN bucket_key
        WHEN subject != '' THEN provider || ':' || subject
        ELSE provider || ':' || id
    END) FROM accounts);
SQL
EOF
}

query_orphan_buckets() {
    ssh "$REMOTE" env REMOTE_ENV="$REMOTE_ENV" bash -s <<'EOF'
set -euo pipefail
db_path="$(awk -F= '$1 == "DB_PATH" { print substr($0, index($0, "=") + 1); exit }' "$REMOTE_ENV")"
if [[ -z "$db_path" || ! -f "$db_path" ]]; then
    exit 0
fi
sqlite3 "$db_path" <<'SQL'
.mode list
.separator |
WITH effective AS (
    SELECT DISTINCT CASE
        WHEN bucket_key != '' THEN bucket_key
        WHEN subject != '' THEN provider || ':' || subject
        ELSE provider || ':' || id
    END AS bucket_key
    FROM accounts
)
SELECT bucket_key
FROM quota_buckets
EXCEPT
SELECT bucket_key FROM effective;
SQL
EOF
}

verify_db_invariants() {
    echo "==> verifying database invariants..."

    local db_flag=""
    local account_count=0
    local bucket_count=0
    local empty_subject_count=0
    local distinct_bucket_count=0

    for ((attempt = 1; attempt <= 20; attempt++)); do
        local db_check
        db_check="$(query_db_invariants)"
        IFS='|' read -r db_flag account_count bucket_count empty_subject_count distinct_bucket_count <<<"$db_check"
        if [[ "$db_flag" == "ok" && "$empty_subject_count" == "0" && "$bucket_count" == "$distinct_bucket_count" ]]; then
            break
        fi
        sleep 1
    done

    echo "    accounts=$account_count buckets=$bucket_count distinct_bucket_keys=$distinct_bucket_count empty_subjects=$empty_subject_count"
    if [[ "$db_flag" != "ok" ]]; then
        echo "    FAIL: database file missing"
        return 1
    fi
    if [[ "$empty_subject_count" != "0" ]]; then
        echo "    FAIL: accounts with empty subject detected"
        return 1
    fi
    if [[ "$bucket_count" != "$distinct_bucket_count" ]]; then
        echo "    FAIL: quota_buckets count does not match distinct effective bucket keys"
        local orphan_buckets
        orphan_buckets="$(query_orphan_buckets || true)"
        if [[ -n "$orphan_buckets" ]]; then
            echo "    orphan buckets:"
            while IFS= read -r bucket; do
                [[ -n "$bucket" ]] && echo "      - $bucket"
            done <<<"$orphan_buckets"
        fi
        return 1
    fi
}

assert_remote_service_active() {
    local service_name="$1"
    local status

    status="$(ssh "$REMOTE" "systemctl is-active $service_name 2>/dev/null || true")"
    if [[ "$status" == "active" ]]; then
        return 0
    fi

    echo "    FAIL: service is $status"
    ssh "$REMOTE" "journalctl -u $service_name -n 15 --no-pager"
    return 1
}

show_recent_restart_events() {
    if [[ "$#" -eq 0 ]]; then
        return 0
    fi
    if [[ "${DEPLOY_VERBOSE:-0}" != "1" ]]; then
        return 0
    fi

    ssh "$REMOTE" env UNITS="$*" bash -s <<'EOF' || true
set -euo pipefail
read -r -a units <<<"$UNITS"
args=()
for unit in "${units[@]}"; do
    args+=(-u "$unit")
done
journalctl "${args[@]}" --since '2 minutes ago' --no-pager -o short-precise | grep -E '(Stopping|Stopped|Started|server starting)' || true
EOF
}

smoke_endpoint() {
    local label="$1"
    local url="$2"
    local auth="${3:-}"
    local expect="${4:-200}"
    local args=(-s -o /dev/null -w '%{http_code}' --max-time 10)

    if [[ -n "$auth" ]]; then
        args+=(-H "Authorization: Bearer $auth")
    fi

    local code
    code="$(curl "${args[@]}" "$url" 2>/dev/null || echo "000")"
    if [[ "$code" == "$expect" ]]; then
        echo "    + $label ($code)"
        return 0
    fi

    echo "    x $label (got $code, expected $expect)"
    return 1
}

run_nonfatal_smoke_suite() {
    local snapshot_id="${1:-}"

    echo ""
    echo "==> smoke testing endpoints..."

    local api_token
    api_token="$(remote_env_value API_TOKEN)"

    local smoke_fail=0
    smoke_endpoint "GET /health" "$SITE/health" || smoke_fail=1
    smoke_endpoint "GET /v1/models" "$SITE/v1/models" "" 401 || smoke_fail=1

    if [[ -n "$api_token" ]]; then
        smoke_endpoint "GET /v1/models (auth)" "$SITE/v1/models" "$api_token" || smoke_fail=1
        smoke_endpoint "GET /admin/dashboard" "$SITE/admin/dashboard" "$api_token" || smoke_fail=1
        smoke_endpoint "GET /admin/accounts" "$SITE/admin/accounts" "$api_token" || smoke_fail=1
        smoke_endpoint "GET /admin/users" "$SITE/admin/users" "$api_token" || smoke_fail=1
        smoke_endpoint "GET /admin/health" "$SITE/admin/health" "$api_token" || smoke_fail=1
    else
        echo "    skipping authenticated endpoints (API_TOKEN not found on remote)"
    fi

    smoke_endpoint "GET /" "$SITE/" || smoke_fail=1
    smoke_endpoint "GET /dashboard" "$SITE/dashboard" || smoke_fail=1
    smoke_endpoint "GET /add-account/claude" "$SITE/add-account/claude" || smoke_fail=1
    smoke_endpoint "GET /add-account" "$SITE/add-account" "" 404 || smoke_fail=1
    smoke_endpoint "GET /ui/" "$SITE/ui/" "" 404 || smoke_fail=1
    smoke_endpoint "GET /ui/add-account" "$SITE/ui/add-account" "" 404 || smoke_fail=1

    if [[ "$smoke_fail" -eq 1 ]]; then
        echo ""
        echo "==> smoke test failures detected"
        if [[ -n "$snapshot_id" ]]; then
            echo "    rollback: bash $SCRIPT_DIR/restore.sh $snapshot_id"
        fi
    else
        echo "    all endpoints OK"
    fi

    if [[ -d "$REPO_ROOT/web/node_modules/playwright-core" ]]; then
        echo ""
        echo "==> browser smoke test..."
        if ! SITE="$SITE" API_TOKEN="$api_token" node "$REPO_ROOT/web/smoke.mjs"; then
            echo "==> browser smoke found JS errors; check output above"
        fi
    else
        echo ""
        echo "    skipping browser smoke (run: cd web && npm i && npx playwright install chromium)"
    fi
}

provision_bluegreen_layout() {
    local legacy_port="$1"
    local blue_port="$2"
    local green_port="$3"

    echo "==> provisioning blue-green layout..."
    ssh "$REMOTE" env \
        SERVICE="$SERVICE" \
        REMOTE_BIN="$REMOTE_BIN" \
        REMOTE_ENV="$REMOTE_ENV" \
        BLUEGREEN_STATE_DIR="$BLUEGREEN_STATE_DIR" \
        BLUEGREEN_LAYOUT_FILE="$BLUEGREEN_LAYOUT_FILE" \
        BLUEGREEN_ACTIVE_SLOT_FILE="$BLUEGREEN_ACTIVE_SLOT_FILE" \
        BLUEGREEN_UPSTREAM_FILE="$BLUEGREEN_UPSTREAM_FILE" \
        CADDYFILE_PATH="$CADDYFILE_PATH" \
        LEGACY_PORT="$legacy_port" \
        BLUE_PORT="$blue_port" \
        GREEN_PORT="$green_port" \
        bash -s <<'EOF'
set -euo pipefail

blue_slot="blue"
green_slot="green"
blue_service="${SERVICE}-${blue_slot}"
green_service="${SERVICE}-${green_slot}"
blue_bin="/usr/local/bin/${SERVICE}-${blue_slot}"
green_bin="/usr/local/bin/${SERVICE}-${green_slot}"
blue_service_unit="/etc/systemd/system/${blue_service}.service"
green_service_unit="/etc/systemd/system/${green_service}.service"
leader_lock="/run/${SERVICE}/background.lock"

mkdir -p "$BLUEGREEN_STATE_DIR" "$(dirname "$BLUEGREEN_UPSTREAM_FILE")"
install -m 755 "$REMOTE_BIN" "$blue_bin"
install -m 755 "$REMOTE_BIN" "$green_bin"

cat >"$BLUEGREEN_LAYOUT_FILE" <<LAYOUT
BLUE_SLOT=$blue_slot
GREEN_SLOT=$green_slot
BLUE_PORT=$BLUE_PORT
GREEN_PORT=$GREEN_PORT
BLUE_SERVICE=$blue_service
GREEN_SERVICE=$green_service
BLUE_BIN=$blue_bin
GREEN_BIN=$green_bin
BLUE_SERVICE_UNIT=$blue_service_unit
GREEN_SERVICE_UNIT=$green_service_unit
UPSTREAM_FILE=$BLUEGREEN_UPSTREAM_FILE
CADDYFILE_PATH=$CADDYFILE_PATH
LEGACY_SERVICE=$SERVICE
LEGACY_PORT=$LEGACY_PORT
LEADER_LOCK=$leader_lock
LAYOUT

write_unit() {
    local service_name="$1"
    local bin_path="$2"
    local port="$3"
    local unit_path="$4"
    cat >"$unit_path" <<UNIT
[Unit]
Description=${service_name}
After=network.target

[Service]
Type=simple
EnvironmentFile=$REMOTE_ENV
ExecStart=/usr/bin/env PORT=${port} BACKGROUND_JOBS_MODE=leader BACKGROUND_LEADER_LOCK_PATH=${leader_lock} ${bin_path}
Restart=always
RestartSec=5
RuntimeDirectory=${SERVICE}
RuntimeDirectoryMode=0755
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT
}

write_unit "$blue_service" "$blue_bin" "$BLUE_PORT" "$blue_service_unit"
write_unit "$green_service" "$green_bin" "$GREEN_PORT" "$green_service_unit"

    if [[ ! -f "$CADDYFILE_PATH" ]]; then
        echo "missing caddyfile: $CADDYFILE_PATH" >&2
        exit 1
    fi

    python3 - "$CADDYFILE_PATH" "$LEGACY_PORT" "$BLUEGREEN_UPSTREAM_FILE" "$BLUE_PORT" <<'PY'
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
legacy_port = sys.argv[2]
upstream_path = pathlib.Path(sys.argv[3])
blue_port = sys.argv[4]
text = path.read_text()

import_line = f"import {upstream_path}"
if import_line in text:
    if not upstream_path.exists():
        upstream_path.write_text(f"reverse_proxy 127.0.0.1:{blue_port}\n")
    sys.exit(0)

lines = text.splitlines(keepends=True)
target_re = re.compile(rf'^([ \t]*)reverse_proxy[ \t]+(?:localhost|127\.0\.0\.1|0\.0\.0\.0):{re.escape(legacy_port)}(?:[ \t].*)?$')

start = None
indent = ""
for idx, line in enumerate(lines):
    match = target_re.match(line)
    if match:
        start = idx
        indent = match.group(1)
        break

if start is None:
    sys.exit(f"reverse_proxy target for port {legacy_port} not found")

brace_depth = lines[start].count("{") - lines[start].count("}")
end = start + 1
while brace_depth > 0 and end < len(lines):
    brace_depth += lines[end].count("{") - lines[end].count("}")
    end += 1

directive = "".join(lines[start:end])
upstream = re.sub(
    rf'(localhost|127\.0\.0\.1|0\.0\.0\.0):{re.escape(legacy_port)}',
    f'127.0.0.1:{blue_port}',
    directive,
    count=1,
)
if not upstream.endswith("\n"):
    upstream += "\n"
upstream_path.write_text(upstream)

replacement = f"{indent}import {upstream_path}\n"
updated = "".join(lines[:start]) + replacement + "".join(lines[end:])
path.write_text(updated)
PY

systemctl daemon-reload
systemctl enable "$blue_service" "$green_service" >/dev/null 2>&1 || true
systemctl restart "$blue_service"

for ((i = 1; i <= 30; i++)); do
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:${BLUE_PORT}/health" 2>/dev/null || echo "000")"
    if [[ "$code" == "200" ]]; then
        exit 0
    fi
    sleep 1
done

echo "blue slot did not become healthy on port ${BLUE_PORT}" >&2
exit 1
EOF
}

set_bluegreen_active_slot() {
    local next_slot="$1"
    local next_port="$2"

    ssh "$REMOTE" env \
        BLUEGREEN_UPSTREAM_FILE="$BLUEGREEN_UPSTREAM_FILE" \
        BLUEGREEN_ACTIVE_SLOT_FILE="$BLUEGREEN_ACTIVE_SLOT_FILE" \
        NEXT_PORT="$next_port" \
        NEXT_SLOT="$next_slot" \
        bash -s <<'EOF'
set -euo pipefail
mkdir -p "$(dirname "$BLUEGREEN_UPSTREAM_FILE")" "$(dirname "$BLUEGREEN_ACTIVE_SLOT_FILE")"
python3 - "$BLUEGREEN_UPSTREAM_FILE" "$NEXT_PORT" <<'PY'
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
next_port = sys.argv[2]
if not path.exists():
    path.write_text(f"reverse_proxy 127.0.0.1:{next_port}\n")
    sys.exit(0)

text = path.read_text()
updated, count = re.subn(
    r'(reverse_proxy[ \t]+)(?:localhost|127\.0\.0\.1|0\.0\.0\.0):\d+',
    rf'\g<1>127.0.0.1:{next_port}',
    text,
    count=1,
)
if count == 0:
    raise SystemExit("reverse_proxy target not found in upstream file")
if not updated.endswith("\n"):
    updated += "\n"
path.write_text(updated)
PY
printf '%s\n' "$NEXT_SLOT" >"$BLUEGREEN_ACTIVE_SLOT_FILE"
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
        BLUEGREEN_LAYOUT_FILE="$BLUEGREEN_LAYOUT_FILE" \
        BLUEGREEN_ACTIVE_SLOT_FILE="$BLUEGREEN_ACTIVE_SLOT_FILE" \
        BLUEGREEN_UPSTREAM_FILE="$BLUEGREEN_UPSTREAM_FILE" \
        CADDYFILE_PATH="$CADDYFILE_PATH" \
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
if [[ -f "$CADDYFILE_PATH" ]]; then
    cp -f "$CADDYFILE_PATH" "$snapshot_dir/$ARTIFACT_PREFIX.caddy"
fi
if [[ -f "$BLUEGREEN_UPSTREAM_FILE" ]]; then
    cp -f "$BLUEGREEN_UPSTREAM_FILE" "$snapshot_dir/$ARTIFACT_PREFIX.upstream"
fi

if [[ -f "$BLUEGREEN_LAYOUT_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$BLUEGREEN_LAYOUT_FILE"
    set +a

    bg_dir="$snapshot_dir/bluegreen"
    mkdir -p "$bg_dir"
    cp -f "$BLUEGREEN_LAYOUT_FILE" "$bg_dir/layout.env"
    if [[ -f "$BLUEGREEN_ACTIVE_SLOT_FILE" ]]; then
        cp -f "$BLUEGREEN_ACTIVE_SLOT_FILE" "$bg_dir/active-slot"
    fi
    if [[ -f "$BLUEGREEN_UPSTREAM_FILE" ]]; then
        cp -f "$BLUEGREEN_UPSTREAM_FILE" "$bg_dir/upstream"
    fi
    if [[ -f "$CADDYFILE_PATH" ]]; then
        cp -f "$CADDYFILE_PATH" "$bg_dir/Caddyfile"
    fi
    if [[ -n "${BLUE_BIN:-}" && -f "${BLUE_BIN:-}" ]]; then
        cp -f "$BLUE_BIN" "$bg_dir/blue.bin"
    fi
    if [[ -n "${GREEN_BIN:-}" && -f "${GREEN_BIN:-}" ]]; then
        cp -f "$GREEN_BIN" "$bg_dir/green.bin"
    fi
    if [[ -n "${BLUE_SERVICE_UNIT:-}" && -f "${BLUE_SERVICE_UNIT:-}" ]]; then
        cp -f "$BLUE_SERVICE_UNIT" "$bg_dir/blue.service"
    fi
    if [[ -n "${GREEN_SERVICE_UNIT:-}" && -f "${GREEN_SERVICE_UNIT:-}" ]]; then
        cp -f "$GREEN_SERVICE_UNIT" "$bg_dir/green.service"
    fi
fi

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
        BLUEGREEN_STATE_DIR="$BLUEGREEN_STATE_DIR" \
        BLUEGREEN_LAYOUT_FILE="$BLUEGREEN_LAYOUT_FILE" \
        BLUEGREEN_ACTIVE_SLOT_FILE="$BLUEGREEN_ACTIVE_SLOT_FILE" \
        BLUEGREEN_UPSTREAM_FILE="$BLUEGREEN_UPSTREAM_FILE" \
        CADDYFILE_PATH="$CADDYFILE_PATH" \
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
caddy_snapshot="$(snapshot_file "$snapshot_dir" ".caddy" || true)"
upstream_snapshot="$(snapshot_file "$snapshot_dir" ".upstream" || true)"
bluegreen_dir="$snapshot_dir/bluegreen"
bluegreen_layout_snapshot="$bluegreen_dir/layout.env"

mkdir -p "$(dirname "$REMOTE_BIN")" "$(dirname "$REMOTE_ENV")" "$(dirname "$REMOTE_SERVICE")" "$(dirname "$REMOTE_DB_PATH")"

if [[ -f "$bluegreen_layout_snapshot" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$bluegreen_layout_snapshot"
    set +a

    mkdir -p "$(dirname "$BLUEGREEN_LAYOUT_FILE")" "$(dirname "$BLUEGREEN_ACTIVE_SLOT_FILE")" "$(dirname "$BLUEGREEN_UPSTREAM_FILE")"
    mkdir -p "$(dirname "$BLUE_BIN")" "$(dirname "$GREEN_BIN")" "$(dirname "$BLUE_SERVICE_UNIT")" "$(dirname "$GREEN_SERVICE_UNIT")"

    awk -F= -v db_path="$REMOTE_DB_PATH" '
        BEGIN { wrote = 0 }
        $1 == "DB_PATH" { print "DB_PATH=" db_path; wrote = 1; next }
        { print }
        END { if (!wrote) print "DB_PATH=" db_path }
    ' "$env_snapshot" > "$REMOTE_ENV"

    systemctl stop "$SERVICE" >/dev/null 2>&1 || true
    [[ -n "${BLUE_SERVICE:-}" ]] && systemctl stop "$BLUE_SERVICE" >/dev/null 2>&1 || true
    [[ -n "${GREEN_SERVICE:-}" ]] && systemctl stop "$GREEN_SERVICE" >/dev/null 2>&1 || true

    rm -f "$REMOTE_DB_PATH" "${REMOTE_DB_PATH}-wal" "${REMOTE_DB_PATH}-shm"
    cp -f "$db_snapshot" "$REMOTE_DB_PATH"
    if [[ -n "$binary_snapshot" ]]; then
        cp -f "$binary_snapshot" "$REMOTE_BIN"
        chmod +x "$REMOTE_BIN"
    fi

    cp -f "$bluegreen_layout_snapshot" "$BLUEGREEN_LAYOUT_FILE"
    [[ -f "$bluegreen_dir/active-slot" ]] && cp -f "$bluegreen_dir/active-slot" "$BLUEGREEN_ACTIVE_SLOT_FILE"
    [[ -f "$bluegreen_dir/upstream" ]] && cp -f "$bluegreen_dir/upstream" "$BLUEGREEN_UPSTREAM_FILE"
    [[ -f "$bluegreen_dir/Caddyfile" ]] && cp -f "$bluegreen_dir/Caddyfile" "$CADDYFILE_PATH"
    [[ -f "$bluegreen_dir/blue.bin" ]] && install -m 755 "$bluegreen_dir/blue.bin" "$BLUE_BIN"
    [[ -f "$bluegreen_dir/green.bin" ]] && install -m 755 "$bluegreen_dir/green.bin" "$GREEN_BIN"
    [[ -f "$bluegreen_dir/blue.service" ]] && cp -f "$bluegreen_dir/blue.service" "$BLUE_SERVICE_UNIT"
    [[ -f "$bluegreen_dir/green.service" ]] && cp -f "$bluegreen_dir/green.service" "$GREEN_SERVICE_UNIT"

    systemctl daemon-reload

    if [[ -f "$BLUEGREEN_UPSTREAM_FILE" || -f "$CADDYFILE_PATH" ]]; then
        if systemctl reload caddy >/dev/null 2>&1; then
            :
        elif command -v caddy >/dev/null 2>&1; then
            caddy reload --config "$CADDYFILE_PATH"
        fi
    fi

    active_slot="$BLUE_SLOT"
    if [[ -f "$BLUEGREEN_ACTIVE_SLOT_FILE" ]]; then
        active_slot="$(tr -d '\n' < "$BLUEGREEN_ACTIVE_SLOT_FILE")"
    fi
    active_service="$BLUE_SERVICE"
    active_port="$BLUE_PORT"
    if [[ "$active_slot" == "$GREEN_SLOT" ]]; then
        active_service="$GREEN_SERVICE"
        active_port="$GREEN_PORT"
    fi

    systemctl restart "$active_service"
    status="$(systemctl is-active "$active_service" 2>/dev/null || true)"
    if [[ "$status" != "active" ]]; then
        journalctl -u "$active_service" -n 20 --no-pager >&2 || true
        exit 1
    fi

    for ((i = 1; i <= 30; i++)); do
        code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:${active_port}/health" 2>/dev/null || echo "000")"
        if [[ "$code" == "200" ]]; then
            printf '%s\n' "$(basename "$snapshot_dir")"
            exit 0
        fi
        sleep 1
    done
    echo "blue-green restore health check failed on port ${active_port}" >&2
    exit 1
fi

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

legacy_blue_service="${SERVICE}-blue"
legacy_green_service="${SERVICE}-green"
legacy_blue_unit="/etc/systemd/system/${legacy_blue_service}.service"
legacy_green_unit="/etc/systemd/system/${legacy_green_service}.service"
legacy_blue_bin="/usr/local/bin/${SERVICE}-blue"
legacy_green_bin="/usr/local/bin/${SERVICE}-green"

systemctl stop "$legacy_blue_service" "$legacy_green_service" >/dev/null 2>&1 || true
systemctl disable "$legacy_blue_service" "$legacy_green_service" >/dev/null 2>&1 || true
rm -f "$legacy_blue_unit" "$legacy_green_unit"
rm -f "$legacy_blue_bin" "$legacy_green_bin"
rm -f "$BLUEGREEN_UPSTREAM_FILE" "$BLUEGREEN_ACTIVE_SLOT_FILE"
rm -rf "$BLUEGREEN_STATE_DIR"
if [[ -n "$caddy_snapshot" ]]; then
    cp -f "$caddy_snapshot" "$CADDYFILE_PATH"
fi
if [[ -n "$upstream_snapshot" ]]; then
    cp -f "$upstream_snapshot" "$BLUEGREEN_UPSTREAM_FILE"
fi

systemctl daemon-reload
if [[ -n "$caddy_snapshot" ]]; then
    if systemctl reload caddy >/dev/null 2>&1; then
        :
    elif command -v caddy >/dev/null 2>&1; then
        caddy reload --config "$CADDYFILE_PATH"
    fi
fi
systemctl enable "$SERVICE" >/dev/null 2>&1 || true
systemctl restart "$SERVICE"

status="$(systemctl is-active "$SERVICE" 2>/dev/null || true)"
if [[ "$status" != "active" ]]; then
    journalctl -u "$SERVICE" -n 20 --no-pager >&2 || true
    exit 1
fi

printf '%s\n' "$(basename "$snapshot_dir")"
EOF
}
