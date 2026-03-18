# 2026-03-18 Claude Native 请求包络分析

## 背景

本轮排障的目标不是检查账号、quota 或 cooldown，而是判断进入 broker 的 `native` Claude 请求，哪些包络可以被上游接受，哪些包络会被上游拒绝。

分析窗口主要覆盖：

- 生产环境 `https://ccc.210k.cc`
- 时间范围：`2026-03-18 10:26 UTC` 到 `2026-03-18 11:08 UTC`
- 重点 provider：`claude`
- 重点 surface：`native`

## 已做的观测增强

为避免后续继续靠 excerpt 猜，本轮已上线以下观测：

1. `request_log` 记录：
   - `client_headers_json`
   - `client_body_excerpt`
   - `request_meta_json`
   - `upstream_request_headers_json`
   - `upstream_request_meta_json`
   - `upstream_request_body_excerpt`
   - `upstream_response_meta_json`
   - `upstream_response_body_excerpt`

2. `excerpt` 不再只保留开头，改为首尾双保留。

3. `request_meta_json` / `upstream_request_meta_json` 额外记录结构化摘要：
   - `top_level_keys`
   - `message_count`
   - `tools_count`
   - `tool_names`
   - `tool_signatures`
   - `tools_sha256`
   - `messages_sha256`
   - `thinking_sha256`
   - `output_config_sha256`
   - `context_management_sha256`
   - `metadata_sha256`

4. 超长 body 不再只靠 sqlite 文本列保存：
   - 完整 client request body 落盘到 `request-log-blobs/client-request/...`
   - 完整 upstream request body 落盘到 `request-log-blobs/upstream-request/...`
   - 如有完整错误响应 body，也会落到 `request-log-blobs/upstream-response/...`

5. `compat` 请求不再把翻译后的 Claude body 误记成 client request：
   - `client_headers_json` / `client_body_excerpt` / `request_meta_json` 现在对应原始 OpenAI compat 请求
   - 翻译后的 Claude 请求仍记录在 `upstream_request_*`
   - 因此现在可以直接做“原始 compat 请求 vs 翻译后的 Claude 请求”的一一对照

6. body artifact 现在默认全量落盘，不再只有截断时才落盘：
   - 小请求同样会有 `request_meta_json.body_artifact_path`
   - 这保证 replay 时不需要依赖 excerpt

7. 新增本地 replay 工具：
   - 脚本：`scripts/replay-request.sh`
   - 用途：从 `request_log.id` 读取 path / headers / 原始 body artifact，直接重放到任意 broker 地址
   - 可本地数据库使用，也可通过 `--remote root@...` 读取生产机上的 sqlite 和 artifact 后在本机回放

## 已确认的请求家族

注意：这里说的“家族”指进入 broker 时的请求包络形状，不是账号、模型或 key。

### 1. 现代 CLI 完整包络

典型 top-level keys：

```json
[
  "context_management",
  "max_tokens",
  "messages",
  "metadata",
  "model",
  "output_config",
  "stream",
  "system",
  "thinking",
  "tools"
]
```

或：

```json
[
  "max_tokens",
  "messages",
  "metadata",
  "model",
  "output_config",
  "stream",
  "system",
  "temperature",
  "tools"
]
```

典型客户端：

- `claude-cli/2.1.78 (external, cli)`

状态：

- 稳定成功

样本：

- `fx` 的 `claude-opus-4-6`
  - `2026-03-18 11:00:03 UTC` `req_011CZARqproGs8FazFtiHmHY`
  - `2026-03-18 11:00:12 UTC` `req_011CZARsKN5PmFGcE4VGGt3u`
- `binbin` 的 `claude-sonnet-4-6`
  - `2026-03-18 11:06:23 UTC` `req_011CZASLHp4SMZfZnb6kCRTP`
  - `2026-03-18 11:08:14 UTC` `req_011CZASUqqrvBy5WDgSuwhHD`

结论：

- 当前上游接受这类“完整 Claude CLI 风格”包络。

### 2. 老 CLI 极简 native 包络

典型 top-level keys：

```json
[
  "max_tokens",
  "messages",
  "model",
  "stream"
]
```

典型客户端：

- `claude-cli/2.1.2 (external, cli)`

典型请求形状：

```json
{
  "max_tokens": 1,
  "messages": [{"role": "user", "content": "Who are you?"}],
  "model": "claude-opus-4-6",
  "stream": true
}
```

