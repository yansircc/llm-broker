#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

snapshot_id="$(create_snapshot "${1:-manual}")"

echo "==> snapshot created: $snapshot_id"
echo "==> restore with: bash $SCRIPT_DIR/restore.sh $snapshot_id"
