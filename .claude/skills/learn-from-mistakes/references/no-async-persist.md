---
match: go[[:space:]]+(s\.store|p\.store|store)\.[a-zA-Z]+
action: inject
match_on: new
paths: internal/
---

# No goroutine async persist

## Symptom

账号状态丢失或回滚：goroutine A 读取账号快照 → goroutine B 修改并持久化 → goroutine A 持久化旧快照覆盖 B 的写入。

## Root Cause

用 `go store.SaveAccount(...)` 异步持久化，goroutine 拿到的是快照时刻的数据。在 3-7 个账号的场景下，多个 goroutine 并发写同一账号的概率不低。

## Correct Approach

1. **所有 store 写操作在 Pool.mu 锁内同步执行**（`persistLocked()`）
2. 永远不要 `go store.SaveAccount(...)` 或 `go store.UpdateAccount(...)`
3. SQLite UPSERT 对 3-7 行数据 < 1ms，同步写完全可接受
4. 如果需要异步操作（如 token 刷新），先异步获取结果，再在锁内同步写入：
   ```go
   // OK: async fetch, sync persist
   go func() {
       token := refreshToken(...)   // async
       pool.StoreTokens(id, token)  // sync (internally locks + persist)
   }()
   ```
