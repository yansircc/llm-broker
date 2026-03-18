# 2026-03-18 Claude Compat 请求包络排障记录

## 目标

本轮要解决的问题很具体：

- `compat` 路径收到 OpenAI chat completions 请求后，翻译成 Claude 请求再上游转发
- 同一时期，`native` Claude 请求可以成功
- 用户观察到失败 case 的消息内容里包含多次香蕉测试词，例如 `hellow nano banana`、`hello red banana`
- 真正重要的维度不是 key、model、account，而是“请求包络是否对齐”

因此本轮排障的核心问题只有一个：

> `compat -> Claude` 的翻译结果，和可被上游接受的 Claude 请求，究竟差在哪一层包络上？

## 一开始为什么难排

初始观测有两个硬伤：

1. `request_log.client_*` 对 `compat` 请求记录错了对象
   - 记录的是翻译后的 Claude body
   - 不是 broker 真正收到的原始 OpenAI compat body
   - 这会导致“客户端原始请求”和“broker 翻译后的上游请求”混在一起，没法做准确对照

2. body 只靠 excerpt，不够做精确回放
   - 尤其是 Claude 请求体较大时，只看 excerpt 很容易误判

如果不先修这两个问题，后面的所有“对比差异”都不牢靠。

## 先补的观测

本轮先把观测补齐，再开始定位：

1. 区分原始 compat client request 和翻译后的 upstream Claude request
   - `client_headers_json`
   - `client_body_excerpt`
   - `request_meta_json`
   这些字段现在对应 broker 收到的原始 compat 请求

2. 保留翻译后的上游请求观测
   - `upstream_request_headers_json`
   - `upstream_request_meta_json`
   - `upstream_request_body_excerpt`

3. 完整 body 默认落盘，不再只在截断时才保存
   - `request-log-blobs/client-request/...`
   - `request-log-blobs/upstream-request/...`
   - `request-log-blobs/upstream-response/...`

4. 新增结构化摘要，方便筛选差异
   - `top_level_keys`
   - `system_kind`
   - `system_count`
   - `system_cache_control_count`
   - `message_count`
   - `tools_count`
   - 各类 sha256 摘要

5. 新增 replay 工具
   - `scripts/replay-request.sh`
   - 可以直接按 `request_log.id` 读取生产机上的原始请求并重放

补完这些后，排障才从“猜”变成“可复现”。

## 真正的定位过程

### 阶段 1：确认不是账号、模型、key 问题

对照发现：

- 同时间窗口内，`native` Claude 请求可成功
- 失败集中发生在 `compat /v1/chat/completions -> claude-sonnet-4-6`
- 用户也明确要求不要把排查方向带偏到 key、model、account

所以调查焦点收敛到“翻译后的 Claude 请求包络”。

### 阶段 2：先看失败样本的共性

关键失败样本：

- `request_log.id=222053`
- 路径：`/compat/v1/chat/completions`
- 结果：`upstream_400`

在旧逻辑下，这类失败样本的共性是：

- `top_level_keys` 看起来并不离谱
- 但 `system_kind=string`
- `system_count` 为空
- `system_cache_control_count` 为空

这说明 `compat` 翻译时把 `system` 生成为了单字符串，而不是 Claude Code 风格的 block 数组。

### 阶段 3：做消融实验，不再靠猜

围绕 `request_log.id=222053` 做了一轮 replay / 消融实验：

1. 原样重放失败译文
   - 结果：`400`

2. 只改 `stream`
   - 结果：`400`

3. 只改 `tools`
   - 结果：`400`

4. 只把 `system` 换成成功 native 样本的 Claude Code 风格 block
   - 结果：`200`

5. 再收敛成最小可行条件
   - 第一段 `system` 固定为：
     `You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK.`
   - 第二段再放原本由 compat 侧拼出来的 system/developer/response_format 指令
   - 结果：`200`

这个实验把问题坐实了：

> 对当前这条 modern Claude compat 路径，关键不是 `stream`、不是 `tools`，关键是 `system` 包络必须长得像 Claude Code。

## 最终结论

对本次 `compat -> Claude` 失败 case，决定性差异是：

- 失败版本：`system` 是单字符串
- 成功版本：`system` 是 block 数组，并且前导 block 必须带 Claude Code 身份前缀

也就是说，真正要对齐的不是“文字内容大致相似”，而是：

1. `system` 的类型
2. `system` 的 block 数量
3. 第一段 block 的 Claude Code 前导身份
4. block 上的 `cache_control`

## 代码修复

最终落地的修复包括：

1. `compatClaudeRequest.System` 从 `string` 改为 `any`
2. 新增 `compatClaudeSystemValue()`
3. modern Claude envelope 下，`system` 改为两段 blocks
   - block 1：固定 Claude Code 身份前导
   - block 2：原先 compat 翻译出的 system/developer/response_format 指令
4. 两段 block 都带 `cache_control: { type: "ephemeral" }`

相关文件：

