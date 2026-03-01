---
match: transportKey|return "direct"
action: inject
match_on: new
paths: internal/transport/
---

# Transport key must include account ID

## Symptom

不同账号的请求共享同一个 HTTP/2 连接池，Anthropic 可能通过 TLS session / HPACK 状态关联多个账号，暴露 relay 模式。

## Root Cause

`transportKey()` 对无 proxy 的账号统一返回 `"direct"`，所有账号共享一个 `http2.Transport`。HTTP/2 连接是多路复用的，同一连接上的请求共享 TLS session ticket 和 HPACK 动态表。

```go
// BUG: 所有无 proxy 账号共享一个连接池
func transportKey(acct *domain.Account) string {
    if acct.Proxy == nil {
        return "direct"  // 所有账号 → 同一个 key
    }
}
```

## Correct Approach

将 `acct.ID` 加入 transport key，每个账号独立连接池：
```go
func transportKey(acct *domain.Account) string {
    if acct.Proxy == nil {
        return "direct:" + acct.ID
    }
    return fmt.Sprintf("%s://%s:%d/%s", acct.Proxy.Type, acct.Proxy.Host, acct.Proxy.Port, acct.ID)
}
```

3-7 个账号的连接池开销可忽略。
