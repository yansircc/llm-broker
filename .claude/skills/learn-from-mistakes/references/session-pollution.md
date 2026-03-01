---
match: \.Pick\(
action: inject
match_on: new
paths: internal/
---

# Session pollution — never switch accounts mid-session

## Symptom

Claude 检测到同一 session 中 user fingerprint 变化（换号 = 不同的 stainless binding），触发封号调查。

## Root Cause

当 bound account 不可用时，Pick 静默选择了另一个账号继续服务。对 Claude 来说，同一个 conversation 中途换人了。

## Correct Approach

1. **bound account 不可用 + isOldSession → 返回 400 拒绝请求**，不能静默切号
2. `isOldSession` 判定条件（任一为真即为 old session）：
   - `messages > 1`
   - 存在 multi-content-block message
   - 不包含 tools definition（非首次请求）
3. 只有 new session（首次请求）才允许分配新账号
4. 前端/客户端收到 400 后应开始新 conversation，而非重试
5. 这是防封的核心机制 — Invariant #4
