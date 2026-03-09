#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

snapshot_ref="${1:-latest}"

echo "==> restoring snapshot: $snapshot_ref"
restored_id="$(restore_snapshot "$snapshot_ref")"
echo "==> restore successful: $restored_id"
