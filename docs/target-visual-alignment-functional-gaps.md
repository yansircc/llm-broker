# UI 已对齐 / 后端待补齐 (functional gaps)

> 最后更新 2026-06-20。这是**唯一权威**的「界面已经按参考站对齐、但功能后端还没补」的清单。
> 逐页视觉改动的明细见 `docs/visual-alignment-review-2026-06-20.md`（change log），本文件只跟踪**还需要写代码（多为后端）的缺口**。

## 路由不变量（保持不变）

- 公开 UI = `/`，客户 UI = `/app/*`，管理 UI = `/console/*`
- 参考站的 `/dashboard`、`/keys`、`/purchase`、`/zh/*` 一律映射到现有 `/app/*`，不改运行时路由契约

## ⚠️ 头号风险：公开页已按「全部上线」宣传，但部分功能后端是 stub

按 FX 决策，公开页（首页 / 定价 / 型号 / 文档 / blog）现在对外**宣称 Claude、AI 生图、月卡订阅、兑换码、全部模型均已可用**。但下表中标 ❌ 的功能后端尚不存在——也就是营销页说能用、用户登录后点进去不工作。

两种收口方式（二选一，别一直挂着）：
1. **补后端**（推荐，下面有清单），让宣传成真；
2. 暂时在 `/app` 交互层把对应控件改成「敬请期待」——但这会和公开页文案矛盾，只是过渡。

本轮 `/app` 已对 stub 控件做了**诚实占位**（不伪造成功）：兑换 → 「兑换码无效或已使用」；Key 测试 → 提示去「使用记录」看真实调用结果。

**另需 FX 确认**：公开页按 Claude 全系列可用宣传——请确认 cdx 后端的 **Claude 账号池已 provision、relay 实际可跑**，否则首页最显眼的卖点会翻车。

## 现有可用的客户端 endpoint（已接后端，UI 真实可用）

`/api/me`、`/api/me/password`、`/api/keys`(CRUD)、`/api/billing/summary`、`/api/billing/ledger`、`/api/payments/create`、`/api/payments/orders`(+refresh)、`/api/payments/notify`、`/api/referrals`、`/api/usage`、`/api/auth/{register,login,logout}`。

> 即：**充值（按量）、API Key 管理、用量记录、余额流水、订单、分销概览、注册登录**这些是真的能用的。

## 缺口清单（UI 已对齐，需补后端）

| 功能 | UI 状态 | 后端缺口 | 优先级 |
| --- | --- | --- | --- |
| **Claude 中转** | ✅ 公开页全系列宣传 | ❓ 确认 Claude 账号池已 provision + relay 可跑（核心，决定首页可信度） | P0 |
| **AI 生图** `/app/images` | ✅ 视觉入口、调用示例、定价 | ❌ image provider driver、relay path、生图 Key 分组、按张计费 | P1 |
| **月卡订阅** `/app/subscriptions` `/app/billing` | ✅ 套餐卡、「立即订阅」按钮 | ❌ 订阅计划、每日额度账本、续费/取消生命周期、支付绑定 | P1 |
| **兑换码** `/app/redeem` | ✅ 表单 + 诚实占位 | ❌ 兑换码表、校验 endpoint、幂等 ledger 入账 | P2 |
| **Key 测试** `/app/key-test` | ✅ 表单 + 诚实占位 | ❌ API key probe endpoint、模型真伪校验、延迟/结果上报 | P2 |
| **佣金提现** `/app/referrals(/earnings)` | ✅ 等级、提现按钮(禁用)、佣金明细空态 | ❌ 佣金 ledger、被邀客户列表、提现流程/结算状态 | P2 |
| **改用户名** `/app/settings` | ✅ 视觉(输入框禁用) | ❌ update username endpoint | P3 |
| **忘记密码** `/app/login` | ✅ 入口(禁用) | ❌ password reset request/token/confirm flow | P3 |

## 「数据已有、字段待补」的小缺口（非阻塞）

| 页面 | 复用的现有真源 | 还差的字段/能力 |
| --- | --- | --- |
| `/app/dashboard` | `/me` `/keys` `/billing/summary` `/usage` `/payments/orders` `/referrals` | 7 天 Token **趋势图**（现为静态卡）；印章活动状态（`已集1/4` 现硬编码）；VIP 阈值 |
| `/app/keys` | Key CRUD/状态/预算 | Key **分组**、明文回看/复制、过期时间、参考站式限速模型 |
| `/app/usage` | 请求用量日志 | **首 token 延迟**（现硬编码 `-`）、真实 **User-Agent 列**、独立 cache 列 |
| `/app/billing` | 一次性下单 | 月卡购买、支付方式选择真正影响下单、USDT 专属处理 |
| `/app/orders` | 订单列表/状态过滤 | **支付方式列**（现硬编码占位）、按方式过滤 |
| `/app/referrals` | 推广码/链接/注册数/奖励汇总 | 阶梯佣金率、被邀客户表、提现请求 |

## blog / partner / contact（本轮已视觉对齐，剩内容/配置项）

- `/blog`：现为静态卡片 + 三篇静态文章。如需持续发文，要 CMS 或 post storage。
- `/partner`、`/contact`：微信入口仅在 `BRAND_SUPPORT_WECHAT`（现 `待配置`）配置后显示。配置真实微信号即可点亮。

## 实现边界（AGENTS.md 不变量，务必遵守）

补这些后端时，每一项都要经**唯一真源**入账，**不要在前端造影子状态**：

- Key 测试结果来自 probe endpoint，不是客户端推断
- 生图走 provider driver / relay / 计费路径，不是 UI-only 请求
- 订阅是 ledger/quota 模型，不是第二个余额布尔
- 兑换码幂等写 billing ledger 行
- 佣金从 payment/referral 事实推导，不是手维护的展示总额
- provider 协议细节止于 `driver.Driver`；`pool`/`relay`/`server`/`store` 不学 provider 特例
