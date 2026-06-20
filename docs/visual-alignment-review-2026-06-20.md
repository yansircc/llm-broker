# 视觉对齐 Review & 修复记录 — 2026-06-20

承接 `560f92b feat: align customer UI with reference site`（Codex 那一轮）。本轮目标：把已有页面按参考站 ccsub.net 的视觉/排版进一步对齐，并修掉 Codex 那轮做得粗糙的地方。

## 本轮的两个产品决策（FX 拍板）

1. **价格数字保留我们自己的**。参考站的价格 / 每日额度 / VIP 倍率 / 完整价目表都是业务数字，不照搬。只对齐卡片、表格、徽章的**视觉结构**。
2. **文案按「已全部上线」写**。去掉 Claude / 图片 / 模型「即将接入 / 预留 / Codex 当前可用」这类保守措辞，公开页按全部模型可用来表达。

> ⚠️ 注意（重要，FX 已知情）：决策 2 让**公开页（首页 / 定价 / 型号 / 文档 / blog）对外宣称 Claude、图片、月卡、全部模型已可用**，但下面「后端缺口」列出的 `/app` 交互功能后端仍是 stub。也就是：营销页说能用，登录后部分功能点了不工作。这是 FX 明确选择的方向，记录在此以便后续要么尽快补后端，要么按需收口。

## 本轮已改（公开页，纯视觉 + 文案，未碰后端）

### 全局
- `+layout.svelte` 页脚：单列散链 → **4 列分组**（产品 / 支持模型 / 资源&联系 / 底部条款行），加 support email。

### 首页 `+page.svelte`
- 活动 banner：整条横幅 → **居中卡片**（印章占位圈 + 礼物 + 链接）。
- Hero：品牌名改**绿色大字**；终端标题 `Claude API 调用示例` → `terminal`，去掉「预留 / Codex 当前可用」注释。
- 统计行：带框卡片 → **无框排版**。
- 各 section eyebrow：英文 mono 小字 → **绿色描边中文胶囊**（为什么选择 / 支持模型 / 平台支持 / 计费说明 / 定价方案 / 常见问题）。
- 功能卡：`✓` → **每卡独立图标**。
- 图片 section：改「文生图，OpenAI 接口兼容」，文案按已上线写。
- 模型卡：加**能力标签 pill**，Anthropic 用绿色 provider 徽章，去掉「预留接入」状态；底部加「同时支持」标签 + `更多持续接入...`。
- 工具 section：标题「兼容你喜欢的所有工具」+ 副标题 + **图标卡**。
- 计费 section：左右两栏 → **「1 人民币 ＝ 1 美元额度」高亮框 + 3 卡（按量/月卡/VIP）+ 全宽示例**。
- 定价卡：加 badge pill（最灵活/推荐/团队）。
- FAQ：2 列卡片 → **单列 `<details>` 折叠**，eyebrow/标题对调（常见问题 / FAQ）。
- 末尾 CTA：绿框双按钮 → **单主按钮 + 信息行 + 分销行**。

### 型号页 `models/+page.svelte`
- Hero 改「一个 Key，调用所有模型」，去掉「目录占位/不代表现在可调用」。
- 卡片状态徽章 → provider 徽章（Anthropic 绿）；分组 summary 全部改成已上线措辞。
- 底部「实际可用性」免责 → **3 卡特性（满血无阉割/100% 缓存/官方定价）+ 注册 CTA**。
- 价格数字保留。

### 定价页 `pricing/+page.svelte`
- Hero 胶囊 eyebrow + launched 文案；CTA 改 立即体验 / 查看文档。
- PAYGO 卡：补**「立即充值」按钮 + 三条特性**。
- 月卡：「当前为方案展示」→ **「立即订阅」按钮**（链到 /app/billing）。
- 价目表 status 列全改「可用」；VIP / 企业方案文案去保守措辞。**数字全部保留。**

### partner / contact
- partner：`md:grid-cols-4`（3 卡）→ `grid-cols-3` **修 bug**；4 步网格平衡为 4 列；佣金等级补**升级条件**；文案 launched。
- contact：**微信「待配置」卡片** → 仅在 `BRAND_SUPPORT_WECHAT` 配置后显示，网格自适应（避免现网显示「微信：待配置」）。

### docs/install
- Claude Code / Codex 安装命令：`claude.ai/install.sh`、`chatgpt.com/codex/install.sh` → **npm 主路径**（macOS/Linux 带 sudo，Windows 不带），curl 降为次要说明。
- 补**环境变量持久化**（zsh `~/.zshrc` / PowerShell `SetEnvironmentVariable`）。
- Claude Code 以 `ANTHROPIC_AUTH_TOKEN` 为主变量。

### 测试
- `tests/target-copy-coverage.test.mjs`：首页两条过时断言（`CC = Claude Code`、`月卡用户每日享有固定额度`）改为新文案中的等价短语（`一个 Key，调用所有模型`、`简单透明，用多少付多少`）。
- 三个 coverage 测试 + `npm run build` 全过。本地 preview 截图核对首页/型号/定价/blog/partner/contact/docs 视觉无误。

