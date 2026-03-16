#!/usr/bin/env bash
set -euo pipefail

CELL_SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$CELL_SCRIPT_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage:
  bash .claude/skills/cells/scripts/register_cell.sh \
    --id CELL_ID \
    --name "Cell Name" \
    --status active|disabled|error \
    --proxy-type socks5|http|https \
    --proxy-host HOST \
    --proxy-port PORT \
    [--proxy-username USERNAME] \
    [--proxy-password PASSWORD] \
    [--label key=value ...]
EOF
}

require_cmd curl
require_cmd jq
require_cmd ssh

cell_id=""
name=""
status=""
proxy_type="socks5"
proxy_host=""
proxy_port=""
proxy_username=""
proxy_password=""
labels=()

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --id) cell_id="${2:-}"; shift 2 ;;
        --name) name="${2:-}"; shift 2 ;;
        --status) status="${2:-}"; shift 2 ;;
        --proxy-type) proxy_type="${2:-}"; shift 2 ;;
        --proxy-host) proxy_host="${2:-}"; shift 2 ;;
        --proxy-port) proxy_port="${2:-}"; shift 2 ;;
        --proxy-username) proxy_username="${2:-}"; shift 2 ;;
        --proxy-password) proxy_password="${2:-}"; shift 2 ;;
        --label) labels+=("${2:-}"); shift 2 ;;
        --help|-h) usage; exit 0 ;;
        *) die "unknown argument: $1" ;;
    esac
done

[[ -n "$cell_id" ]] || die "--id is required"
[[ -n "$name" ]] || die "--name is required"
[[ -n "$status" ]] || die "--status is required"
[[ -n "$proxy_host" ]] || die "--proxy-host is required"
[[ -n "$proxy_port" ]] || die "--proxy-port is required"
[[ "$proxy_port" =~ ^[0-9]+$ ]] || die "--proxy-port must be numeric"
[[ -z "$proxy_password" || -n "$proxy_username" ]] || die "--proxy-password requires --proxy-username"

case "$status" in
    active|disabled|error) ;;
    *) die "--status must be active, disabled, or error" ;;
esac

case "$proxy_type" in
    socks5|http|https) ;;
    *) die "--proxy-type must be socks5, http, or https" ;;
esac

for label in "${labels[@]}"; do
    [[ "$label" == *=* ]] || die "--label must use key=value format: $label"
done

labels_payload="$(labels_json "${labels[@]}")"
payload="$(jq -n \
    --arg id "$cell_id" \
    --arg name "$name" \
    --arg status "$status" \
    --arg proxy_type "$proxy_type" \
    --arg proxy_host "$proxy_host" \
    --argjson proxy_port "$proxy_port" \
    --arg proxy_username "$proxy_username" \
    --arg proxy_password "$proxy_password" \
    --argjson labels "$labels_payload" '
    {
      id: $id,
      name: $name,
      status: $status,
      proxy: {
        type: $proxy_type,
        host: $proxy_host,
        port: $proxy_port
      },
      labels: $labels
    }
    | if $proxy_username != "" then .proxy.username = $proxy_username else . end
    | if $proxy_password != "" then .proxy.password = $proxy_password else . end
')"

note "registering cell $cell_id on $SITE"
admin_api POST "/admin/egress/cells" "$payload" | jq .
