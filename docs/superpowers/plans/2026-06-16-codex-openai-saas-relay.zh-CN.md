# Codex OpenAI SaaS 中转服务实现计划

> **给 agentic worker：** 必须使用子技能：用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 按任务执行本计划。步骤使用 checkbox（`- [ ]`）格式，便于跟踪。

**目标：** 把 broker 改造成只提供 Codex/OpenAI 中转的对外服务，支持客户账号、API key、7pay 人民币充值、按 USD 等价额度做 token 计费，以及注册邀请奖励。

**架构：** 保持 relay 作为请求执行内核，把所有商业状态放到独立 billing 和 admission 边界后面。Codex 是唯一上游 provider。余额从 append-only ledger 加有界 checkpoint 派生；支付订单、请求日志、网关事件、容量遥测都是证据，不是余额真相。

**技术栈：** Go 1.24、SQLite、SvelteKit、标准库 `net/http`、`golang.org/x/crypto/bcrypt`、7pay/zpay 表单 API、现有 request-log artifact 存储。

---

## 文档形态

本分支只使用一个主计划文档。

原因：产品收口、身份、计费、支付、邀请、relay 结算共享同一个不变量：

```text
admit(request) = balance(user) > 0 AND capacity.Allow(user, key) AND abuse.Allow(user, key)
settle(success) = ledger += charge(actual_usage, price_snapshot)
balance(user) = latest_checkpoint.balance + SUM(ledger rows after checkpoint)
```

在这个不变量实现前拆成多个文档，会让不同文档各自维护一部分真相。执行时仍然按 task 分阶段推进。

## 稳定轴、变化轴、不变量

稳定轴：

- `relay` 通过 provider driver 执行已鉴权请求，并提取 usage。
- `driver` 拥有 provider 协议。
- `request_log` 记录运行证据。

变化轴：

- 产品表面收缩为 Codex/OpenAI only。
- 外部客户替代 admin 手动创建的 API user。
- 计费、支付、邀请成为一等产品状态。

不变量：

- Provider-specific 行为终止于 `driver.Driver`。
- 商业状态只通过 `billing_ledger` 进入系统。
- 一次成功的可计费请求，最多创建一条以 request id 为 key 的 debit ledger row。
- 一笔已支付的 7pay 订单，最多创建一条以 order id 为 key 的 credit ledger row。
- 邀请奖励是 ledger credit，不是 user 字段。
- 公开流量必须通过容量闸门；token price 是客户侧计价单位，不是上游成本模型。

## Source of Truth 规则

- `billing_ledger.amount_micros` 是唯一余额真相。Balance checkpoint 是基于 ledger sequence 的有界读取 projection，不是可变余额列。
- `payment_orders.status` 是支付状态，不是余额。
- `request_log.cost_usd` 是运行成本证据，不是客户余额。
- `model_prices` 是当前客户零售价格真相。每条 debit ledger row 保存 `price_snapshot_json`。
- 人民币支付金额和 RMB-to-USD-equivalent 汇率在创建订单时快照。
- `api_keys.token_hash` 用于 relay 请求鉴权。`users` 不保存 API token hash。API key hash 必须是 deterministic SHA-256，这样才能做 indexed lookup；bcrypt 只用于密码。
- `events` 和日志只做观测，不做状态真相。

## 单位经济和容量

上游成本基础和公开 token 计费不是一回事。Codex 账号是订阅席位，真实约束是 rate-limit windows 和账号风险成本；客户侧按 token 计费只是产品上更清晰的零售计价方式。

所以 token pricing 只是收费函数。Admission 还必须执行容量控制：

- global concurrent request limit
- per-user concurrent request limit
- per-API-key request-per-minute limit
- 首次成功付款前的 reward-only 更严格限制
- 可选的 high-cost model 最小余额要求

Capacity settings 是业务策略，不是 provider 协议。Provider-specific utilization 仍然属于 `driver`/`pool`；admission 只消费通用可用性和配置好的客户限制。

## 对外产品表面

Canonical relay：

- `POST /openai/responses`

兼容层：

- `GET /v1/models`
- `POST /v1/responses` 作为同一个 Codex relay path 的 alias
- `POST /v1/chat/completions` 只做基础 text/chat 兼容

兼容层约束：

- 所有 compat path 必须走和 `/openai/responses` 相同的计费链路。
- 不支持的 OpenAI chat 参数在边界处拒绝，并返回 OpenAI-shaped JSON error。
- Compat 不得重新引入 Claude 或 Gemini model routing。

Admin surface：

- 现有 admin token 继续作为 operator root credential。
- Admin UI 只保留 Codex account 的 provider/account 操作。
- Admin billing 页面管理用户、API key、价格、订单、ledger 调整、邀请设置。

