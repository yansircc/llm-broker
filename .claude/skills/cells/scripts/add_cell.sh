#!/usr/bin/env bash
set -euo pipefail

CELL_SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$CELL_SCRIPT_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage:
  bash .claude/skills/cells/scripts/add_cell.sh local  [options...]
  bash .claude/skills/cells/scripts/add_cell.sh remote [options...]

Shared options:
  --id CELL_ID
  --name "Cell Name"
  --listen-port PORT
  --ipv6 IPV6
  --ipv6-prefixlen PREFIXLEN
  --label key=value          Repeatable
  --expect-ip IPV6           Optional verification target
  --leave-disabled           Do not activate after verification

Local mode:
  --iface eth0
  --listen-host 127.0.0.1
  --service-name NAME

Remote mode:
  --target root@HOST
  --proxy-host WG_IP
  --wg-bind-ip WG_IP
  --allow-from CIDR
  --iface eth0
  --service-name NAME
  --install-package
EOF
}

mode="${1:-}"
[[ -n "$mode" ]] || { usage; exit 1; }
shift || true

case "$mode" in
    local|remote) ;;
    --help|-h) usage; exit 0 ;;
    *) die "first argument must be local or remote" ;;
esac

cell_id=""
name=""
listen_port=""
ipv6=""
ipv6_prefixlen="128"
expect_ip=""
leave_disabled=0
labels=()
iface="eth0"
listen_host="127.0.0.1"
service_name=""
target=""
proxy_host=""
wg_bind_ip=""
allow_from="10.77.0.1/32"
install_package=0

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --id) cell_id="${2:-}"; shift 2 ;;
        --name) name="${2:-}"; shift 2 ;;
        --listen-port) listen_port="${2:-}"; shift 2 ;;
        --ipv6) ipv6="${2:-}"; shift 2 ;;
        --ipv6-prefixlen) ipv6_prefixlen="${2:-}"; shift 2 ;;
        --label) labels+=("${2:-}"); shift 2 ;;
        --expect-ip) expect_ip="${2:-}"; shift 2 ;;
        --leave-disabled) leave_disabled=1; shift 1 ;;
        --iface) iface="${2:-}"; shift 2 ;;
        --listen-host) listen_host="${2:-}"; shift 2 ;;
        --service-name) service_name="${2:-}"; shift 2 ;;
        --target) target="${2:-}"; shift 2 ;;
        --proxy-host) proxy_host="${2:-}"; shift 2 ;;
        --wg-bind-ip) wg_bind_ip="${2:-}"; shift 2 ;;
        --allow-from) allow_from="${2:-}"; shift 2 ;;
        --install-package) install_package=1; shift 1 ;;
        --help|-h) usage; exit 0 ;;
        *) die "unknown argument: $1" ;;
    esac
done

[[ -n "$cell_id" ]] || die "--id is required"
[[ -n "$name" ]] || die "--name is required"
[[ -n "$listen_port" ]] || die "--listen-port is required"
[[ -n "$ipv6" ]] || die "--ipv6 is required"
[[ "$ipv6_prefixlen" =~ ^[0-9]+$ ]] || die "--ipv6-prefixlen must be numeric"

for label in "${labels[@]}"; do
    [[ "$label" == *=* ]] || die "--label must use key=value format: $label"
done

case "$mode" in
    local)
        provision_args=(
            --id "$cell_id"
            --listen-port "$listen_port"
            --ipv6 "$ipv6"
            --ipv6-prefixlen "$ipv6_prefixlen"
            --iface "$iface"
            --listen-host "$listen_host"
        )
        if [[ -n "$service_name" ]]; then
            provision_args+=(--service-name "$service_name")
        fi
        "$CELL_SCRIPT_DIR/provision_local_danted.sh" "${provision_args[@]}"
        proxy_host="$listen_host"
        ;;
    remote)
        [[ -n "$target" ]] || die "--target is required for remote mode"
        [[ -n "$proxy_host" ]] || die "--proxy-host is required for remote mode"
        [[ -n "$wg_bind_ip" ]] || die "--wg-bind-ip is required for remote mode"
        provision_args=(
            --target "$target"
            --id "$cell_id"
            --listen-port "$listen_port"
            --wg-bind-ip "$wg_bind_ip"
            --allow-from "$allow_from"
            --ipv6 "$ipv6"
            --ipv6-prefixlen "$ipv6_prefixlen"
            --iface "$iface"
        )
        if [[ -n "$service_name" ]]; then
            provision_args+=(--service-name "$service_name")
        fi
        if [[ "$install_package" == "1" ]]; then
            provision_args+=(--install-package)
        fi
        "$CELL_SCRIPT_DIR/provision_remote_danted.sh" "${provision_args[@]}"
        ;;
esac

register_args=(
    --id "$cell_id"
    --name "$name"
    --status disabled
    --proxy-type socks5
    --proxy-host "$proxy_host"
    --proxy-port "$listen_port"
)
for label in "${labels[@]}"; do
    register_args+=(--label "$label")
done

"$CELL_SCRIPT_DIR/register_cell.sh" "${register_args[@]}"

verify_args=(--cell-id "$cell_id")
if [[ -n "$expect_ip" ]]; then
    verify_args+=(--expect-ip "$expect_ip")
fi
egress_ip="$("$CELL_SCRIPT_DIR/verify_cell.sh" "${verify_args[@]}")"
note "cell $cell_id egress ip: $egress_ip"

if [[ "$leave_disabled" == "1" ]]; then
    note "leaving cell $cell_id disabled"
    fetch_cell_json "$cell_id" | jq .
    exit 0
fi

register_args_active=("${register_args[@]}")
register_args_active[5]="active"
"$CELL_SCRIPT_DIR/register_cell.sh" "${register_args_active[@]}"
note "cell $cell_id is active"
fetch_cell_json "$cell_id" | jq .
