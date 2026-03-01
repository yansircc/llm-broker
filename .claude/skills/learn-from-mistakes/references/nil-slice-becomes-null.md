---
match: writeJSON\(|json\.Marshal|json\.NewEncoder
action: inject
match_on: new
paths: internal/server/
---

# nil slice serializes to null in JSON

## Symptom

前端收到 `"sessions": null` 而非 `"sessions": []`，JavaScript 代码 `data.sessions.map(...)` 抛 TypeError crash。

## Root Cause

Go 中 `var s []T`（nil slice）JSON 序列化为 `null`，而 `s := make([]T, 0)`（empty slice）序列化为 `[]`。在构造 DTO 或响应体时，未初始化的 slice 字段会产生 null。

## Correct Approach

1. **所有 slice 字段在序列化前必须 nil guard**：
   ```go
   if dto.Sessions == nil {
       dto.Sessions = []SessionDTO{}
   }
   ```
2. 或者在 DTO 构造时直接用 `make`：
   ```go
   Sessions: make([]SessionDTO, 0),
   ```
3. 配合 `no-map-interface-in-writeJSON` 使用 typed DTO — 这样 nil guard 集中在一处
4. 前端不应该 defensively check `null`，后端保证 `[]` 是契约