状态：

- 稳定失败

样本：

- `2026-03-18 10:57:10 UTC` `req_011CZAReMPzweimedsiUvX6B`
- `2026-03-18 10:57:14 UTC` `req_011CZARebHvKopjzsL193uhr`
- `2026-03-18 10:57:17 UTC` `req_011CZARepxSoyR43d9jmETuY`
- `2026-03-18 11:00:19 UTC` `req_011CZARtGh7qFooeAsPtFHVc`

错误：

- 全部是 `400 invalid_request_error: Error`

结论：

- 不是 `opus` 模型整体坏掉。
- 是“过旧、过简的 native 包络”被上游拒绝。

### 3. Anthropic/JS 0.73.0 / OpenClaw 风格 `system + tools` 包络

典型 top-level keys：

```json
[
  "max_tokens",
  "messages",
  "model",
  "stream",
  "system",
  "tools"
]
```

典型客户端：

- `Anthropic/JS 0.73.0`

状态：

- 对 `claude-haiku-4-5` 可成功
- 对 `claude-sonnet-4-6` 稳定失败

失败样本：

- `2026-03-18 10:54:42 UTC` `req_011CZARTQVERFonQX6hevkUY`
- `2026-03-18 10:52:16 UTC` `req_011CZARGg62hgH4XS8W1tKJe`
- `2026-03-18 11:00:43 UTC` `req_011CZARuzvtW2LTMsA5n1YU3`

结论：

- 这不是 transport / quota 问题。
- 同一类客户端包络，对不同模型的可接受性不同。
- 当前证据支持：这类包络对 `sonnet-4-6` 不够对齐。

### 4. claude-code/1.0 风格包络

典型 top-level keys：

```json
[
  "max_tokens",
  "messages",
  "model",
  "output_config",
  "stream",
  "system",
  "thinking",
  "tools"
]
```

典型客户端：

- `claude-code/1.0`

状态：

- 对 `claude-sonnet-4-6` 失败

样本：

- `2026-03-18 10:42:20 UTC` `req_011CZAQWguBA3R1ScsQaHQMj`

结论：

- 单独补 `thinking` / `output_config` 不是充分条件。

### 5. bare non-stream 简单包络

典型 top-level keys：

```json
[
  "max_tokens",
  "messages",
  "model"
]
```

状态：

- 对 `claude-sonnet-4-6` 失败

样本：

- `2026-03-18 10:30:44 UTC` `req_011CZAPdPMJwp56goqYMvVvM`
- `2026-03-18 10:30:45 UTC` `req_011CZAPdWL1angBhoqdTTvvf`

结论：

- 最裸的简化包络也会被拒绝。

### 6. 模型不存在/不支持类请求

样本：

- `claude-haiku-4-6`
- `2026-03-18 11:06:24 UTC` 到 `11:06:26 UTC`

错误：

- `404`

结论：

- 这类不属于本轮 `400 invalid_request_error` 的同一种问题。
- 更像是模型名无效或当前上游不可用。

## 当前最重要的工程结论

### 结论 1：问题不在账号池

同一个时间段：

- `fx` 的 `claude-opus-4-6` 连续成功
- `binbin` 的 `claude-opus-4-6` 极简请求连续失败
- `binbin` 稍后改成现代 CLI 完整包络后，`claude-sonnet-4-6` 又连续成功

因此可以排除：

- 账号整体失效
- cell 整体异常
- cooldown 误判为主因
- 模型整体全坏

### 结论 2：问题在请求包络，不在单一字段

已经能确认：

- 现代完整 CLI 包络可以通过
- 老 CLI 极简包络会失败
- `Anthropic/JS 0.73.0` 的 `system + tools` 包络在 `sonnet-4-6` 上失败
- `thinking` / `output_config` 并不是唯一决定因素

所以问题不是某一个 header 或某一个字段单独缺失，而是“整个请求家族是否足够像当前 Anthropic 接受的 Claude CLI 风格”。

### 结论 3：当前 broker 不应再把所有 native 请求当成一种东西

对 `native` 路径，至少已经观察到 3 类需要区别处理的请求：

1. 现代 CLI 完整包络：可直接转发
2. 老 CLI 极简包络：高概率被上游拒绝
3. Anthropic/JS / OpenClaw 风格包络：需要专门对齐，至少在 `sonnet-4-6` 上如此

