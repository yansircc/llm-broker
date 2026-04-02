# Claude Relay 防封加固方案 v3

## 背景

当前中转服务已有基础防护（身份改写、遥测拦截、header 过滤），但 `/v1/messages` 请求中仍有两类高风险泄漏点：system prompt 中嵌入的环境文本，以及 `metadata.user_id` 中嵌入的 device_id。参考 cc-gateway 项目的 rewriter 方案，提出以下加固计划。

---

## 数据路径实证

各字段出现在哪个端点：

| 字段 | `/v1/messages` | `/api/event_logging/batch` | 当前处理 |
|------|:-:|:-:|------|
| `metadata.user_id`（含 device_id） | ✓ | — | 已改写，但 device_id 未清洗 |
| system prompt `<env>` 文本块 | ✓ | — | **未处理** |
| `env` 对象（40+ 环境维度） | ✗ | ✓ | 已拦截（遥测端点返回 200） |
| `process` 指标（RAM/heap/rss） | ✗ | ✓ | 已拦截（同上） |
| `baseUrl` / `gateway` | ✗ | ✓ | 已拦截（同上） |
| headers（UA/stainless） | ✓ | ✓ | 部分处理 |

**关键结论**：`env` 对象和进程指标只在遥测端点，而遥测端点已被完全拦截。真正需要加固的是 `/v1/messages` 路径上的 system prompt 文本和 metadata。

---

## 现状评估

| 防护能力 | 当前状态 | 说明 |
|----------|----------|------|
| `metadata.user_id` 改写 | ⚠️ 部分 | 外层已改写，但内嵌 device_id 未清洗 |
| 遥测端点拦截 | ✅ 完备 | 返回 200，env/process/baseUrl 均不转发 |
| 认证 header 不转发 | ✅ 完备 | x-api-key、Authorization 已剥离 |
| 计费文本剥离 | ✅ 完备 | 从 system prompt 中移除 billing 文本块 |
| User-Agent 统一 | ⚠️ 写死版本 | `claude-cli/2.2.0`，版本过时是异常信号 |
| System prompt 环境文本 | ❌ 未处理 | Platform/Shell/OS/Working directory 明文泄漏 |
| `x-stainless-*` 稳定绑定 | ✅ 已有 | 按账号绑定，方向正确 |
| `x-anthropic-billing-header` | ✅ 不转发 | 不在 header 白名单中，上游收不到 |

---

## 加固计划（5 项）

### P0 — 高优先级（直接关联封号风险）

#### 1. System Prompt `<env>` 文本块清洗

- **风险**：每次 `/v1/messages` 请求的 system prompt 包含 `<env>` 块，明文暴露：
  - `Platform: darwin` → 操作系统
  - `Shell: zsh` → shell 类型
  - `OS Version: Darwin 24.4.0` → 精确内核版本
  - `Working directory: /Users/jack/projects/foo` → 用户名和项目路径
  - `Primary working directory: ...` → 同上
- **方案**：在 Claude driver 调用 `identity.Transform()` 时，对 system prompt 文本做正则替换：
  - Platform / Shell / OS Version → 账号级别的规范值
  - 路径前缀 `/Users/xxx/`、`/home/xxx/` → 规范化路径
- **改动范围**：新增 `internal/identity/prompt_sanitize.go`，由 `transform.go` 调用
- **参考实现**：`cc-gateway/src/rewriter.ts:90-136`

#### 2. `metadata.user_id` 全链路 device_id 清洗

- **风险**：Claude Code 客户端将 `metadata.user_id` 设置为 JSON 字符串，内含 `device_id`（64-char hex）、`account_uuid`、`session_id`。当前改写逻辑替换了外层 user_id，但如果内层 device_id 透传到上游，多客户端共享同一 device_id 会被关联
- **方案**：需要同时修改两个阶段：
  1. **Plan() 阶段的 session 提取**：`claudeSessionUUID()` (`internal/driver/claude.go:350`) 当前从 `metadata.user_id` 提取 session UUID。如果 user_id 是 JSON 格式（`{"device_id":"...","account_uuid":"...","session_id":"..."}`），需要扩展解析逻辑，从 JSON 中提取 `session_id` 供 session binding 使用
  2. **Transform() 阶段的 user_id 改写**：`RewriteUserID()` (`internal/identity/rewrite.go:15`) 将整个 user_id 替换为账号级别的规范格式，确保 device_id 不透传
  - **两个阶段必须一起改**，否则会出现"device_id 被清洗了，但会话绑定失效"的半成品
