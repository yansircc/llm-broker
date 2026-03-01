---
match: store\.(SaveAccount|SetAccount)
action: inject
match_on: new
paths: internal/
---

# Pool is the sole write entry for account state

## Symptom

账号状态不一致：多个包各自调 `store.SaveAccount` 修改账号，互相覆盖。无法推理状态转换，状态机散落在 4+ 个文件中。

## Root Cause

直接调用 `store.SaveAccount` / `store.SetAccount` 绕过了 Pool 的状态序列化。Pool.mu 保护的 in-memory 状态和 SQLite 持久化状态出现分裂。

## Correct Approach

1. **所有账号状态写操作必须通过 Pool 的三个入口**：
   - `Pool.Observe(accountID, statusCode, headers)` — 处理上游响应后的状态更新
   - `Pool.Update(accountID, fn)` — admin API 修改账号属性
   - `Pool.StoreTokens(accountID, tokens)` — OAuth token 刷新后存储
2. Pool 内部通过 `persistLocked()` 在持有 `mu` 锁的情况下同步写 SQLite
3. 不要在 `relay`、`server`、`oauth` 等包中直接调用 store 的写方法
4. 读操作可以直接走 store（如 `store.GetAccount`），但写必须走 Pool
