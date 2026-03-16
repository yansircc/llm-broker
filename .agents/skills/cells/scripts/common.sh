#!/usr/bin/env bash
set -euo pipefail

CELL_SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$CELL_SCRIPT_DIR" rev-parse --show-toplevel)"
source "$REPO_ROOT/.claude/skills/deploy/scripts/common.sh"

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

note() {
    printf '==> %s\n' "$*" >&2
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

slugify() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9._-' '-'
}

labels_json() {
    if [[ "$#" -eq 0 ]]; then
        printf '{}'
        return
    fi

    printf '%s\n' "$@" | jq -Rn '
        [inputs | select(length > 0) | split("=")] as $pairs
        | reduce $pairs[] as $pair (
            {};
            .[$pair[0]] = (
                if ($pair | length) > 1 then
                    ($pair[1:] | join("="))
                else
                    ""
                end
            )
        )
    '
}

broker_api_token() {
    local token
    token="$(remote_env_value API_TOKEN)"
    [[ -n "$token" ]] || die "API_TOKEN not found in $REMOTE_ENV on $REMOTE"
    printf '%s' "$token"
}

admin_api() {
    local method="$1"
    local path="$2"
    local body="${3:-}"
    local token

    token="$(broker_api_token)"
    if [[ -n "$body" ]]; then
        curl -fsS -X "$method" \
            -H "Authorization: Bearer $token" \
            -H "Content-Type: application/json" \
            --data "$body" \
            "$SITE$path"
        return
    fi

    curl -fsS -X "$method" \
        -H "Authorization: Bearer $token" \
        "$SITE$path"
}

fetch_cell_json() {
    local cell_id="$1"
    admin_api GET "/admin/egress/cells" | jq -e --arg id "$cell_id" '.[] | select(.id == $id)'
}