- **改动范围**：`internal/driver/claude.go`（`claudeSessionUUID`）、`internal/identity/rewrite.go`（`RewriteUserID`）
- **参考实现**：`cc-gateway/src/rewriter.ts:39-49`

### P1 — 中优先级（header 侧信道）

#### 3. User-Agent 版本可配置化

- **风险**：当前写死 `claude-cli/2.2.0`，Claude Code 频繁更新，过时版本是异常信号
- **方案**：User-Agent 版本改为服务端配置项（`CLAUDE_CLI_VERSION`），管理员定期更新。**不从客户端提取**，保证同一出口所有请求版本一致
- **改动范围**：`internal/config/config.go`（新增字段）、`internal/identity/headers.go`（读取配置）

### P2 — 低优先级（纵深防御）

#### 4. 请求速率随机化

- **风险**：固定间隔的请求模式不自然
- **方案**：在 relay 转发前加入 0-300ms 的随机 jitter（在 relay 层，不侵入 pool）
- **改动范围**：`internal/relay/relay_attempt.go`

#### 5. `x-stainless-*` Header 规范化增强

- **风险**：当前按账号绑定回放，方向正确。但 `x-stainless-os`、`x-stainless-arch` 等字段应与账号 canonical profile 一致
- **方案**：确保 stainless header 中的 os/arch 与账号 profile 对齐（canonicalize，**不是** randomize）
- **改动范围**：`internal/identity/stainless.go`
- **迁移策略**：现有 stainless 绑定 TTL 为 24 小时（`internal/pool/pool_runtime.go:155`），上线后旧绑定不会立即生效。需要在部署时**一次性清理旧绑定**（通过 migrate 命令或 admin API 清空 stainless binding 表），让所有账号在下次请求时重新捕获与 canonical profile 一致的 stainless header

---

## 已删除项（相比 v1/v2）

| 原编号 | 原标题 | 删除原因 |
|--------|--------|----------|
| v1 P0 #2 | 请求 Body 环境指纹正规化 | `env` 对象只在遥测端点，已被拦截 |
| v1 P0 #3 | 进程指标遮蔽 | 只在遥测端点，已被拦截 |
| v2 P1 #4 | billing header 正规化 | `x-anthropic-billing-header` 不在 header 白名单中，上游收不到，目标不存在 |
| v1 P1 #6 | 遥测 Body 深度清洗 | 已被拦截；客户端绕过是网络侧问题 |
| v1 P1 #7 | stainless 自然化变异 | 方向错误，应 canonicalize 而非 randomize |
| v1 P2 #9 | 账号冷却策略增强 | 违反架构边界 |
| v1 P2 #10 | 客户端 Device ID 正规化 | 与 P0 #2 同一条链路 |

---

## 实施顺序

```
Phase 1（1 天）：P0 #1 #2 — prompt 清洗 + device_id 全链路清洗
Phase 2（0.5 天）：P1 #3 — UA 版本可配置
Phase 3（0.5 天）：P2 #4 #5 — jitter + stainless 对齐 + 旧绑定迁移
```

## 技术设计要点

- Prompt 清洗和 device_id 清洗在 `internal/identity/` 包实现，由 Claude driver 通过 `identity.Transform()` 调用。`claudeSessionUUID()` 的 JSON 解析属于 Claude 协议面，留在 driver 侧
- **Canonical profile 运行时确定性生成**：从 `account_id + 服务端种子` 通过 SHA256 确定性推导 platform/shell/os/home_dir 组合。不落库、不加列。只有在将来证明必须跨重启保持随机采样结果时，才讨论持久化
- stainless 上线时需配合**一次性清理旧绑定**，否则现网账号 24 小时内不会生效
- 单元测试用真实请求样本作为 fixture，覆盖每项改写
- 改写发生在上游请求构造前，不影响现有请求日志（日志记录改写前的原始数据）
