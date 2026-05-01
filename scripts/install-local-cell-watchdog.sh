#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
    echo "usage: $0 <ssh-target>" >&2
    exit 1
fi

remote="$1"
repo_root="$(cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cd "$repo_root"

arch="$(ssh "$remote" 'uname -m')"
case "$arch" in
    x86_64|amd64) goarch="amd64" ;;
    aarch64|arm64) goarch="arm64" ;;
    *)
        echo "unsupported remote arch: $arch" >&2
        exit 1
        ;;
esac

binary="$tmpdir/local-cell-watchdog"
GOOS=linux GOARCH="$goarch" CGO_ENABLED=0 \
    go build -o "$binary" ./cmd/local-cell-watchdog

scp "$binary" \
    "$repo_root/ops/systemd/local-cell-watchdog.service" \
    "$repo_root/ops/systemd/local-cell-watchdog.timer" \
    "$remote:/tmp/"

ssh "$remote" '
set -euo pipefail
install -m 0755 /tmp/local-cell-watchdog /usr/local/bin/local-cell-watchdog
install -m 0644 /tmp/local-cell-watchdog.service /etc/systemd/system/local-cell-watchdog.service
install -m 0644 /tmp/local-cell-watchdog.timer /etc/systemd/system/local-cell-watchdog.timer
systemctl daemon-reload
systemctl enable --now local-cell-watchdog.timer
systemctl start local-cell-watchdog.service
systemctl status local-cell-watchdog.service --no-pager -l --lines=20 || true
systemctl status local-cell-watchdog.timer --no-pager -l --lines=20 || true
'
