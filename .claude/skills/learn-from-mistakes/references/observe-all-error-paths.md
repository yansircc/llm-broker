---
match: ObserveSuccess\(
action: inject
match_on: new
paths: internal/server/
---

# All upstream error responses must route through Pool.Observe

## Symptom

被封号的账号永远不会被标记为 blocked：admin probe 收到 403 ban 响应后调用 `ObserveSuccess`，账号状态保持 active，后续请求持续路由到这个已封号的账号，全部失败。

## Root Cause

来自 commit `ed153cc` 的实际 bug。probe 对所有上游响应统一调用 `ObserveSuccess`，没有区分成功和失败。relay 路径有完整的 Observe 逻辑，但新增的 probe 路径是"天真"的。

## Correct Approach

1. **每个与上游交互的代码路径都必须检查 status code 并正确路由**：
   ```go
   // WRONG: 对所有响应统一调 ObserveSuccess
   s.pool.ObserveSuccess(a.ID, resp.Header)

   // RIGHT: 区分成功和失败
   if resp.StatusCode >= 400 {
       body, _ := io.ReadAll(resp.Body)
       resp.Body.Close()
       s.pool.Observe(pool.UpstreamResult{
           AccountID:  a.ID,
           StatusCode: resp.StatusCode,
           Body:       body,
           Headers:    resp.Header,
       })
   } else {
       s.pool.ObserveSuccess(a.ID, resp.Header)
   }
   ```
2. 新增任何直接调用上游 API 的代码（probe、health check、admin test）都必须遵循此模式
3. 只有确认 2xx 的响应才能使用 ObserveSuccess
