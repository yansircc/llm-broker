---
match: pool\.Get\(|pool\.List\(|pool\.Pick\(
action: inject
match_on: new
paths: internal/
---

# Pool.Get/List/Pick returns shallow copy — treat as read-only

## Symptom

账号状态被意外修改：handler 通过 `pool.Get(id)` 获取账号，修改了 `acct.ExtInfo["key"] = value` 或 `*acct.LastUsedAt = time.Now()`，绕过 Pool 锁直接污染了 in-memory 状态。

## Root Cause

`Pool.Get()`、`List()`、`Pick()` 返回 `copy := *acct; return &copy` — 这是**浅拷贝**。结构体本身是新的，但指针和 map 字段（`Proxy *ProxyConfig`、`ExtInfo map[string]interface{}`、`LastUsedAt *time.Time`、`OverloadedUntil *time.Time`）仍与 Pool 内部的 canonical account 共享底层内存。

通过浅拷贝的指针/map 字段写入 = 绕过 `Pool.mu` 锁 + 绕过 `persistLocked` = 违反 Invariant #1。

## Correct Approach

1. **永远不要修改 Pool.Get/List/Pick 返回值的任何字段** — 视为只读快照
2. 需要修改账号状态 → 使用 `pool.Update(id, func(a *Account) { ... })`
3. 需要修改 tokens → 使用 `pool.StoreTokens(id, ...)`
4. `GetProxy()` 也返回内部指针的直接引用 — 不可修改