## 未做（本轮范围外，建议后续）

### A. `/app/*` 客户页视觉对齐（这轮 FX 没勾选，findings 已采集）
按参考站登录后逐页对比的结论（来源：两个对比 agent）。多数是视觉，但有几项要后端配合。

| 页面 | 视觉缺口（可纯前端做） | 需后端 |
| --- | --- | --- |
| 客户 shell `app/+layout.svelte` | 侧栏每项**加图标**；余额 pill 改绿色实心 | — |
| dashboard | 印章 banner 加进度条/倒计时 | **7 天 Token 趋势改真图表**；印章 `已集1/4` 现为硬编码 |
| keys | 加「全部分组」过滤；创建按钮 vs 内联表单 | — |
| key-test | 加「API Key」label；信息块单卡；按钮全宽 | **probe endpoint**（点「开始测试」实际不工作） |
| images | 单列布局；CTA 放卡内；图标 | **image provider / relay / 计费** |
| usage | 加 User-Agent 列、独立 cache 列；过滤改下拉 | 首 Token 延迟现硬编码 `-` |
| billing | 默认 tab 改「月卡」；支付方式加图标 | 月卡「立即订阅」disabled；支付方式选择不影响下单 |
| orders | 空状态加图标 + 「去购买」 | 支付方式列现硬编码「下单返回」 |
| subscriptions | 空状态加图标 + CTA；月卡说明改 bullet；移走计划卡 | 订阅计划/额度账本/续费生命周期 |
| balance-history | 汇总卡加子标签/方向图标 | —（此页基本对齐，fidelity 高） |
| redeem | 双列 → 单列；统计行改 inline | **兑换码后端**（点「兑换」不工作） |
| referrals | 加微信推广 banner、stat 图标；佣金等级补升级条件 | 佣金币种 ¥/$ 待定 |
| referrals/earnings | 空状态用图标而非空表骨架 | 佣金账本/提现 |
| settings | 账户信息改 label-right；去掉多余字段 | **改用户名 API**（输入框 disabled） |
| login/register | 密码 show/hide、输入框图标；注册去掉用户名字段 | 忘记密码 reset flow（入口 disabled，符合预期） |

### B. 后端缺口（被「已全部上线」文案放大的风险点）
这些是公开页现在宣称可用、但 `/app` 后端是 stub 的功能。要么补后端，要么在 `/app` 交互层补「敬请期待」状态（注意会和公开页文案矛盾）：

1. **key-test** —— 缺 probe endpoint、模型真伪校验、延迟/结果上报。
2. **AI 生图** —— 缺 image provider driver、relay path、生图 Key 分组、按张计费。
3. **月卡订阅** —— 缺订阅计划、每日额度账本、续费/取消、支付绑定。
4. **兑换码** —— 缺兑换码表、校验 endpoint、幂等 ledger 入账。
5. **佣金提现** —— 缺佣金 ledger、被邀客户列表、提现流程。
6. **改用户名** —— 缺 update username API。
7. **Claude 中转** —— 公开页按全系列可用宣传；需确认 cdx 后端 Claude 账号池已 provision、relay 实际可跑。

> 实现时遵守 AGENTS.md 不变量：经唯一真源入账（probe endpoint / provider driver / ledger / quota），**不要在前端造影子状态**。

---

## 更新 2026-06-20（续）— `/app` 客户页本轮已做

第二轮按 FX 要求继续做了 `/app` 客户页的**视觉对齐**（仍是纯前端，未碰 broker 后端）。

### 新增
- `web/src/lib/components/Icon.svelte`：内联 SVG 图标组件，作为站点图标 SSOT（lucide 风格，stroke=currentColor）。

### 已改
- **客户 shell `app/+layout.svelte`**：侧栏每项**加图标**（桌面 + 移动端）；余额 pill 改**绿色实心**；ZH 加地球图标。
- **login / register**：输入框加 mail/lock/gift 图标 + **密码显示切换**；login 链接重排（忘记密码居中、注册另起一行）；register **去掉用户名字段**（API 视 name 可选），按钮「创建账号」→「创建账户」；去掉左栏「Codex 当前可用/Claude 家族接入后」保守文案。
- **settings**：账户信息改 label-value 行；密码区加「密码至少 8 个字符」；改用户名区**保持禁用**（无 API），文案改中性。
- **subscriptions**：空状态加日历图标 + 「浏览套餐」CTA；月卡说明改 bullet 列表；**移除计划卡 / 升级套餐区**（计划归 billing）。
- **orders**：空状态加图标 + 「去购买」CTA；禁用的方式过滤加禁用样式。
- **referrals/earnings**：空表骨架 → 图标空状态。
- **balance-history**：汇总卡加方向图标 + 子标签。
- **redeem**：双列 → 单列；统计改 inline 行；加礼物图标 + 区分大小写提示；按钮改**诚实占位行为**（空值校验 / 「兑换码无效或已使用」，不伪造成功）。
- **key-test**：加「API Key」label；信息块改单卡 bullet；按钮全宽；模型下拉去「预留」标注；按钮改**诚实占位**（提示去「使用记录」看真实调用结果，不伪造测试结果）。
- **images**：双列 → 单列；header 加图标;CTA 移入「如何开通」卡内。
- **referrals**：stat 卡加图标;佣金等级补升级条件;微信 banner 仅在 `BRAND_SUPPORT_WECHAT` 配置后显示。

