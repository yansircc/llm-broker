#!/usr/bin/env bash
set -euo pipefail

CELL_SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$CELL_SCRIPT_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage:
  bash .claude/skills/cells/scripts/provision_local_danted.sh \
    --id CELL_ID \
    --listen-port PORT \
    --ipv6 IPV6 \
    [--ipv6-prefixlen 128] \
    [--iface eth0] \
    [--listen-host 127.0.0.1] \
    [--service-name danted-...]
EOF
}

require_cmd ssh

cell_id=""
listen_port=""
ipv6=""
ipv6_prefixlen="128"
iface="eth0"
listen_host="127.0.0.1"
service_name=""

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --id) cell_id="${2:-}"; shift 2 ;;
        --listen-port) listen_port="${2:-}"; shift 2 ;;
        --ipv6) ipv6="${2:-}"; shift 2 ;;
        --ipv6-prefixlen) ipv6_prefixlen="${2:-}"; shift 2 ;;
        --iface) iface="${2:-}"; shift 2 ;;
        --listen-host) listen_host="${2:-}"; shift 2 ;;
        --service-name) service_name="${2:-}"; shift 2 ;;
        --help|-h) usage; exit 0 ;;
        *) die "unknown argument: $1" ;;
    esac
done

[[ -n "$cell_id" ]] || die "--id is required"
[[ -n "$listen_port" ]] || die "--listen-port is required"
[[ -n "$ipv6" ]] || die "--ipv6 is required"
[[ "$listen_port" =~ ^[0-9]+$ ]] || die "--listen-port must be numeric"
[[ "$ipv6_prefixlen" =~ ^[0-9]+$ ]] || die "--ipv6-prefixlen must be numeric"

if [[ -z "$service_name" ]]; then
    service_name="danted-$(slugify "$cell_id")"
fi

conf_path="/etc/${service_name}.conf"
service_path="/etc/systemd/system/${service_name}.service"

note "provisioning local Dante cell $cell_id on broker host $REMOTE"
ssh "$REMOTE" env \
    CELL_ID="$cell_id" \
    LISTEN_HOST="$listen_host" \
    LISTEN_PORT="$listen_port" \
    IPV6="$ipv6" \
    IPV6_PREFIXLEN="$ipv6_prefixlen" \
    IFACE="$iface" \
    SERVICE_NAME="$service_name" \
    CONF_PATH="$conf_path" \
    SERVICE_PATH="$service_path" \
    bash -s <<'EOF'
set -euo pipefail

if [[ -x /usr/sbin/danted ]]; then
    danted_bin="/usr/sbin/danted"
elif command -v danted >/dev/null 2>&1; then
    danted_bin="$(command -v danted)"
else
    echo "danted is not installed on broker host" >&2
    exit 1
fi

cat >"$CONF_PATH" <<CONF
logoutput: syslog
internal: $LISTEN_HOST port = $LISTEN_PORT
external: $IPV6
socksmethod: none
user.privileged: root
user.unprivileged: nobody

client pass {
  from: 127.0.0.1/32 to: 0.0.0.0/0
  log: error connect disconnect
}
client pass {
  from: ::1/128 to: ::/0
  log: error connect disconnect
}

socks pass {
  from: 127.0.0.1/32 to: ::/0
  protocol: tcp udp
  log: error connect disconnect
}
socks pass {
  from: ::1/128 to: ::/0
  protocol: tcp udp
  log: error connect disconnect
}
CONF

cat >"$SERVICE_PATH" <<UNIT
[Unit]
Description=Broker local SOCKS5 for $CELL_ID
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStartPre=/bin/sh -c 'ip -6 addr show dev $IFACE | grep -q "$IPV6" || ip -6 addr add $IPV6/$IPV6_PREFIXLEN dev $IFACE nodad'
ExecStart=$danted_bin -f $CONF_PATH
ExecStopPost=/bin/sh -c 'ip -6 addr show dev $IFACE | grep -q "$IPV6" && ip -6 addr del $IPV6/$IPV6_PREFIXLEN dev $IFACE || true'
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now "$SERVICE_NAME"
for _ in $(seq 1 10); do
    systemctl is-active --quiet "$SERVICE_NAME" || true
    if ss -ltn | grep -q "$LISTEN_HOST:$LISTEN_PORT"; then
        printf '%s\n' "$SERVICE_NAME"
        exit 0
    fi
    sleep 1
done
systemctl status "$SERVICE_NAME" --no-pager >&2 || true
exit 1
printf '%s\n' "$SERVICE_NAME"
EOF
