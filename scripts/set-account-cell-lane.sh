#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
用法:
  scripts/set-account-cell-lane.sh --remote <ssh_host> --account <email_or_id> --lane <unset|native|compat|all> [--allow-shared-cell] [--dry-run]

说明:
  - 实际修改的是“账号绑定 cell”的 labels.lane，不是账号字段。
  - lane=compat  => 仅接 compat 请求
  - lane=all     => 同时接 native / compat
  - lane=unset   => 删除 lane，回到默认 native-only
  - lane=native  => unset 的别名；脚本会直接删除 lane，而不是写回 native
  - 默认拒绝修改共享 cell；若 cell 绑定了多个账号，需显式传 --allow-shared-cell

示例:
  scripts/set-account-cell-lane.sh --remote ccc --account kun --lane compat
  scripts/set-account-cell-lane.sh --remote ccc --account kun --lane unset
  scripts/set-account-cell-lane.sh --remote ccc --account kun --lane all
  scripts/set-account-cell-lane.sh --remote ccc --account 2f7183ba-4398-446a-b5a7-b0b421bc3115 --lane unset
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少命令: $1" >&2
    exit 1
  fi
}

REMOTE=""
ACCOUNT=""
LANE=""
ALLOW_SHARED_CELL=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remote)
      REMOTE="${2:-}"
      shift 2
      ;;
    --account)
      ACCOUNT="${2:-}"
      shift 2
      ;;
    --lane)
      LANE="${2:-}"
      shift 2
      ;;
    --allow-shared-cell)
      ALLOW_SHARED_CELL=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$REMOTE" || -z "$ACCOUNT" || -z "$LANE" ]]; then
  usage >&2
  exit 1
fi

LANE="$(printf '%s' "$LANE" | tr '[:upper:]' '[:lower:]')"
case "$LANE" in
  unset)
    ;;
  native)
    LANE="unset"
    ;;
  compat|all)
    ;;
  *)
    echo "--lane 只能是 unset、native、compat、all" >&2
    exit 1
    ;;
esac

require_cmd ssh

ssh "$REMOTE" env TARGET_ACCOUNT="$ACCOUNT" TARGET_LANE="$LANE" ALLOW_SHARED_CELL="$ALLOW_SHARED_CELL" DRY_RUN="$DRY_RUN" bash -s <<'EOF'
set -euo pipefail

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "远端缺少命令: $1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd python3
require_cmd awk

token="$(awk -F= '$1=="API_TOKEN"{print substr($0,index($0,"=")+1); exit}' /etc/llm-broker.env)"
if [[ -z "$token" ]]; then
  echo "远端 /etc/llm-broker.env 缺少 API_TOKEN" >&2
  exit 1
fi

source /var/lib/llm-broker/bluegreen/layout.env
active_slot="$(tr -d '\n' < /var/lib/llm-broker/bluegreen/active-slot)"
case "$active_slot" in
  blue)
    port="$BLUE_PORT"
    ;;
  green)
    port="$GREEN_PORT"
    ;;
  *)
    echo "invalid active slot: $active_slot" >&2
    exit 1
    ;;
esac

accounts_json="$(mktemp)"
cells_json="$(mktemp)"
payload_json="$(mktemp)"
verify_json="$(mktemp)"

cleanup() {
  rm -f "$accounts_json" "$cells_json" "$payload_json" "$verify_json"
}
trap cleanup EXIT

curl -sS -H "x-api-key: $token" "http://127.0.0.1:${port}/admin/accounts" >"$accounts_json"
curl -sS -H "x-api-key: $token" "http://127.0.0.1:${port}/admin/egress/cells" >"$cells_json"

summary_json="$(
  python3 - "$accounts_json" "$cells_json" "$TARGET_ACCOUNT" "$TARGET_LANE" "$ALLOW_SHARED_CELL" "$payload_json" <<'PY'
import json
import sys

accounts_path, cells_path, target, target_lane, allow_shared_raw, payload_path = sys.argv[1:7]
allow_shared = allow_shared_raw == "1"

with open(accounts_path, "r", encoding="utf-8") as fh:
    accounts = json.load(fh)
with open(cells_path, "r", encoding="utf-8") as fh:
    cells = json.load(fh)

matches = [acct for acct in accounts if acct.get("email") == target or acct.get("id") == target]
if not matches:
    raise SystemExit(f"找不到账号: {target}")