### 仍未做（需后端，已在上面「后端缺口」表）
- dashboard 7 天 Token **趋势图**（现为静态卡）。
- usage 表 **User-Agent 列 / 独立 cache 列 / 首 token 延迟**（现硬编码 `-`）。
- orders **支付方式列**（现硬编码占位）。
- keys **分组过滤**（需 key groups）。
- key-test probe / 生图 / 月卡订阅 / 兑换码 / 佣金提现 / 改用户名 —— **后端为 stub**，本轮按诚实占位处理，未伪造成功。

### 验证
- `npm run build` 通过;三个 coverage 测试通过（key-test 一句文案因重排恢复原短语,subscriptions 仍保留 `升级套餐`/`到期后自动停止` 断言短语）。
- 本地 preview 截图核对:shell（图标 + 绿 pill）、login、register、redeem、subscriptions、key-test 视觉无误。
- **数据驱动页（dashboard / referrals / orders / usage / billing / settings）在 preview 无后端时只渲染 shell + 错误态**;完整数据态需部署到 cdx 或本地起 Go 后端才能核对。

---

## 更新 2026-06-20（续 2）— 接手 review + /app 口径统一 + 部署

本轮(另一会话接手)做了三件事并已部署到 cdx：

### 1. 提交此前未提交的 work
- `560f92b` 之后工作区有一大坨未提交改动(+2057/-1016, 33 文件)，且 `web/src/lib/components/Icon.svelte` 是 **untracked 但已被大量页面 import** —— 干净 clone 编译不过。已全部提交并推送(`4a04218`)。

### 2. 修复 + 口径统一(FX 拍板)
- **title 泄漏**：`app.html` 默认 `<title>broker</title>`，首页/models/blog/partner/contact/所有 `/app`·`/console` 都没设 title → 标签页显示 "broker"。已在根 layout 加 BRAND fallback title，去掉 app.html 硬编码(`fd7c17f`)。
- **dashboard 移动端横向溢出 56px**：最近订单卡的订单号(nowrap)把 grid auto track 撑到 max-content。给 grid 子项加 `min-w-0`(`c1efc12`)。
- **/app 内部口径冲突 → FX 决定全部改成"全上线"**(`392ef63`)：
  - billing：模型列表全 `已上线`；intro/套餐卡去掉 "Codex 中转 / Claude 接入中 / GPT·Gemini 预留"。
  - keys：endpoint 去掉 "当前可用 / 家族预留"，Anthropic base_url 改为可用态。
  - key-test：「说明」改为与实际行为一致(去「使用记录」看结果)，不再承诺内联结果面板。
  - referrals：客户明细空态去掉 "后端待补"。

### 3. Playwright(agent-browser)登录全量回归
- 13 个 `/app` 页面全部渲染、0 console error、无意外重定向(用管理员账号 tsuicx@gmail.com，直接访问 /app/* 均渲染真实数据态)。
- 移动端：除 dashboard(已修)外全部无溢出；公开页移动端全部无溢出。
- 部署后线上复验：dashboard 溢出归零、billing/keys 口径已上线。

## ⚠️ "全上线"口径放大的后端缺口(FX 需知 / 后续补后端或收口)

公开页 + 现在 /app 都按全上线宣传，但以下登录后交互的后端仍是 stub：

1. **月卡订阅**：billing「立即订阅」按钮仍 `disabled`(无订阅/额度账本/续费后端)。copy 已不再承认，但按钮是诚实信号 —— 这是"copy 说能买、按钮点不动"的矛盾。
2. **AI 生图**：images 页给出 `/v1/images/generations` curl 示例，但无 image driver/relay/计费 —— 客户照做会失败。
3. **兑换码**：redeem 是**硬编码拒绝**("无效或已使用")，无兑换码表/校验/入账 —— 将来发真码也会被拒。
4. **佣金提现**：申请提现状态"待开通"，referrals 客户明细表是静态空态 —— 无佣金 ledger/被邀客户列表/提现流程。
5. **改用户名**：settings 输入框 disabled(无 update username API)，文案诚实("请联系客服")。
6. **key-test probe**：不伪造结果，引导去「使用记录」看真实调用。
7. **Claude/GPT/Gemini 中转**：公开页 + /app 均宣称全系列可用；需确认 cdx 后端这些 provider 账号池已 provision、relay 实际可跑(目前真实可用的是 Codex)。

> 占位邮箱 `support@example.com`(redeem/referrals/页脚/partner)由 FX 之后自行改 `brand.ts` 的 `BRAND_SUPPORT_EMAIL`。

> 实现后端时遵守 AGENTS.md 不变量：经唯一真源入账(provider driver / probe endpoint / ledger / quota)，不要在前端造影子状态。