Customer surface：

- 注册/登录/登出
- API key 管理
- 余额和用量
- 充值二维码流程
- 邀请链接和奖励历史

## 数据模型

所有 money-like value 使用整数。

```text
1 USD-equivalent credit = 1_000_000 credit_micros
1 RMB = 100 fen
```

### `users`

把现有 `users` 重建为客户身份表。

Columns：

- `id TEXT PRIMARY KEY`
- `email TEXT NOT NULL UNIQUE`
- `name TEXT NOT NULL`
- `password_hash TEXT NOT NULL`
- `email_verified_at INTEGER`
- `status TEXT NOT NULL DEFAULT 'active'`
- `referral_code TEXT NOT NULL UNIQUE`
- `referred_by_user_id TEXT NOT NULL DEFAULT ''`
- `created_at INTEGER NOT NULL`
- `last_login_at INTEGER`

现有 admin 创建的 users 可以通过 `name` 派生 `email` 做本地连续性迁移。这个分支还没有外部客户数据，所以迁移允许显式、窄范围处理。

### `api_keys`

Columns：

- `id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `name TEXT NOT NULL`
- `token_hash TEXT NOT NULL UNIQUE`
- `token_prefix TEXT NOT NULL`
- `status TEXT NOT NULL DEFAULT 'active'`
- `allowed_surface TEXT NOT NULL DEFAULT 'native'`
- `created_at INTEGER NOT NULL`
- `last_used_at INTEGER`

Indexes：

- `idx_api_keys_user ON api_keys(user_id, created_at)`

Hashing rule：

- `api_keys.token_hash = hex(SHA256(plaintext_api_key))`
- API key 是高熵随机 bearer token，所以 deterministic SHA-256 可以接受，并且可以走索引等值查询。
- 不要对 API key 使用 bcrypt；bcrypt 带盐，不能通过 token hash 做 equality lookup。

### `web_sessions`

Columns：

- `id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `token_hash TEXT NOT NULL UNIQUE`
- `created_at INTEGER NOT NULL`
- `last_seen_at INTEGER NOT NULL`
- `expires_at INTEGER NOT NULL`

Indexes：

- `idx_web_sessions_expires ON web_sessions(expires_at)`
- `idx_web_sessions_user ON web_sessions(user_id, expires_at)`

### `email_verifications`

Columns：

