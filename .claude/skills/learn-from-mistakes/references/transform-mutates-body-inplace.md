---
match: \.Transform\(|Transform\(body
action: inject
match_on: new
paths: internal/(relay|identity)/
---

# Transform mutates body in-place — must re-parse on retry

## Symptom

重试请求时 identity 泄露：第一次 attempt 的 Transform 把 user_id 改成了 account-A 的 hash，第二次 attempt 换了 account-B 但 body map 里还是 A 的 user_id。或者 system prompt 中的 billing headers 已被第一次 Transform 删除，第二次 attempt 的 Transform 跳过了（因为已经没有匹配的 header 了）。

## Root Cause

`identity.Transform()` **原地修改** body map：
- 删除 system prompt 中的 billing headers（line 49）
- 重写 `metadata.user_id`（line 58）
- 删除 `cache_control` 条目（lines 118-119, 136）
- 删除 TTL 字段

Go 的 `json.Unmarshal` 到 `map[string]interface{}` 产生的是引用类型，Transform 修改的就是这个引用。

## Correct Approach

1. **每次 retry attempt 必须从 `rawBody` 重新 Unmarshal 出新的 body map**：
   ```go
   // relay.go 现有正确模式
   var attemptBody map[string]interface{}
   json.Unmarshal(rawBody, &attemptBody) // 每次 attempt 全新解析
   transformer.Transform(attemptBody, ...)
   ```
2. 不要缓存 TransformResult 跨 attempt 复用
3. 不要对同一个 body map 调用两次 Transform