## 现阶段不再需要争论的事实

以下判断已经有足够证据支撑：

1. `400 invalid_request_error: Error` 不是 quota / cooldown 的表现。
2. `native` 流量中，成功与失败主要由“请求包络家族”决定。
3. 同一 key、同一 broker、同一时间段，可以同时看到：
   - 极简包络失败
   - 完整包络成功
4. 因此问题属于“请求对齐”而不是“账号调度”。

## 后续 replay 结论

在补齐原始 request / upstream request / response artifact 之后，继续对 `native` 失败样本做了最小差异 replay。

关键失败样本：

- `222611`
  - `claude-sonnet-4-6`
  - `top_level_keys=["max_tokens","messages","metadata","model","stream","system","tools"]`
  - `system_count=1`
- `222573`
  - `claude-sonnet-4-6`
  - `top_level_keys=["max_tokens","messages","metadata","model","output_config","stream","system","thinking","tools"]`
  - `system_count=1`
- `222613`
  - `claude-sonnet-4-6`
  - `top_level_keys=["max_tokens","messages","metadata","model"]`
  - 无 `system`

对这三类失败样本，分别只做一个修改：

- 在 `system` 最前面补一个 Claude Code 风格前导 block
- 文本为：
  `You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK.`

结果：

- 三类样本全部从 `400` 变为 `200`

这说明对当前 OAuth Claude 账号而言，`sonnet/opus` 相关 native 请求的决定性因素已经明确：

1. 不是 `thinking` / `output_config` 单独决定
2. 不是 `stream` 单独决定
3. 关键是 `system` 必须具备 Claude Code 风格前导 block
4. 对 `string system`，不能只保留字符串，要落成 Claude text block 数组

## 已落地修复

修复位置：

- `internal/driver/claude.go`
- `internal/driver/claude_envelope.go`

修复策略：

1. 只在 Claude driver 边界做规范化，不碰 core
2. 只对 `messages` 请求做，不影响 `count_tokens`
3. 只对 `claude-sonnet-4*` / `claude-opus-4*` 生效
4. 规范化规则：
   - `system` 缺失：补成一个 Claude Code block
   - `system` 是字符串：改成 `[ClaudeCodeBlock, 原system文本block]`
   - `system` 是 block 数组但第一块不是 Claude Code：前插 Claude Code block
   - 已经是 Claude Code 风格：保持不变

补充测试：

- `internal/driver/claude_build_request_test.go`

## 上线验证

发布时间：

- `2026-03-18 13:27 UTC` 左右

上线后用历史失败样本原样 replay：

- `222611` -> `HTTP 200`
- `222573` -> `HTTP 200`
- `222613` -> `HTTP 200`

对应线上新日志：

- `222737`
  - `status=ok`
  - `model=claude-sonnet-4-6`
  - `system_kind=array`
  - `system_count=2`
- `222738`
  - `status=ok`
  - `model=claude-sonnet-4-6`
  - `system_kind=array`
  - `system_count=2`
- `222739`
  - `status=ok`
  - `model=claude-sonnet-4-6`
  - `system_kind=array`
  - `system_count=1`

并且在部署后窗口内，未再观察到新的 `upstream_400`。

## compat 切流后的观察重点

用户已说明接下来会把 `compat` 请求切进来，并且都会走 `kun` 账号。

兼容层切流后，重点关注：

1. `provider='claude' AND surface='compat'`
2. `account_id='2f7183ba-4398-446a-b5a7-b0b421bc3115'`（kun）
3. compat 翻译后 upstream request 的：
   - `top_level_keys`
   - `tools_count`
   - `tools_sha256`
   - `body_artifact_path`
   - `upstream_status`
   - `upstream_error_type`

重点目标：

- 验证 compat 翻译后的包络是否更稳定地落在“现代完整 CLI 风格”附近
- 验证 compat 是否能避开当前 native 的多家族碎片化问题

## 建议的后续动作

1. 不要再把所有 `native` 请求透明等价对待。
2. 对明显过旧的 native 极简包络，优先考虑：
   - 入口识别
   - 返回可读错误
   - 或做显式升级/对齐
3. 对后续仍出现的 `native 400`，继续按“失败样本 replay + 单变量消融”处理，不再靠泛化猜测。
4. compat 切流后，优先观察它是否天然绕开 native 当前的问题家族。
