---
match: WithAttrs|WithGroup
action: inject
match_on: new
paths: internal/events/loghandler
---

# WithAttrs/WithGroup must share parent's mutex

## Symptom

并发写 ring buffer 导致日志条目乱序或 panic：`concurrent map writes` on `subscribers` map。

## Root Cause

`WithAttrs` 和 `WithGroup` 创建新的 `LogHandler` 时用了 `mu: sync.RWMutex{}`（新 mutex），但共享了父 handler 的 `ring` slice、`subscribers` map、`ringPos` 和 `ringCount`。两个 handler 用不同的锁写同一片内存 = 数据竞态。

```go
// BUG: 新 mutex 无法保护共享的 ring 和 subscribers
return &LogHandler{
    ring:        h.ring,        // 共享 slice
    subscribers: h.subscribers, // 共享 map
    mu:          sync.RWMutex{}, // 新 mutex — 保护不了共享状态
}
```

## Correct Approach

共享同一个 mutex（改为 `*sync.RWMutex` 指针）：
```go
return &LogHandler{
    ring:        h.ring,
    subscribers: h.subscribers,
    mu:          h.mu,  // 共享父 handler 的 mutex 指针
}
```

或者让 WithAttrs/WithGroup 返回一个代理，把 `Handle` 委托给原始 handler。
