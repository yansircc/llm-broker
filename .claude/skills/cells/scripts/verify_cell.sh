#!/usr/bin/env bash
set -euo pipefail

CELL_SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$CELL_SCRIPT_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage:
  bash .claude/skills/cells/scripts/verify_cell.sh --cell-id CELL_ID [--expect-ip IPV6]

  or

  bash .claude/skills/cells/scripts/verify_cell.sh \
    --proxy-type socks5|http|https \
    --proxy-host HOST \
    --proxy-port PORT \
    [--proxy-username USERNAME] \
    [--proxy-password PASSWORD] \
    [--expect-ip IPV6]

Options:
  --url URL         Defaults to https://api64.ipify.org
  --timeout SEC     Defaults to 20
EOF
}

require_cmd curl
require_cmd jq
require_cmd ssh

cell_id=""
proxy_type=""
proxy_host=""
proxy_port=""
proxy_username=""
proxy_password=""
url="https://api64.ipify.org"
expect_ip=""
timeout="20"

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --cell-id) cell_id="${2:-}"; shift 2 ;;
        --proxy-type) proxy_type="${2:-}"; shift 2 ;;
        --proxy-host) proxy_host="${2:-}"; shift 2 ;;
        --proxy-port) proxy_port="${2:-}"; shift 2 ;;
        --proxy-username) proxy_username="${2:-}"; shift 2 ;;
        --proxy-password) proxy_password="${2:-}"; shift 2 ;;
        --url) url="${2:-}"; shift 2 ;;
        --expect-ip) expect_ip="${2:-}"; shift 2 ;;
        --timeout) timeout="${2:-}"; shift 2 ;;
        --help|-h) usage; exit 0 ;;
        *) die "unknown argument: $1" ;;
    esac
done

if [[ -n "$cell_id" ]]; then
    note "loading proxy config from cell $cell_id"
    cell_json="$(fetch_cell_json "$cell_id")" || die "cell not found: $cell_id"
    proxy_type="$(jq -r '.proxy.type // empty' <<<"$cell_json")"
    proxy_host="$(jq -r '.proxy.host // empty' <<<"$cell_json")"
    proxy_port="$(jq -r '.proxy.port // empty' <<<"$cell_json")"
    proxy_username="$(jq -r '.proxy.username // empty' <<<"$cell_json")"
    proxy_password="$(jq -r '.proxy.password // empty' <<<"$cell_json")"
fi

[[ -n "$proxy_type" ]] || die "proxy type is required"
[[ -n "$proxy_host" ]] || die "proxy host is required"
[[ -n "$proxy_port" ]] || die "proxy port is required"
[[ "$proxy_port" =~ ^[0-9]+$ ]] || die "proxy port must be numeric"
[[ "$timeout" =~ ^[0-9]+$ ]] || die "--timeout must be numeric"
[[ -z "$proxy_password" || -n "$proxy_username" ]] || die "--proxy-password requires --proxy-username"

case "$proxy_type" in
    socks5) proxy_scheme="socks5h" ;;
    http|https) proxy_scheme="$proxy_type" ;;
    *) die "--proxy-type must be socks5, http, or https" ;;
esac

note "verifying proxy $proxy_type://$proxy_host:$proxy_port from broker host $REMOTE"
egress_ip="$(
    ssh "$REMOTE" env \
        PROXY_SCHEME="$proxy_scheme" \
        PROXY_HOST="$proxy_host" \
        PROXY_PORT="$proxy_port" \
        PROXY_USERNAME="$proxy_username" \
        PROXY_PASSWORD="$proxy_password" \
        URL="$url" \
        EXPECT_IP="$expect_ip" \
        TIMEOUT="$timeout" \
        bash -s <<'EOF'
set -euo pipefail
proxy_url="${PROXY_SCHEME}://${PROXY_HOST}:${PROXY_PORT}"
curl_args=(-fsS --max-time "$TIMEOUT" --proxy "$proxy_url")
if [[ -n "$PROXY_USERNAME" ]]; then
    curl_args+=(--proxy-user "$PROXY_USERNAME:$PROXY_PASSWORD")
fi
result="$(curl "${curl_args[@]}" "$URL")"
if [[ -n "$EXPECT_IP" && "$result" != "$EXPECT_IP" ]]; then
    printf 'expected %s, got %s\n' "$EXPECT_IP" "$result" >&2
    exit 1
fi
printf '%s\n' "$result"
EOF
)"

printf '%s\n' "$egress_ip"
