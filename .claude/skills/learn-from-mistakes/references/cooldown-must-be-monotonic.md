---
match: OverloadedUntil[[:space:]]*=
action: inject
match_on: new
paths: internal/
---

# Cooldown must be monotonic — never shorten

## Symptom

账号冷却期被缩短：一个 429 设置了 60s cooldown，紧接着一个 200 把 OverloadedUntil 清零，账号立即可用，触发连续 429。

## Root Cause

直接赋值 `account.OverloadedUntil = newTime` 没有比较已有值。如果新值比已有值小，冷却期被缩短。

## Correct Approach

1. **永远通过 `applyCooldown` 设置 OverloadedUntil**，它实现 `max(existing, proposed)` 语义：
   ```go
   func applyCooldown(acct *Account, until time.Time) {
       if until.After(acct.OverloadedUntil) {
           acct.OverloadedUntil = until
       }
   }
   ```
2. 不要直接赋值 `OverloadedUntil = ...`，即使你认为新值一定更大
3. 清除冷却期的唯一合法方式是 `RunCleanup` 中的过期检查，或 admin API 的显式重置
4. 这是 Invariant #2 — 在 code review 中零容忍直接赋值