- `internal/server/compat_openai_types.go`
- `internal/server/compat_openai_claude.go`
- `internal/server/compat_openai_chat.go`
- `internal/server/compat_openai_chat_test.go`

## 中途最坑的一次误判：代码其实对了，但没有真正在线上生效

本轮不是“改完就结束”，中间还踩了一个发布层面的坑。

现象：

- 本地测试通过
- replay 也曾短暂成功
- 但用户再发真实请求，线上仍然继续报 `400`

继续往下查后发现，问题不在协议本身，而在部署流程：

1. 蓝绿部署时，流量一度切到了新版本 `green`
2. 但 deploy 脚本随后执行数据库 invariant 校验
3. 校验发现线上存在历史遗留的孤儿 `quota_buckets`
4. 因此 deploy 自动回滚
5. 公网又切回旧版本 `blue`

表面看起来像“已经发版”，实际上公网仍在跑旧逻辑。

现场证据：

- `/etc/caddy/llm-broker.upstream` 又回到 `127.0.0.1:3001`
- 最新失败日志仍然是：
  - `system_kind=string`
  - `system_count=null`
  - `system_cache_control_count=null`

这一步如果不查清，很容易误以为修复方向错了。

## 为什么 deploy 会回滚

回滚不是因为新代码坏了，而是因为线上数据库里有历史派生状态垃圾：

- `accounts` 的有效 bucket 数：`9`
- `quota_buckets` 表里的记录数：`14`

多出来的 5 个 bucket 都已经不再被任何账号引用，属于孤儿派生状态：

- `claude:0e0b3bdc-a74c-4407-9b01-65bc92032fc5`
- `claude:48866501-8a1f-4b55-bc5a-73b31641b03f`
- `claude:5b887ee8-c6d4-4dd8-9c93-bfd3f7dce279`
- `claude:ad506281-6321-481a-a77f-6dc1ef84f8a7`
- `claude:cdab258c-6b48-4cab-aadb-6a2a4f83f437`

这些 bucket 不是账号主数据，而是派生状态，所以正确做法不是绕过校验，而是让迁移把它们清掉。

## 针对 deploy 回滚的永久修复

为避免以后继续被同类历史垃圾卡住，额外修了 `quota_buckets` 迁移逻辑：

1. `migrateQuotaBucketsTable()` 不再只在空表时 seed
2. 每次 migrate 都会：
   - 从 `accounts` 同步缺失 bucket
   - 删除不再被任何账号引用的孤儿 bucket

对应测试：

- `TestMigrate_QuotaBucketsPrunesOrphansAndBackfillsMissing`

这保证以后发布时，如果库里残留历史 bucket，迁移会自动收敛到真实账号集合，而不是把发布打回。

## 最终验证闭环

### 1. 本地测试

- `go test ./internal/relay ./internal/server`
- `go test ./internal/store ./internal/server ./internal/relay`
- `go test ./...`

全部通过。

### 2. 发布验证

最终成功发布后：

- 公网流量稳定在 `green`
- `caddy` 指向 `127.0.0.1:3002`
- 数据库 invariant 通过：
  - `accounts=9`
  - `buckets=9`
  - `distinct_bucket_keys=9`

### 3. replay 验证

使用历史失败样本：

- `request_log.id=222053`

对公网重放后，返回：

- `HTTP 200`

对应成功日志：

- `request_log.id=222489`
- 时间：`2026-03-18 13:06:35 UTC`
- `status=ok`
- `system_kind=array`
- `system_count=2`
- `system_cache_control_count=2`

### 4. 用户真实请求验证

用户再次发送同样内容后，线上最新 compat 请求变为成功：

- `222496`
  - `2026-03-18 13:08:01 UTC`
  - `status=ok`
  - `system_kind=array`
  - `system_count=2`
- `222497`
  - `2026-03-18 13:08:04 UTC`
  - `status=ok`
  - `system_kind=array`
  - `system_count=1`

这说明问题已经从真实流量角度闭环，而不只是实验样本成功。

## 这次排障留下的有效方法

本轮真正有效的方法不是“多看几眼日志”，而是下面这条链路：

1. 先把原始 client request 和翻译后的 upstream request 彻底拆开观测
2. 完整 body 落盘，保证可以复现
3. 做 request-level replay，而不是人工拼请求猜测
4. 用消融实验定位决定性字段
5. 发布后不要只看 `/health`，要回查真实请求是否命中新逻辑
6. 如果发布又回旧版本，先查 deploy/rollback 过程，不要误判为代码无效

## 当前结论

截至 `2026-03-18 13:08 UTC`，本次 `compat -> Claude` 问题的结论已经明确：

1. 真正导致 `400` 的主因是 `system` 包络不对齐
2. 修复方式是把 `system` 改成 Claude Code 风格 block 数组，而不是单字符串
3. 中途“改了但还失败”的原因是 deploy 因孤儿 `quota_buckets` 校验失败而自动回滚
4. 该部署问题也已做永久修复
5. 线上真实 compat 请求已经成功