if len(matches) > 1:
    raise SystemExit(f"账号匹配不唯一: {target}")

acct = matches[0]
cell_id = (acct.get("cell") or {}).get("id") or acct.get("cell_id") or ""
if not cell_id:
    raise SystemExit(f"账号 {acct.get('email') or acct.get('id')} 没有绑定 cell，不能切 lane")

cell = next((item for item in cells if item.get("id") == cell_id), None)
if cell is None:
    raise SystemExit(f"找不到 cell: {cell_id}")

cell_accounts = cell.get("accounts") or []
if not any(item.get("id") == acct.get("id") for item in cell_accounts):
    raise SystemExit(f"cell {cell_id} 的 accounts 列表里没有目标账号 {acct.get('id')}")
if len(cell_accounts) > 1 and not allow_shared:
    names = [item.get("email") or item.get("id") or "<unknown>" for item in cell_accounts]
    raise SystemExit(
        f"cell {cell_id} 当前绑定了多个账号: {', '.join(names)}；这是 cell 级切换，默认拒绝，请先拆分绑定或显式传 --allow-shared-cell"
    )

labels = dict(cell.get("labels") or {})
old_lane = labels.get("lane") or "<unset>"
if target_lane == "unset":
    labels.pop("lane", None)
    new_lane = "<unset>"
else:
    labels["lane"] = target_lane
    new_lane = target_lane

payload = {
    "id": cell["id"],
    "name": cell["name"],
    "status": cell["status"],
    "proxy": cell.get("proxy"),
    "labels": labels,
}

with open(payload_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, ensure_ascii=False, separators=(",", ":"))

summary = {
    "account_id": acct.get("id"),
    "account_email": acct.get("email"),
    "cell_id": cell_id,
    "old_lane": old_lane,
    "new_lane": new_lane,
    "active_native_before": acct.get("available_native"),
    "active_compat_before": acct.get("available_compat"),
    "cell_accounts": [
        {
            "id": item.get("id"),
            "email": item.get("email"),
            "provider": item.get("provider"),
        }
        for item in cell_accounts
    ],
}
print(json.dumps(summary, ensure_ascii=False))
PY
)"

python3 - "$summary_json" "$active_slot" "$port" "$DRY_RUN" <<'PY'
import json
import sys

summary = json.loads(sys.argv[1])
active_slot = sys.argv[2]
port = sys.argv[3]
dry_run = sys.argv[4] == "1"

print(f"active_slot={active_slot} port={port}")
print(f"account={summary['account_email']} ({summary['account_id']})")
print(f"cell={summary['cell_id']}")
print(f"lane: {summary['old_lane']} -> {summary['new_lane']}")
print(
    f"before: native={summary['active_native_before']} compat={summary['active_compat_before']}"
)
if len(summary["cell_accounts"]) > 1:
    joined = ", ".join(
        f"{item['email']}[{item['provider']}]"
        for item in summary["cell_accounts"]
    )
    print(f"shared_cell_accounts={joined}")
if dry_run:
    print("dry_run=true")
PY

if [[ "$DRY_RUN" == "1" ]]; then
  exit 0
fi

response_json="$(
  curl -sS -X POST \
    -H "x-api-key: $token" \
    -H "Content-Type: application/json" \
    --data @"$payload_json" \
    "http://127.0.0.1:${port}/admin/egress/cells"
)"

curl -sS -H "x-api-key: $token" "http://127.0.0.1:${port}/admin/accounts" >"$verify_json"

python3 - "$response_json" "$verify_json" "$TARGET_ACCOUNT" <<'PY'
import json
import sys

response = json.loads(sys.argv[1])
with open(sys.argv[2], "r", encoding="utf-8") as fh:
    accounts = json.load(fh)
target = sys.argv[3]

acct = next((item for item in accounts if item.get("email") == target or item.get("id") == target), None)
if acct is None:
    raise SystemExit(f"写入后找不到目标账号: {target}")

lane = (response.get("labels") or {}).get("lane") or "<unset>"
print(f"applied_lane={lane}")
print(
    "after: "
    f"native={acct.get('available_native')} "
    f"compat={acct.get('available_compat')} "
    f"cell_id={acct.get('cell_id') or '<none>'}"
)
PY
EOF
