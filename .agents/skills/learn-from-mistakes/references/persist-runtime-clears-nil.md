---
match: PersistRuntime|HydrateRuntime
action: inject
paths: internal/domain/
---

# PersistRuntime must clear JSON columns when runtime field is nil

## Symptom

删除的 Proxy 在重启后复活：admin 通过 `pool.Update` 设置 `acct.Proxy = nil`，PersistRuntime 跳过序列化（`if a.Proxy != nil` 不满足），ProxyJSON 保留旧值。下次 HydrateRuntime 从 ProxyJSON 反序列化出已删除的 Proxy。

## Root Cause

PersistRuntime 的 `if a.Proxy != nil` 守卫导致 nil → JSON 的清除路径缺失：
```go
// BUG: 当 Proxy 被设为 nil 时，ProxyJSON 不会被清空
func (a *Account) PersistRuntime() {
    if a.Proxy != nil {
        data, _ := json.Marshal(a.Proxy)
        a.ProxyJSON = string(data)
    }
    // 缺少 else { a.ProxyJSON = "" }
}
```

同样的问题影响 `ExtInfo` / `ExtInfoJSON`。

## Correct Approach

1. **PersistRuntime 必须处理 nil 情况**，将对应 JSON 列清空：
   ```go
   func (a *Account) PersistRuntime() {
       if a.Proxy != nil {
           data, _ := json.Marshal(a.Proxy)
           a.ProxyJSON = string(data)
       } else {
           a.ProxyJSON = ""
       }
       if a.ExtInfo != nil {
           data, _ := json.Marshal(a.ExtInfo)
           a.ExtInfoJSON = string(data)
       } else {
           a.ExtInfoJSON = ""
       }
   }
   ```
2. 新增任何 runtime-only 字段（有 JSON 列对应）时，PersistRuntime 和 HydrateRuntime 必须同步更新
