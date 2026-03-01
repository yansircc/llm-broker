---
match: rm[[:space:]]+-r[[:space:]]|rm[[:space:]]+[^-]|rm[[:space:]]*$
action: inject
tools: Bash
---
# rm command needs -f flag in sandboxed/automated contexts

## Symptom

`rm` command gets rejected or hangs waiting for confirmation in the sandbox environment.

## Root Cause

Without `-f` flag, `rm` prompts for confirmation on each file, which hangs in non-interactive contexts.

## Correct Approach

Always use `rm -f` for files, `rm -rf` for directories. Never use bare `rm` without the `-f` flag.
