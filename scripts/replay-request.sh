#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
用法:
  scripts/replay-request.sh --id <request_log_id> --target <broker_base_url> --api-key <key> [--db <db_path>] [--remote <ssh_host>] [--dry-run]

说明:
  - 默认从 request_log 读取原始 client path / headers / body artifact。
  - 如果指定 --remote，会先通过 ssh 到远端读 sqlite 和 body artifact，再在本机向 --target 发起回放请求。
  - client_headers_json 只保存了允许观测的一小部分头；认证头不会复用，会改用 --api-key 提供的 key。

示例:
  scripts/replay-request.sh --id 221642 --target http://127.0.0.1:3000 --api-key fx
  scripts/replay-request.sh --id 221642 --target https://cc.210k.cc --api-key fx --remote root@172.236.22.238

环境变量:
  DB_PATH   默认 /var/lib/llm-broker/llm-broker.db
  REMOTE    等价于 --remote
  API_KEY   等价于 --api-key
  TARGET    等价于 --target
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少命令: $1" >&2
    exit 1
  fi
}

fetch_row_json() {
  local sql="$1"
  if [[ -n "$REMOTE" ]]; then
    ssh "$REMOTE" "sqlite3 -readonly -json '$DB_PATH' \"$sql\""
    return
  fi
  sqlite3 -readonly -json "$DB_PATH" "$sql"
}

fetch_body_file() {
  local src="$1"
  local dst="$2"
  if [[ -n "$REMOTE" ]]; then
    ssh "$REMOTE" "cat '$src'" >"$dst"
    return
  fi
  cat "$src" >"$dst"
}

REQUEST_ID=""
DB_PATH="${DB_PATH:-/var/lib/llm-broker/llm-broker.db}"
REMOTE="${REMOTE:-}"
API_KEY="${API_KEY:-}"
TARGET="${TARGET:-}"
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --id)
      REQUEST_ID="${2:-}"
      shift 2
      ;;
    --db)
      DB_PATH="${2:-}"
      shift 2
      ;;
    --remote)
      REMOTE="${2:-}"
      shift 2
      ;;
    --api-key)
      API_KEY="${2:-}"
      shift 2
      ;;
    --target)
      TARGET="${2:-}"
      shift 2
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

if [[ -z "$REQUEST_ID" || -z "$TARGET" || -z "$API_KEY" ]]; then
  usage >&2
  exit 1
fi
if [[ ! "$REQUEST_ID" =~ ^[0-9]+$ ]]; then
  echo "--id 必须是数字 request_log.id" >&2
  exit 1
fi

require_cmd sqlite3
require_cmd jq
require_cmd curl
if [[ -n "$REMOTE" ]]; then
  require_cmd ssh
fi

SQL="SELECT path, COALESCE(client_headers_json, '{}') AS client_headers_json, COALESCE(json_extract(request_meta_json, '\$.body_artifact_path'), '') AS body_artifact_path, COALESCE(client_body_excerpt, '') AS client_body_excerpt FROM request_log WHERE id = ${REQUEST_ID} LIMIT 1;"
ROW_JSON="$(fetch_row_json "$SQL")"

if [[ -z "$ROW_JSON" || "$ROW_JSON" == "[]" ]]; then
  echo "没有找到 request_log.id=$REQUEST_ID" >&2
  exit 1
fi

PATH_PART="$(printf '%s' "$ROW_JSON" | jq -r '.[0].path // empty')"
HEADERS_JSON="$(printf '%s' "$ROW_JSON" | jq -c '.[0].client_headers_json | if type == "string" then (fromjson? // {}) else . end')"
ARTIFACT_PATH="$(printf '%s' "$ROW_JSON" | jq -r '.[0].body_artifact_path // empty')"
BODY_EXCERPT="$(printf '%s' "$ROW_JSON" | jq -r '.[0].client_body_excerpt // empty')"

if [[ -z "$PATH_PART" ]]; then
  echo "request_log.id=$REQUEST_ID 缺少 path" >&2
  exit 1
fi

BODY_FILE="$(mktemp)"
cleanup() {
  rm -f "$BODY_FILE"
}
trap cleanup EXIT

if [[ -n "$ARTIFACT_PATH" ]]; then
  fetch_body_file "$ARTIFACT_PATH" "$BODY_FILE"
else
  printf '%s' "$BODY_EXCERPT" >"$BODY_FILE"
fi

if [[ ! -s "$BODY_FILE" ]]; then
  echo "request_log.id=$REQUEST_ID 没有可用的 body 内容" >&2
  exit 1
fi

URL="${TARGET%/}${PATH_PART}"
declare -a CURL_ARGS
CURL_ARGS=(-sS -i -X POST "$URL" -H "x-api-key: $API_KEY" --data-binary "@$BODY_FILE")

HAS_CONTENT_TYPE=0
while IFS=$'\t' read -r key value; do
  [[ -z "$key" ]] && continue
  lower_key="$(printf '%s' "$key" | tr '[:upper:]' '[:lower:]')"
  case "$lower_key" in
    authorization|x-api-key)
      continue
      ;;
    content-type)
      HAS_CONTENT_TYPE=1
      ;;
  esac
  CURL_ARGS+=(-H "$key: $value")
done < <(printf '%s' "$HEADERS_JSON" | jq -r 'to_entries[] | [.key, (.value | tostring)] | @tsv')

if [[ "$HAS_CONTENT_TYPE" -eq 0 ]]; then
  CURL_ARGS+=(-H "Content-Type: application/json")
fi

echo "replay request_log.id=$REQUEST_ID" >&2
echo "  target: $URL" >&2
echo "  remote: ${REMOTE:-<local>}" >&2
echo "  body: ${ARTIFACT_PATH:-<excerpt>}" >&2

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "dry-run: 未实际发送请求" >&2
  exit 0
fi

curl "${CURL_ARGS[@]}"
