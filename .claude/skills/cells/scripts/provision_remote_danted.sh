#!/usr/bin/env bash
set -euo pipefail

CELL_SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$CELL_SCRIPT_DIR/common.sh"

usage() {
    cat <<'EOF'
Usage:
  bash .claude/skills/cells/scripts/provision_remote_danted.sh \
    --target root@HOST \
    --id CELL_ID \
    --listen-port PORT \
    --wg-bind-ip WG_IP \
    --allow-from CIDR \
    --ipv6 IPV6 \
    [--iface eth0] \
    [--service-name danted-...] \
    [--install-package]
EOF
}

require_cmd ssh

target=""
cell_id=""
listen_port=""
wg_bind_ip=""
allow_from="10.77.0.1/32"
ipv6=""
iface="eth0"
service_name=""
install_package=0

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --target) target="${2:-}"; shift 2 ;;
        --id) cell_id="${2:-}"; shift 2 ;;
        --listen-port) listen_port="${2:-}"; shift 2 ;;
        --wg-bind-ip) wg_bind_ip="${2:-}"; shift 2 ;;
        --allow-from) allow_from="${2:-}"; shift 2 ;;
        --ipv6) ipv6="${2:-}"; shift 2 ;;
        --iface) iface="${2:-}"; shift 2 ;;
        --service-name) service_name="${2:-}"; shift 2 ;;
        --install-package) install_package=1; shift 1 ;;
        --help|-h) usage; exit 0 ;;
        *) die "unknown argument: $1" ;;
    esac
done

[[ -n "$target" ]] || die "--target is required"
[[ -n "$cell_id" ]] || die "--id is required"
[[ -n "$listen_port" ]] || die "--listen-port is required"
[[ -n "$wg_bind_ip" ]] || die "--wg-bind-ip is required"
[[ -n "$ipv6" ]] || die "--ipv6 is required"
[[ "$listen_port" =~ ^[0-9]+$ ]] || die "--listen-port must be numeric"

if [[ -z "$service_name" ]]; then
    service_name="danted-$(slugify "$cell_id")"
fi

conf_path="/etc/${service_name}.conf"
service_path="/etc/systemd/system/${service_name}.service"

note "provisioning remote Dante cell $cell_id on $target"
ssh "$target" env \
    CELL_ID="$cell_id" \
    LISTEN_PORT="$listen_port" \
    WG_BIND_IP="$wg_bind_ip" \
    ALLOW_FROM="$allow_from" \
    IPV6="$ipv6" \
    IFACE="$iface" \
    SERVICE_NAME="$service_name" \
    CONF_PATH="$conf_path" \
    SERVICE_PATH="$service_path" \
    INSTALL_PACKAGE="$install_package" \
    bash -s <<'EOF'
set -euo pipefail

install_dante() {
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y dante-server
}

if [[ -x /usr/sbin/danted ]]; then
    danted_bin="/usr/sbin/danted"
elif command -v danted >/dev/null 2>&1; then
    danted_bin="$(command -v danted)"
elif [[ "$INSTALL_PACKAGE" == "1" ]]; then
    install_dante
    if [[ -x /usr/sbin/danted ]]; then
        danted_bin="/usr/sbin/danted"
    elif command -v danted >/dev/null 2>&1; then
        danted_bin="$(command -v danted)"
    else
        echo "danted install succeeded but binary still missing" >&2
        exit 1
    fi
else
    echo "danted is not installed on remote host; rerun with --install-package" >&2
    exit 1
fi

ip addr show | grep -q " $WG_BIND_IP/" || {
    echo "wg bind ip not present: $WG_BIND_IP" >&2
    exit 1
}

cat >"$CONF_PATH" <<CONF
logoutput: syslog
internal: $WG_BIND_IP port = $LISTEN_PORT
external: $IPV6
socksmethod: none
user.privileged: root
user.unprivileged: nobody

client pass {
  from: $ALLOW_FROM to: 0.0.0.0/0
  log: error connect disconnect
}

socks pass {
  from: $ALLOW_FROM to: ::/0
  protocol: tcp udp
  log: error connect disconnect
}
CONF

cat >"$SERVICE_PATH" <<UNIT
[Unit]
Description=WG-backed SOCKS5 for $CELL_ID
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStartPre=/bin/sh -c 'ip -6 addr show dev $IFACE | grep -q "$IPV6" || ip -6 addr add $IPV6/64 dev $IFACE nodad'
ExecStart=$danted_bin -f $CONF_PATH
ExecStopPost=/bin/sh -c 'ip -6 addr show dev $IFACE | grep -q "$IPV6" && ip -6 addr del $IPV6/64 dev $IFACE || true'
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

if command -v ufw >/dev/null 2>&1; then
    ufw_status="$(ufw status | sed -n '1s/^Status: //p' || true)"
    if [[ "$ufw_status" == "active" ]]; then
        if ! ufw status numbered | grep -Fq "$WG_BIND_IP $LISTEN_PORT/tcp on $IFACE"; then
            ufw allow in on "$IFACE" from "$ALLOW_FROM" to "$WG_BIND_IP" port "$LISTEN_PORT" proto tcp comment "$CELL_ID socks"
        fi
    fi
fi

systemctl daemon-reload
systemctl enable --now "$SERVICE_NAME"
for _ in $(seq 1 10); do
    systemctl is-active --quiet "$SERVICE_NAME" || true
    if ss -ltn | grep -q "$WG_BIND_IP:$LISTEN_PORT"; then
        printf '%s\n' "$SERVICE_NAME"
        exit 0
    fi
    sleep 1
done
systemctl status "$SERVICE_NAME" --no-pager >&2 || true
exit 1
printf '%s\n' "$SERVICE_NAME"
EOF