- `id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `email TEXT NOT NULL`
- `token_hash TEXT NOT NULL UNIQUE`
- `created_at INTEGER NOT NULL`
- `expires_at INTEGER NOT NULL`
- `consumed_at INTEGER`

Rules：

- MVP 不包含邮箱验证流程。
- 客户创建 API key 和 relay admission 不要求 `users.email_verified_at`。
- 邀请奖励按触发点拆分：受邀方在注册成功时获得奖励；邀请方必须等受邀方成功付费后才获得奖励。
- `email_verifications` 可以作为 legacy schema 暂时保留到后续清理，但不能影响注册、API key、admission 或 referral payout。
- Verification token 存储为 `hex(SHA256(raw_token))`；raw token 只出现在邮件链接里。
- MVP token 过期时间为 1 小时。

### `billing_settings`

可变商业设置的 key-value 表。

Required keys：

- `cny_to_usd_rate_micros`：默认 `1000000`，表示 `1 RMB = 1 USD-equivalent`
- `referral_new_user_bonus_micros`
- `referral_inviter_bonus_micros`

### `admission_limits`

可配置的客户容量限制。

Columns：

- `scope TEXT NOT NULL`
- `scope_id TEXT NOT NULL DEFAULT ''`
- `max_concurrent INTEGER NOT NULL DEFAULT 0`
- `requests_per_minute INTEGER NOT NULL DEFAULT 0`
- `min_balance_micros INTEGER NOT NULL DEFAULT 1`
- `updated_at INTEGER NOT NULL`
- `PRIMARY KEY (scope, scope_id)`

Required scopes：

- `global`
- `user`
- `api_key`
- `reward_only`

Default policy：

- 付费用户：使用 admin 配置的 user/API-key limits
- reward-only 用户：`max_concurrent = 1`
- 未验证用户：不允许 relay admission，也不允许创建 API key

这个表表达业务容量，不解析 Codex rate-limit headers。

### `model_prices`

当前价格表。

Columns：

- `model TEXT PRIMARY KEY`
- `input_micros_per_million INTEGER NOT NULL`
- `output_micros_per_million INTEGER NOT NULL`
- `cache_read_micros_per_million INTEGER NOT NULL DEFAULT 0`
- `cache_create_micros_per_million INTEGER NOT NULL DEFAULT 0`
- `updated_at INTEGER NOT NULL`

Ledger row 会快照匹配到的价格，所以这个表可以原地更新，不会改写历史账单。

### `billing_ledger`

Append-only 客户余额账本。

Columns：

- `seq INTEGER PRIMARY KEY AUTOINCREMENT`
- `id TEXT NOT NULL UNIQUE`
- `user_id TEXT NOT NULL`
- `amount_micros INTEGER NOT NULL`
- `kind TEXT NOT NULL`
- `source_type TEXT NOT NULL`
- `source_id TEXT NOT NULL`
- `idempotency_key TEXT NOT NULL UNIQUE`
- `description TEXT NOT NULL DEFAULT ''`
- `price_snapshot_json TEXT NOT NULL DEFAULT ''`
- `metadata_json TEXT NOT NULL DEFAULT '{}'`
- `created_at INTEGER NOT NULL`

Allowed `kind`：

- `payment_credit`
- `referral_signup_credit`
- `usage_debit`
- `admin_adjustment`

Indexes：

- `idx_billing_ledger_user_seq ON billing_ledger(user_id, seq)`
- `idx_billing_ledger_user_created ON billing_ledger(user_id, created_at)`
- `idx_billing_ledger_source ON billing_ledger(source_type, source_id)`

### `billing_balance_checkpoints`

Append-only ledger 之上的有界读取 checkpoint。

Columns：

- `user_id TEXT PRIMARY KEY`
- `ledger_seq INTEGER NOT NULL`
- `balance_micros INTEGER NOT NULL`
- `created_at INTEGER NOT NULL`

Balance query：

```text
latest = billing_balance_checkpoints[user_id]
balance = latest.balance_micros + SUM(billing_ledger.amount_micros WHERE user_id=? AND seq > latest.ledger_seq)
```

Checkpoint 是派生 projection。删除 checkpoint 后，余额可以从 ledger rows 重新计算。

### `payment_orders`

Columns：

- `id TEXT PRIMARY KEY`
- `out_trade_no TEXT NOT NULL UNIQUE`
- `user_id TEXT NOT NULL`
- `gateway TEXT NOT NULL DEFAULT 'zpay'`
- `status TEXT NOT NULL`
- `product_name TEXT NOT NULL`
- `amount_cny_fen INTEGER NOT NULL`
- `credit_micros INTEGER NOT NULL`
- `exchange_rate_micros INTEGER NOT NULL`
- `payment_type TEXT NOT NULL DEFAULT 'alipay'`
- `zpay_trade_no TEXT NOT NULL DEFAULT ''`
- `qrcode TEXT NOT NULL DEFAULT ''`
- `qr_image TEXT NOT NULL DEFAULT ''`
- `created_at INTEGER NOT NULL`
- `paid_at INTEGER`
- `updated_at INTEGER NOT NULL`

Allowed `status`：

- `pending`
- `paid`
- `failed`
- `expired`

### `payment_events`

记录 gateway callback 和主动查询结果的观测表。

Columns：

- `id TEXT PRIMARY KEY`
- `order_id TEXT NOT NULL`
- `gateway TEXT NOT NULL`
- `event_type TEXT NOT NULL`
- `valid_signature INTEGER NOT NULL`
- `payload_json TEXT NOT NULL`
- `created_at INTEGER NOT NULL`

这个表只是证据，绝不驱动余额。

### `referrals`

Columns：

- `id TEXT PRIMARY KEY`
- `inviter_user_id TEXT NOT NULL`
- `invitee_user_id TEXT NOT NULL UNIQUE`
- `invite_code TEXT NOT NULL`
- `created_at INTEGER NOT NULL`
- `credited_at INTEGER NOT NULL`

Referral fulfillment 在同一个 transaction 里插入 referral record 和两条 ledger row：

- `referral:new_user:<invitee_user_id>`
- `referral:inviter:<invitee_user_id>`

### `billable_requests`

Relay request 的持久计费捕获表。

Columns：

- `request_id TEXT PRIMARY KEY`
- `user_id TEXT NOT NULL`
- `api_key_id TEXT NOT NULL`
- `model TEXT NOT NULL`
- `surface TEXT NOT NULL`
- `status TEXT NOT NULL`
- `input_tokens INTEGER NOT NULL DEFAULT 0`
- `output_tokens INTEGER NOT NULL DEFAULT 0`
- `cache_read_tokens INTEGER NOT NULL DEFAULT 0`
- `cache_create_tokens INTEGER NOT NULL DEFAULT 0`
- `price_snapshot_json TEXT NOT NULL DEFAULT ''`
- `ledger_id TEXT NOT NULL DEFAULT ''`
- `created_at INTEGER NOT NULL`
- `usage_observed_at INTEGER`
- `settled_at INTEGER`

Allowed `status`：

- `in_progress`
- `usage_observed`
- `settled`
- `aborted_no_usage`
- `settlement_failed`

Rules：

- Upstream request 开始前插入 `in_progress`。
- 转发最终 stream completion event 给客户端前，先持久化 final usage。
- Settlement 以 `request_id` 幂等。
- Startup reconciliation 会结算没有 `ledger_id` 的 `usage_observed` rows。

### `request_log`

保留现有 request log 作为运行证据，并增加 correlation fields：

- `request_id TEXT NOT NULL DEFAULT ''`
- `api_key_id TEXT NOT NULL DEFAULT ''`

不要给 `request_log` 增加余额字段。

## 包和文件边界

新增：

- `internal/billing/types.go`：ledger、price、order-independent billing types
- `internal/billing/pricing.go`：从 usage 和 price snapshot 计算整数成本
- `internal/billing/service.go`：balance、checkpoints、settlement、referral credits、admin adjustments
- `internal/billing/service_test.go`：ledger invariant 和 idempotency 测试
- `internal/admission/service.go`：balance/capacity/abuse admission checks 和 runtime counters
- `internal/admission/service_test.go`：capacity、reward-only、limit tests
- `internal/email/sender.go`：verification email sender interface、SMTP implementation、本地 stdout dev sender
- `internal/email/sender_test.go`：email rendering 和 secret-safe logging tests
- `internal/payments/zpay/sign.go`：7pay MD5 签名和验签
- `internal/payments/zpay/client.go`：`mapi.php` 创建订单和 `api.php?act=order` 主动查询
- `internal/payments/zpay/sign_test.go`：标准签名/验签测试
- `internal/server/customer_auth.go`：注册、登录、登出、session auth
- `internal/server/customer_billing.go`：余额、订单、充值、轮询
- `internal/server/customer_keys.go`：客户 API key CRUD
- `internal/server/admin_billing.go`：admin prices、settings、orders、ledger adjustments
- `internal/domain/api_key.go`
- `internal/domain/admission.go`
- `internal/domain/billing.go`
- `internal/domain/payment.go`
- `internal/domain/session.go`
- `internal/store/sqlite_api_keys.go`
- `internal/store/sqlite_admission.go`
- `internal/store/sqlite_billing.go`
- `internal/store/sqlite_payments.go`
- `internal/store/sqlite_sessions.go`
- `web/src/routes/app/*`：客户门户
- `web/src/routes/admin-billing/*`：admin billing 页面

修改：

- `cmd/relay/main.go`：只注册 Codex driver
- `internal/config/config.go`：增加 7pay 和 session settings
- `internal/auth/auth.go`：鉴权 admin token 或 API key rows
- `internal/relay/relay.go`：接收 billing service dependency
- `internal/relay/relay_flow.go`：upstream 执行前做 admission
- `internal/relay/relay_attempt.go`：成功 usage 只结算一次
- `internal/server/server.go`：接入 billing 和 payment services
- `internal/server/server_routes.go`：public auth、customer、payment、admin billing routes
- `internal/server/compat_openai_chat.go`：Codex-only compat 或显式 unsupported errors
- `internal/store/schema.sql`：显式 schema
- `internal/store/sqlite_schema.go`：显式 migration
- `web/src/routes/users/*`：admin user view 指向 customer/API key 模型
- `web/src/routes/accounts/*`：Codex-only 文案和过滤

编译通过后删除或退役：

- Claude/Gemini provider registration
- Claude/Gemini onboarding UI paths
- Claude/Gemini compat routing
- 只断言 Claude/Gemini 支持存在的测试

如果测试仍需要 compile scaffolding，provider driver 文件可以暂时保留。但 runtime route、model catalog、UI entry 不允许暴露非 Codex 产品。

## 实现任务

### Task 1: 锁定 Codex-Only Runtime 边界

**Files：**

- Modify: `cmd/relay/main.go`
- Modify: `internal/server/server_routes.go`
- Modify: `internal/server/driver_views.go`
- Modify: `internal/server/compat_openai_chat.go`
- Modify: `internal/server/contract_test.go`
- Modify: `internal/server/compat_openai_chat_test.go`

- [ ] 增加失败测试，证明 `/admin/providers`、`/v1/models`、`/openai/responses`、compat routes 只暴露 Codex/OpenAI models。
- [ ] 增加失败测试，证明 Claude/Gemini add-account routes 和 compat model prefixes 被拒绝。
- [ ] 从 `cmd/relay/main.go` 移除 Claude/Gemini driver registration。
- [ ] 让 `handleListModels` 只返回 Codex models。
- [ ] 用 Codex-only resolver 替换 Claude/Gemini compat resolver。
- [ ] Run: `go test ./internal/server ./cmd/relay -count=1`
- [ ] Commit: `git commit -m "refactor: constrain relay runtime to codex"`

### Task 2: 拆分客户身份和 API Keys

**Files：**

- Create: `internal/domain/api_key.go`
- Create: `internal/domain/session.go`
- Create: `internal/email/sender.go`
- Create: `internal/email/sender_test.go`
- Create: `internal/store/sqlite_api_keys.go`
- Create: `internal/store/sqlite_sessions.go`
- Create: `internal/server/customer_auth.go`
- Create: `internal/server/customer_keys.go`
- Modify: `internal/domain/user.go`
- Modify: `internal/auth/auth.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite_users.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`
- Modify: `internal/server/server_routes.go`
- Test: `internal/store/sqlite_migration_test.go`

- [ ] 增加 migration tests，覆盖新的 `users`、`api_keys`、`web_sessions`、`email_verifications` schema。
- [ ] 增加 auth tests，证明 API key lookup 使用 deterministic SHA-256 token hashes 返回 `user_id`、`api_key_id`、surface、status。
- [ ] 增加 customer auth handler tests，覆盖 register、login、logout、session lookup。
- [ ] 实现客户登录的 bcrypt password hashing。
- [ ] 实现 `POST /api/auth/register`，创建 user，并在有有效 invite code 时履约受邀方注册奖励。
- [ ] 把 relay token hashes 从 `users` 移到 `api_keys`。
- [ ] 保持 admin `API_TOKEN` 验证不变，用于 operator routes。
- [ ] 生成 API key 时只返回一次 plaintext，存储 SHA-256 hash 和 prefix。
- [ ] active 登录客户无需邮箱验证即可创建 API key。
- [ ] Run: `go test ./internal/auth ./internal/server ./internal/store -count=1`
- [ ] Commit: `git commit -m "feat: split customers and api keys"`

### Task 3: 增加 Billing Ledger 和 Pricing

**Files：**

- Create: `internal/domain/billing.go`
- Create: `internal/billing/types.go`
- Create: `internal/billing/pricing.go`
- Create: `internal/billing/service.go`
- Create: `internal/billing/service_test.go`
- Create: `internal/store/sqlite_billing.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`
- Test: `internal/store/sqlite_migration_test.go`

- [ ] 增加测试，证明余额是 latest checkpoint 加 checkpoint 之后的 ledger rows。
- [ ] 增加测试，证明删除 checkpoint 后可以从 ledger rows 重算出同样余额。
- [ ] 增加测试，证明 settlement 以 request id 做幂等。
- [ ] 增加测试，证明 price updates 不改变已有 debit snapshot JSON。
- [ ] 从 `driver.Usage` 实现整数 usage pricing。
- [ ] 实现 `Credit`、`DebitUsage`、`Balance`、`WriteCheckpoint`、`AdminAdjust`。
- [ ] 增加 `billable_requests` persistence helpers，覆盖 `in_progress`、`usage_observed`、`settled`、`settlement_failed`。
- [ ] 在 migration 或显式 admin bootstrap 中 seed 默认 Codex model prices。
- [ ] Run: `go test ./internal/billing ./internal/store -count=1`
- [ ] Commit: `git commit -m "feat: add billing ledger"`

### Task 4: 增加容量和反滥用 Admission Gates

**Files：**

- Create: `internal/domain/admission.go`
- Create: `internal/admission/service.go`
- Create: `internal/admission/service_test.go`
- Create: `internal/store/sqlite_admission.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`

- [ ] 增加测试，证明 admission 要求正余额、配置容量、邮箱已验证。
- [ ] 增加测试，证明 reward-only 用户最多只能有 1 个 concurrent request。
- [ ] 增加测试，证明 paid users 使用 admin 配置的 per-user 和 per-key limits。
- [ ] 增加测试，证明 requests-per-minute limit 会在 upstream account selection 前拒绝超量调用。
- [ ] 实现 `admission_limits` storage 和 default rows。
- [ ] 实现按 user 和 API key 计数的 runtime counters。
- [ ] 记录 admission rejects，包含 `user_id`、`api_key_id`、`reason`、`balance_micros` 和 limit fields。
- [ ] Run: `go test ./internal/admission ./internal/server ./internal/store -count=1`
- [ ] Commit: `git commit -m "feat: add relay admission limits"`

Admission law：

```text
allow = balance > min_balance AND under_global_limit AND under_user_limit AND under_key_limit
```

### Task 5: 把 Billing 接入 Relay Admission 和 Settlement

**Files：**

- Modify: `internal/relay/relay.go`
- Modify: `internal/relay/relay_flow.go`
- Modify: `internal/relay/relay_attempt.go`
- Modify: `internal/relay/relay_attempt_test.go`
- Modify: `internal/admission/service.go`
- Modify: `internal/billing/service.go`
- Modify: `internal/requestlog/artifact.go`
- Modify: `internal/domain/log.go`
- Modify: `internal/store/sqlite_logs.go`

- [ ] 增加失败 relay tests，覆盖余额或容量不满足时的 admission rejection。
- [ ] 增加失败 relay tests，覆盖余额 `> 0` 且 capacity available 时允许请求，并按真实 usage debit。
- [ ] 增加失败 relay tests，证明同一个 request id 两次 settlement 只创建一条 ledger debit。
- [ ] 增加失败 stream tests，证明 final usage 在转发 final completion event 前已持久化。
- [ ] 增加失败 disconnect tests，证明 stream abort 但已观察到 usage 时仍会结算；没有 usage 时标记 `aborted_no_usage`。
- [ ] 增加失败 startup reconciliation tests，证明没有 `ledger_id` 的 `usage_observed` rows 会被结算。
- [ ] 给 request-log 增加 correlation fields：`request_id` 和 `api_key_id`。
- [ ] Admission 通过后、upstream request 开始前，插入 `billable_requests.in_progress`。
- [ ] 在 authentication 之后、account selection 之前做 admission。
- [ ] Non-stream request 在 usage 解析后、写 response body 前结算。
- [ ] Stream request 在观察到 final usage 时结算；转发 final completion 给客户端前先持久化 usage。
- [ ] Settlement 失败时，持久化 `settlement_failed` 和 causal error，并记录 `request_id`、`user_id`、`api_key_id`、`model`、usage。
- [ ] Run: `go test ./internal/relay ./internal/admission ./internal/billing ./internal/requestlog ./internal/store -count=1`
- [ ] Commit: `git commit -m "feat: bill relay usage"`

MVP 接受的 failure model：

- 如果用户余额为正并发起并发请求，所有已 admit 的请求都可以结算，并可能把余额打到更深的负数。这是首版明确接受的行为。
- Reward-only 用户限制为 1 个 concurrent request，所以接受的 concurrent overrun 只适用于邮箱已验证且在 admin 配置付费限制内的用户。
- 如果 stream 在观察到任何 final usage 前断开，系统记录 `aborted_no_usage`；MVP 不发明 token estimate。
- 如果 usage 已观察到但 SQLite settlement 失败，持久的 `billable_requests` 状态允许 startup reconciliation。

`aborted_no_usage` 人工审查的移除条件：

- 只有在真实 abort-without-usage 数量让人工审查不可接受后，才增加 provider-specific prompt-token estimation。

### Task 6: 增加 7pay 充值

**Files：**

- Create: `internal/domain/payment.go`
- Create: `internal/payments/zpay/sign.go`
- Create: `internal/payments/zpay/client.go`
- Create: `internal/payments/zpay/sign_test.go`
- Create: `internal/store/sqlite_payments.go`
- Create: `internal/server/customer_billing.go`
- Modify: `internal/config/config.go`
- Modify: `internal/server/server_routes.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`

- [ ] 增加签名测试，覆盖过滤 `sign`、`sign_type`、空值、ASCII key 排序、value 原样拼接、小写 MD5。
- [ ] 增加创建订单测试，证明调用 zpay 前已插入本地 pending order。
- [ ] 增加 webhook 测试，证明 bad signature 返回 `fail`。
- [ ] 增加 webhook 测试，证明非 `TRADE_SUCCESS` 返回 `success` 且不履约。
- [ ] 增加 webhook 测试，证明金额不匹配返回 `fail` 且不写 ledger credit。
- [ ] 增加 webhook 幂等测试，证明重复 callback 不会重复 credit。
- [ ] 增加主动查询轮询测试，证明 pending order 可以通过 `api.php?act=order` 履约。
- [ ] 增加 config keys：`ZPAY_PID`、`ZPAY_KEY`、`ZPAY_CID`、`SITE_URL`。
- [ ] 实现 `POST /api/payments/orders`。
- [ ] 实现 `GET /api/payments/orders/{out_trade_no}`，pending 时带节流主动查询。
- [ ] 实现 `GET|POST /api/payments/zpay/notify`。
- [ ] 所有日志和 stored event payload 都必须脱敏 `ZPAY_KEY`。
- [ ] Run: `go test ./internal/payments/zpay ./internal/server ./internal/store -count=1`
- [ ] Commit: `git commit -m "feat: add zpay recharge"`

7pay 履约法则：

```text
payment_order(pending) + valid paid gateway evidence + amount match
  -> payment_order(paid)
  -> billing_ledger(payment_credit, source_id=out_trade_no)
```

### Task 7: 增加注册和付费触发的邀请奖励

**Files：**

- Modify: `internal/domain/billing.go`
- Modify: `internal/billing/service.go`
- Modify: `internal/billing/service_test.go`
- Modify: `internal/server/customer_auth.go`
- Modify: `internal/store/sqlite_billing.go`
- Modify: `internal/store/sqlite_users.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/sqlite_schema.go`

- [ ] 增加测试，证明使用有效 invite code 注册后 invitee 立即获得 credit。
- [ ] 增加测试，证明 inviter 必须等 invitee 成功付费后才获得 credit。
- [ ] 增加测试，证明无效 invite code 拒绝注册。
- [ ] 增加测试，证明重复 signup/retry 不会重复发 referral credits。
- [ ] 增加 `referrals` table 和 referral settings。
- [ ] 给每个用户生成稳定 referral code。
- [ ] 在注册履约路径插入 invitee referral ledger credit。
- [ ] 在支付履约路径用 invitee-scoped idempotency key 插入 inviter referral ledger credit。
- [ ] 在 customer `/api/me` 暴露 referral summary。
- [ ] Run: `go test ./internal/billing ./internal/server ./internal/store -count=1`
- [ ] Commit: `git commit -m "feat: add referral signup credits"`

Referral law：

```text
register(user_with_invite_code)
  -> users.referred_by_user_id = inviter
  -> referrals(inviter, invitee)
  -> ledger += invitee signup bonus

paid_order(invitee)
  -> ledger += inviter paid referral bonus
```

### Task 8: 增加客户门户

**Files：**

- Create: `web/src/routes/app/+layout.svelte`
- Create: `web/src/routes/app/login/+page.svelte`
- Create: `web/src/routes/app/register/+page.svelte`
- Create: `web/src/routes/app/dashboard/+page.svelte`
- Create: `web/src/routes/app/keys/+page.svelte`
- Create: `web/src/routes/app/billing/+page.svelte`
- Create: `web/src/routes/app/referrals/+page.svelte`
- Modify: `web/src/routes/+page.ts`
- Modify: `web/src/routes/+layout.svelte`
- Modify: `web/src/lib/admin-types.ts`

- [ ] 构建客户登录和注册表单。
- [ ] 构建基于 ledger-derived balance 的余额面板。
- [ ] 构建充值表单：创建 7pay QR order，并每 3 秒轮询 order status。
- [ ] 构建 API key list/create/revoke UI。
- [ ] 构建 usage table，数据来自 request logs 和 ledger debits。
- [ ] 构建 referral link 和 bonus history。
- [ ] Admin token login 和 customer login 必须在视觉和路由上分离。
- [ ] Run: `cd web && npm run build`
- [ ] Commit: `git commit -m "feat: add customer portal"`

UI rule：

- Customer pages 不暴露上游账号身份、proxy cells、provider account management。
- Admin pages 不要求输入 customer passwords。

### Task 9: 增加 Admin Billing Controls

**Files：**

- Create: `internal/server/admin_billing.go`
- Create: `web/src/routes/admin-billing/+page.svelte`
- Create: `web/src/routes/admin-billing/orders/+page.svelte`
- Create: `web/src/routes/admin-billing/users/[id]/+page.svelte`
- Modify: `internal/server/server_routes.go`
- Modify: `web/src/routes/+layout.svelte`
- Modify: `web/src/routes/users/+page.svelte`
- Modify: `web/src/routes/users/[id]/+page.svelte`

- [ ] 增加 billing settings 的 admin endpoints。
- [ ] 增加 admission limits 的 admin endpoints。
- [ ] 增加 model prices 的 admin endpoints。
- [ ] 增加 payment order lookup 的 admin endpoints。
- [ ] 增加 user ledger 和 manual adjustments 的 admin endpoints。
- [ ] 增加测试，证明 manual adjustments 是带 idempotency keys 的 ledger rows。
- [ ] 增加测试，证明 price changes 只影响未来 debits。
- [ ] 构建 settings、admission limits、prices、orders、user balance、ledger 的 admin pages。
- [ ] Run: `go test ./internal/server ./internal/billing -count=1`
- [ ] Run: `cd web && npm run build`
- [ ] Commit: `git commit -m "feat: add admin billing controls"`

### Task 10: 退役 Claude/Gemini 产品残留

**Files：**

- Modify or delete: `internal/driver/claude*.go`
- Modify or delete: `internal/driver/gemini*.go`
- Modify: `internal/domain/account.go`
- Modify: `internal/server/admin_oauth*.go`
- Modify: `internal/server/compat_openai_*`
- Modify: `web/src/routes/add-account/[provider]/+page.svelte`
- Modify: `README.md`

- [ ] 删除告诉用户配置 Claude Code 或 Gemini 的公开文档。
- [ ] 删除除 Codex 外的 UI provider choices。
- [ ] 删除唯一断言 Claude/Gemini 被支持的测试。
- [ ] 只有在 Codex 仍然使用且不暴露非 Codex 产品时，才保留 generic provider abstractions。
- [ ] Run: `go test ./...`
- [ ] Run: `cd web && npm run build`
- [ ] Commit: `git commit -m "refactor: retire non-codex product paths"`

边界：

- `driver.Driver` 可以继续保持 generic。
- Server routes、model catalogs、customer UI、docs 不允许宣传 Claude/Gemini。

### Task 11: 端到端验证和发布文档

**Files：**

- Create: `scripts/smoke-billing.sh`
- Create: `docs/codex-openai-saas-relay.md`
- Modify: `README.md`

- [ ] 增加 smoke script：创建用户、授予 credit、创建 API key、用 fake upstream 调 `/openai/responses`，并验证只有一条 debit ledger row。
- [ ] 增加 balance `<= 0` rejection 的 smoke script mode。
- [ ] 增加 reward-only concurrent request rejection 的 smoke script mode。
- [ ] 增加 checkpointed balance 等于 full ledger recomputation 的 smoke script mode。
- [ ] 增加 live 7pay payment smoke notes，因为 gateway credentials 依赖环境。
- [ ] 记录环境变量：`API_TOKEN`、`ENCRYPTION_KEY`、`ZPAY_PID`、`ZPAY_KEY`、`ZPAY_CID`、`SITE_URL`、可选 SMTP settings、session settings。
- [ ] 记录 Codex/OpenAI-compatible callers 的 public client setup。
- [ ] Run: `go test ./...`
- [ ] Run: `cd web && npm run build`
- [ ] Run: `bash scripts/smoke-billing.sh`
- [ ] Commit: `git commit -m "docs: document codex saas relay"`

## 验证矩阵

合并本分支前必须通过：

- `go test ./...`
- `cd web && npm run build`
- `bash scripts/smoke-billing.sh`
- 7pay sign tests 使用 deterministic fixtures 通过
- webhook duplicate delivery test 证明只产生一条 ledger credit
- relay duplicate settlement test 证明只产生一条 ledger debit
- relay stream usage-observed reconciliation test 通过
- reward-only concurrent request rejection test 通过
- checkpointed balance 等于 full ledger recomputation
- balance `<= 0` rejection test 通过
- concurrent positive-balance requests 作为 MVP 接受行为已写入文档

## 明确非目标

- 本阶段不做月订阅。
- 本阶段不做 prepaid reservation 或 per-request hold。
- 本阶段不保证付费用户“只允许最后一次负余额请求”；付费用户 overrun 由配置化 admission limits 约束。
- 不提供 multi-provider public product surface。
- 不把 provider-specific billing logic 放进 `pool`、`relay` 或 `server` shared core。
- 不增加可变 `users.balance` column。允许 bounded balance checkpoints 作为派生 projection。
- 本阶段不做自动退款流；admin adjustment ledger rows 是人工冲正路径。

## 本计划固定下来的决定

- 使用一个主计划文档。
- `/openai/responses` 是 canonical。
- 保留一个小的 OpenAI-compatible surface，但它只是 projection，不是第二条执行路径。
- 内部使用 USD-equivalent integer credits。
- 创建 payment order 时快照 RMB-to-USD-equivalent rate。
- Admission 包含余额、容量、反滥用闸门。
- API keys 使用 deterministic SHA-256 hashes；客户密码使用 bcrypt。
- Balance reads 使用 ledger checkpoints，避免 hot-path 无界 SUM 查询。
- Stream billing 在 final stream completion 转发前持久化 usage，并通过 startup reconciliation 结算已观察但未结算的 usage。
- 允许 admitted request 在 postpaid settlement 后把余额扣成负数。
- MVP 接受配置化 admission limits 内的付费用户并发超花。
- 受邀方注册成功后获得 reward credits；邀请方等受邀方首次成功付费后获得 reward credits。
