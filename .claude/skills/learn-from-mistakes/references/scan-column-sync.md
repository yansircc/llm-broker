---
match: accountCols|scanAccount
action: inject
paths: internal/store/sqlite_accounts
---

# accountCols, scanAccount, SaveAccount — 三个列表必须同步

## Symptom

添加新字段后运行时 panic 或静默数据错位：`scanAccount` 的 `Scan(...)` 参数个数与 `accountCols` 列数不匹配 → SQLite 返回 "expected N destination arguments in Scan, not M"。或者参数个数对但位置错 → 字段值赋错位，数据静默损坏。

## Root Cause

`sqlite_accounts.go` 中有 **三个必须保持同步的并行列表**：
1. `accountCols`（line 11）— SELECT 的列名列表，决定查询顺序
2. `scanAccount()`（line 18）— `scanner.Scan(...)` 的参数列表，按位置匹配列
3. `SaveAccount()`（line 113）— INSERT 的列/值列表 + ON CONFLICT SET

这三个列表各有 29 个条目（当前），没有编译期检查它们是否一致。

## Correct Approach

1. **添加新列时，三个地方必须同时更新，且位置一致**：
   - `accountCols` 追加列名
   - `scanAccount` 追加对应变量和 Scan 参数
   - `SaveAccount` 追加 INSERT 列名、值占位、ON CONFLICT SET
2. **检查数量一致**：修改后手动数一下三个列表的条目数
3. **同时更新**：`schema.sql`（新建表场景）和 `migrate()`（已有表场景）
4. 如果新字段有 runtime/JSON 对应关系，同步更新 `PersistRuntime` 和 `HydrateRuntime`
