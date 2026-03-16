#!/usr/bin/env bash
# Reactive guardrails — reads mistake files, matches against tool_input content.
#
# PreToolUse hook. Input: JSON via stdin.
#   Exit 0 = allow (stdout JSON with additionalContext = warning)
#   Exit 2 = block (stderr shown to agent as error)
#
# Regex engine: grep -qE (POSIX ERE)
# Pattern constraints for references/*.md match fields:
#   - No lookahead/lookbehind: (?!...), (?<=...) are NOT supported
#   - No \d, \w, \b — use [0-9], [a-zA-Z_], word boundaries via context
#   - Literal braces: use [{] [}] or \\{ \\}
#   - Quotes in patterns: use character class ["'] to avoid YAML frontmatter issues
set -euo pipefail

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name')

REFS_DIR="$(cd "$(dirname "$0")" && pwd)/../references"

# ─── Fast path ────────────────────────────────────────────────
[ -d "$REFS_DIR" ] || exit 0
shopt -s nullglob
FILES=("$REFS_DIR"/*.md)
shopt -u nullglob
[ ${#FILES[@]} -gt 0 ] || exit 0

# ─── Only process relevant tools ─────────────────────────────
case "$TOOL_NAME" in
  Edit|Write|MultiEdit|Bash) ;;
  *) exit 0 ;;
esac

# ─── Build content strings for matching ───────────────────────
FILE_PATH=""
NEW_CONTENT=""
OLD_CONTENT=""
BASH_CONTENT=""

case "$TOOL_NAME" in
  Bash)
    BASH_CONTENT=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
    ;;
  Edit)
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')
    OLD_CONTENT=$(echo "$INPUT" | jq -r '.tool_input.old_string // ""')
    NEW_CONTENT=$(echo "$INPUT" | jq -r '.tool_input.new_string // ""')
    ;;
  Write)
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')
    NEW_CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // ""')
    ;;
  MultiEdit)
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')
    OLD_CONTENT=$(echo "$INPUT" | jq -r '[.tool_input.edits[]? | .old_string] | map(select(. != null)) | join("\n")')
    NEW_CONTENT=$(echo "$INPUT" | jq -r '[.tool_input.edits[]? | .new_string] | map(select(. != null)) | join("\n")')
    ;;
esac

# ─── Helpers ──────────────────────────────────────────────────
extract_frontmatter_field() {
  local file="$1"
  local key="$2"
  awk -v key="$key" '
    BEGIN { in_fm = 0; fm_count = 0 }
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm && $0 == "---" { exit }
    in_fm {
      if ($0 ~ "^" key ":[[:space:]]*") {
        sub("^" key ":[[:space:]]*", "", $0)
        print $0
        exit
      }
    }
  ' "$file"
}

extract_title() {
  local file="$1"
  awk '
    /^# / {
      sub(/^# /, "", $0)
      print $0
      exit
    }
  ' "$file"
}

match_in_target() {
  local pattern="$1"
  local target="$2"
  [ -z "$target" ] && return 1
  echo "$target" | grep -qE "$pattern"
}

# ─── Match against mistake files ─────────────────────────────
WARNINGS=""
BLOCK_MSG=""

for ref_file in "${FILES[@]}"; do
  match=$(extract_frontmatter_field "$ref_file" "match")
  action=$(extract_frontmatter_field "$ref_file" "action")
  tools=$(extract_frontmatter_field "$ref_file" "tools")
  paths=$(extract_frontmatter_field "$ref_file" "paths")
  match_on=$(extract_frontmatter_field "$ref_file" "match_on")

  [ -z "$match" ] && continue
  [ -z "$action" ] && action="inject"
  [ -z "$match_on" ] && match_on="both"

  # Optional tool filtering per rule
  if [ -n "$tools" ]; then
    tools_normalized=$(echo "$tools" | tr ',' '|' | tr -d ' ')
    case "|$tools_normalized|" in
      *"|$TOOL_NAME|"*) ;;
      *) continue ;;
    esac
  fi

  # Optional path filtering for file-edit tools
  if [ -n "$paths" ] && [ "$TOOL_NAME" != "Bash" ]; then
    if ! echo "$FILE_PATH" | grep -qE "$paths"; then
      continue
    fi
  fi

  hit=1
  case "$TOOL_NAME" in
    Bash)
      match_in_target "$match" "$BASH_CONTENT" || hit=0
      ;;
    Write)
      match_in_target "$match" "$NEW_CONTENT" || hit=0
      ;;
    Edit|MultiEdit)
      case "$match_on" in
        old) match_in_target "$match" "$OLD_CONTENT" || hit=0 ;;
        new) match_in_target "$match" "$NEW_CONTENT" || hit=0 ;;
        both|*)
          if ! match_in_target "$match" "$OLD_CONTENT" && ! match_in_target "$match" "$NEW_CONTENT"; then
            hit=0
          fi
          ;;
      esac
      ;;
  esac

  [ "$hit" -eq 1 ] || continue

  filename=$(basename "$ref_file")
  title=$(extract_title "$ref_file")

  if [ "$action" = "block" ]; then
    message=$(extract_frontmatter_field "$ref_file" "message")
    if [ -n "$title" ]; then
      BLOCK_MSG="${BLOCK_MSG}${message:-See $filename} (${title})\n"
    else
      BLOCK_MSG="${BLOCK_MSG}${message:-See $filename}\n"
    fi
  else
    if [ -n "$title" ]; then
      WARNINGS="${WARNINGS}⚠️ 读 @.claude/skills/learn-from-mistakes/references/${filename} — ${title}\n"
    else
      WARNINGS="${WARNINGS}⚠️ 读 @.claude/skills/learn-from-mistakes/references/${filename}\n"
    fi
  fi
done

# ─── Block takes priority ────────────────────────────────────
if [ -n "$BLOCK_MSG" ]; then
  printf "BLOCKED:\n%b" "$BLOCK_MSG" >&2
  exit 2
fi

# ─── Inject context warnings ─────────────────────────────────
if [ -n "$WARNINGS" ]; then
  CTX=$(printf "%b" "$WARNINGS")
  jq -n --arg ctx "$CTX" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      additionalContext: $ctx
    }
  }'
  exit 0
fi

exit 0
