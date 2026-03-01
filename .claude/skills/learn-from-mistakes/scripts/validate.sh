#!/usr/bin/env bash
# PostToolUse hook — validate mistake file format after write.
#
# Only fires on Write/Edit to .../learn-from-mistakes/references/*.md
# Outputs additionalContext with errors if format is invalid.
set -euo pipefail

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')

# Fast path: only care about mistake files in references/
case "$FILE_PATH" in
  */learn-from-mistakes/references/*.md) ;;
  *) exit 0 ;;
esac

extract_frontmatter_field() {
  local file="$1"
  local key="$2"
  awk -v key="$key" '
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

ERRORS=""

match=$(extract_frontmatter_field "$FILE_PATH" "match")
action=$(extract_frontmatter_field "$FILE_PATH" "action")
message=$(extract_frontmatter_field "$FILE_PATH" "message")
tools=$(extract_frontmatter_field "$FILE_PATH" "tools")
paths=$(extract_frontmatter_field "$FILE_PATH" "paths")
match_on=$(extract_frontmatter_field "$FILE_PATH" "match_on")

# 1. match field required
[ -z "$match" ] && ERRORS="${ERRORS}missing match field\n"

# 2. match regex must be valid
if [ -n "$match" ]; then
  echo "" | grep -E "$match" >/dev/null 2>&1 || {
    [ $? -eq 2 ] && ERRORS="${ERRORS}invalid match regex: $match\n"
  }
fi

# 3. action must be inject or block
case "$action" in
  inject|block) ;;
  "") ERRORS="${ERRORS}missing action field\n" ;;
  *) ERRORS="${ERRORS}action must be inject or block, got: $action\n" ;;
esac

# 4. block requires message
if [ "$action" = "block" ] && [ -z "$message" ]; then
  ERRORS="${ERRORS}action=block requires a message field\n"
fi

# 5. tools field format (optional)
if [ -n "$tools" ]; then
  case "$tools" in
    *[!A-Za-z,|\ ]* ) ERRORS="${ERRORS}tools contains unsupported chars: $tools (use Bash|Edit|Write|MultiEdit)\n" ;;
  esac
  for t in $(echo "$tools" | tr ',|' '  '); do
    case "$t" in
      Bash|Edit|Write|MultiEdit) ;;
      "") ;;
      *) ERRORS="${ERRORS}invalid tool in tools: $t\n" ;;
    esac
  done
fi

# 6. match_on enum (optional)
if [ -n "$match_on" ]; then
  case "$match_on" in
    old|new|both) ;;
    *) ERRORS="${ERRORS}match_on must be one of: old, new, both\n" ;;
  esac
fi

# 7. regex portability checks for grep -E
if [ -n "$match" ]; then
  if echo "$match" | grep -qE '\(\?([=!<]|:)' ; then
    ERRORS="${ERRORS}match uses unsupported lookaround/non-capturing syntax for grep -E: $match\n"
  fi
  if echo "$match" | grep -q '\\[dwsb]' ; then
    ERRORS="${ERRORS}match uses unsupported PCRE class (\\d/\\w/\\s/\\b) for grep -E: $match\n"
  fi
fi

# 8. match should target code content (WHAT), not file paths (WHERE)
if [ -n "$match" ] && echo "$match" | grep -qE '^\*\*/|\*\.(ts|tsx|js|jsx|md|sql|css)$|^src/|^lib/|^app/'; then
  ERRORS="${ERRORS}match looks like a file path glob — should match code content instead:\n"
  ERRORS="${ERRORS}  BAD:  match: src/**/*.ts          -> triggers on any ts file\n"
  ERRORS="${ERRORS}  GOOD: match: db\\.insert.*\\.values -> triggers on batch insert code\n"
fi

# 9. paths regex must be valid (optional)
if [ -n "$paths" ]; then
  echo "dummy/path" | grep -E "$paths" >/dev/null 2>&1 || {
    [ $? -eq 2 ] && ERRORS="${ERRORS}invalid paths regex: $paths\n"
  }
fi

# Output validation errors
if [ -n "$ERRORS" ]; then
  CTX=$(printf "MISTAKE format error (%s):\n%bFix and re-save." "$(basename "$FILE_PATH")" "$ERRORS")
  jq -n --arg ctx "$CTX" '{
    hookSpecificOutput: {
      hookEventName: "PostToolUse",
      additionalContext: $ctx
    }
  }'
fi

exit 0
