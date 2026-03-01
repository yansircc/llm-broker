---
match: onAuthFailure|\.on[a-zA-Z]+Failure
action: inject
match_on: new
paths: internal/pool/
---

# Callback under Pool.mu lock must be dispatched as goroutine

## Symptom

生产环境死锁：Observe() 持有 `p.mu.Lock()`，同步调用 `p.onAuthFailure(acct.ID)` → onAuthFailure 触发 token 刷新 → 刷新完成后调用 `pool.StoreTokens()` → StoreTokens 试图获取 `p.mu.Lock()` → 死锁。进程永久挂起。

## Root Cause

Go 的 `sync.RWMutex` 不可重入。在持有锁的代码段内同步调用任何可能回调 Pool 方法的函数，必然死锁。

## Correct Approach

1. **在锁内调用可能回调 Pool 的函数时，必须用 goroutine 分发**：
   ```go
   // pool.go 现有正确模式
   p.mu.Lock()
   // ... 修改状态 ...
   if p.onAuthFailure != nil {
       go p.onAuthFailure(acct.ID)  // goroutine 分发，避免死锁
   }
   p.mu.Unlock()
   ```
2. nil check 是必须的 — callback 通过 `SetOnAuthFailure` 延迟注册
3. 新增任何从 Observe/Update 路径触发的 callback 时，都必须用 `go` 前缀
