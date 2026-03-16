---
match: uTLSHandshake|HandshakeContext
action: inject
match_on: new
paths: internal/transport/
---

# uTLS handshake must use parent context

## Symptom

请求取消或超时后，TLS 握手仍在后台继续，泄漏 goroutine 和文件描述符。在 529 overload 期间连接积压导致 VPS fd 耗尽。

## Root Cause

`uTLSHandshake` 接受 `ctx` 参数但忽略它（`_ context.Context`），改用 `context.Background()` 调用 `HandshakeContext`：

```go
// BUG: 忽略传入的 context
func uTLSHandshake(_ context.Context, rawConn net.Conn, serverName string) (net.Conn, error) {
    // ...
    if err := tlsConn.HandshakeContext(context.Background()); err != nil {
```

## Correct Approach

传递实际 context：
```go
func uTLSHandshake(ctx context.Context, rawConn net.Conn, serverName string) (net.Conn, error) {
    // ...
    if err := tlsConn.HandshakeContext(ctx); err != nil {
```

这样请求取消时 TLS 握手也会被取消，不会泄漏资源。
