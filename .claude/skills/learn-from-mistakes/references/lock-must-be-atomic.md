---
match: AcquireRefreshLock|SetNX|refreshLocks
action: inject
match_on: new
paths: internal/
---

# Lock operations must be atomic

## Symptom

两个 goroutine 同时刷新同一账号的 token：goroutine A 检查锁不存在 → goroutine B 检查锁不存在 → 两者都获取锁 → 两者都发起 refresh → 一个 refresh token 被消耗后另一个失败。

## Root Cause

用 Get + Set 两步实现分布式锁（check-then-act）不是原子操作。在两步之间存在竞态窗口。

## Correct Approach

1. **使用 SetNX 语义（set-if-not-exists）实现原子锁获取**：
   ```go
   func (p *Pool) AcquireRefreshLock(accountID string) bool {
       p.mu.Lock()
       defer p.mu.Unlock()
       if _, held := p.refreshLocks[accountID]; held {
           return false  // already locked
       }
       p.refreshLocks[accountID] = time.Now()
       return true  // acquired
   }
   ```
2. 检查 + 设置必须在同一个临界区内（同一个 `mu.Lock()` 范围）
3. 不要用 `if !exists { set }` 两步分离的模式
4. 释放锁同样需要在锁内操作：`delete(p.refreshLocks, accountID)`
