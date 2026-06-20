<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_DESCRIPTION, BRAND_NAME, BRAND_TAGLINE } from '$lib/brand';

	let origin = $state('https://your-domain.example');

	$effect(() => {
		if (browser) origin = window.location.origin;
	});

	const stats = [
		{ value: '1 RMB = 1 USD', label: '官方 1.4 折汇率' },
		{ value: '多线路智能调度', label: '自动故障切换' },
		{ value: '99.9% 稳定率', label: '7x24 监控' }
	];

	const features = [
		{
			icon: '🛡️',
			title: '满血不掺水',
			body: '企业级通道直连，拒绝逆向阉割和以次充好。内置一键 Key 测试功能，可验证模型、余额和连通性。'
		},
		{
			icon: '🔌',
			title: '一键接入',
			body: '兼容 OpenAI / Anthropic 格式，Claude Code、Codex、Cursor、OpenClaw 等工具按同一 Base URL / Key 即可接入。'
		},
		{
			icon: '💰',
			title: '极致性价比',
			body: '1 人民币 = 1 美元额度，按真实消耗扣费。余额长期有效，不用不扣。'
		},
		{
			icon: '📊',
			title: '稳定透明',
			body: '多线路调度、失败自动切换。每笔调用、扣费、订单和推广收益都可追溯。'
		}
	];

	const modelCards = [
		{ name: 'Claude Opus 4.8', provider: 'Anthropic', tags: ['推理', '编码', '最强', '最新'], copy: '旗舰级复杂推理与架构设计模型', input: '$5', output: '$25' },
		{ name: 'Claude Sonnet 4.6', provider: 'Anthropic', tags: ['编码', '高效', '主力'], copy: '日常主力编码模型，速度与质量均衡', input: '$3', output: '$15' },
		{ name: 'Claude Haiku 4.5', provider: 'Anthropic', tags: ['快速', '轻量'], copy: '快速轻量，适合补全、摘要和批量任务', input: '$0.8', output: '$4' },
		{ name: 'GPT-5.4', provider: 'OpenAI', tags: ['推理', '编码', '多模态'], copy: '下一代通用推理、编码与多模态能力', input: '$5', output: '$15' }
	];

	const modelList = ['GPT-4o', 'o3', 'o3-pro', 'o4-mini', 'Codex', '更多持续接入...'];

	const platforms = [
		{ icon: '🤖', name: 'Claude Code', body: 'Anthropic 官方 CLI，配置 Base URL 即可使用。' },
		{ icon: '⌘', name: 'Codex', body: 'OpenAI 编码 CLI，适合终端编码工作流。' },
		{ icon: '</>', name: 'OpenCode', body: '接入 OpenAI 兼容端点，本地代理式开发。' },
		{ icon: '🐾', name: 'OpenClaw', body: '统一 Key 管理，按项目拆分调用与账单。' },
		{ icon: '▦', name: 'Cursor', body: '配置自定义 Base URL 后复用同一余额。' },
		{ icon: '🧩', name: 'VS Code', body: '配合插件或 CLI，在编辑器内直接调用模型。' },
		{ icon: '🌊', name: 'Windsurf', body: '为 AI IDE 提供统一模型入口和账本。' },
		{ icon: '🍒', name: 'CherryStudio', body: '桌面客户端测试模型、Key 和多会话。' }
	];

	const billingHighlights = [
		{ title: '按量计费', body: '用多少扣多少，余额永不过期，不用不扣。' },
		{ title: '月卡更划算', body: '每日固定额度自动刷新，高频开发成本更低。' },
		{ title: 'VIP 等级折扣', body: '累计消费越多，调用倍率越优惠。' }
	];

	const plans = [
		{
			name: '基础',
			badge: '最灵活',
			price: '¥20 起',
			body: '适合试用、低频调用和个人小项目。',
			items: ['$20 起充额度', '余额永不过期', '支持全部模型']
		},
		{
			name: '标准版月卡',
			badge: '推荐',
			price: '¥499/月',
			body: '适合高频个人开发者和连续编码任务。',
			items: ['$50/天额度', '每日自动刷新', '支持按量余额叠加'],
			featured: true
		},
		{
			name: '团队版月卡',
			badge: '团队',
			price: '¥1,888/月',
			body: '适合小团队、工作室和批量自动化任务。',
			items: ['$200/天额度', '团队项目分 Key 管理', '更高并发']
		}
	];

	const faqGroups = [
		{
			title: '基础入门',
			items: [
				[`${BRAND_NAME} 是什么？`, `${BRAND_NAME} 是面向开发者的 AI API 中转服务，一个 Key 即可调用 Claude、GPT、o3、Gemini、Codex 等主流模型，人民币充值、国内直连，按官方定价享约 1.4 折。`],
				['国内能直接用吗？需要梯子吗？', '可以直连，无需梯子。在工具里把 Base URL 指向平台地址、填入 API Key 即可使用，无需海外账号。'],
				['支持哪些模型？', 'Claude Opus / Sonnet / Haiku 全系列，GPT-5.4 / GPT-4o、o3 / o4-mini、Gemini、Codex，以及文生图模型，并持续接入更多。'],
				['支持哪些工具和 IDE？', '兼容 OpenAI / Anthropic 格式，支持 Claude Code、Codex、Cursor、VS Code、OpenClaw、OpenCode、CherryStudio 等。']
			]
		},
		{
			title: '定价与计费',
			items: [
				['为什么价格这么便宜（1块钱=1刀）？', '余额以美元额度展示，人民币按平台配置兑换为美元额度，相当于官方约 1.4 折，我们只赚汇率差。'],
				['1 块钱大概能用多少？', '按模型而异。以 Sonnet 为例，一次普通编码对话约 $0.045，¥1 大约可对话 22 次；轻量模型更省。'],
				['实际扣费怎么计算？', '按每次调用的实际 Token 消耗（输入 / 输出 / 缓存）乘以模型单价扣减，与官方计价口径一致，调用记录可查。'],
				['月卡的「每日额度」怎么用？用不完能累积吗？', '月卡每日零点刷新固定额度（如 $50/天），当日用不完不累积，可与按量余额同时使用。'],
				['退款政策是什么？', '充值额度永不过期。如对服务有疑问，可通过支持邮箱联系我们处理。']
			]
		},
		{
			title: '功能与能力',
			items: [
				['支持图片识别吗？', '支持。GPT-4o、Gemini、Claude 等多模态模型可识别图片；文生图则用 OpenAI Images 兼容接口，新建生图分组 Key 即可。'],
				['可以创建多个 API Key 吗？', '可以按项目、工具或用途创建不同 Key，便于统计消耗和定位问题。'],
				['支持企业用户吗？', '支持。提供对公转账、开具发票和定制 API 部署，详见企业定制方案。']
			]
		},
		{
			title: '安全与稳定',
			items: [
				['会封号吗？安全吗？', '平台只展示必要的 Key 信息，敏感凭据加密保存、不在页面明文泄露；通过中转调用可降低账号风险。'],
				['服务稳定性怎么保障？', '多线路智能调度、失败自动切换，7x24 监控，订单、余额和调用日志全程可查。']
			]
		}
	];
</script>

<section class="border-b border-brand/20 bg-brand/[0.05]">
	<div class="mx-auto max-w-3xl px-5 py-6">
		<div class="rounded-2xl border border-brand/40 bg-black/30 px-6 py-5 text-center shadow-lg shadow-brand/10">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">🎁 六月限时活动 · 截止 06/30</div>
			<h2 class="mt-2 text-xl font-bold tracking-tight sm:text-2xl">集印章开盲盒，最高随机 $500 额度</h2>
			<div class="mt-4 flex items-center justify-center gap-2">
				{#each Array(4) as _, i}
					<span class="flex h-9 w-9 items-center justify-center rounded-full border border-dashed border-brand/40 text-brand/60">+</span>
				{/each}
				<span class="px-1 text-brand">→</span>
				<span class="flex h-9 w-9 items-center justify-center rounded-full border border-brand/60 bg-brand/10 text-lg">🎁</span>
			</div>
			<p class="mt-3 text-sm text-muted">登录赠 1 枚（限 1 次）· 每充 ¥49 得 1 枚 · 邀好友首充双方各 +1 枚</p>
			<a class="mt-3 inline-flex items-center gap-1 text-sm font-semibold text-brand hover:underline" href="{base}/app/register">立即领取 →</a>
		</div>
	</div>
</section>

<section class="relative overflow-hidden border-b border-line">
	<div class="absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(0,255,65,0.12),transparent_30rem)]"></div>
	<div class="relative mx-auto max-w-4xl px-5 pt-20 pb-16 text-center lg:pt-28">
		<div class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.08] px-3 py-1 font-mono text-xs text-brand">
			<span class="h-1.5 w-1.5 rounded-full bg-brand"></span>{BRAND_TAGLINE}
		</div>
		<h1 class="mt-6 text-7xl font-bold tracking-tight text-slate-100 [text-shadow:0_0_32px_rgba(0,255,65,0.45)] sm:text-8xl">{BRAND_NAME}</h1>
		<p class="mt-5 text-xl text-slate-200 sm:text-2xl">{BRAND_DESCRIPTION}</p>
		<p class="mt-3 font-mono text-sm text-faint">一个 Key 调用 Claude、GPT、o3、Codex 等全部主流模型 · 1 人民币 = 1 美元额度</p>
		<div class="mt-8 flex flex-wrap justify-center gap-4">
			<a class="rounded-md bg-brand px-8 py-3 text-base font-semibold text-black shadow-lg shadow-brand/20" href="{base}/app/register">立即体验 →</a>
			<a class="rounded-md border border-line bg-black/20 px-8 py-3 text-base hover:border-brand/50 hover:text-brand" href="{base}/docs">查看文档 ›</a>
		</div>
		<div class="mx-auto mt-12 max-w-2xl rounded-xl border border-line bg-[#050505] text-left shadow-2xl shadow-black/40">
			<div class="flex items-center gap-2 border-b border-line px-4 py-3 font-mono text-xs text-faint">
				<span class="h-2.5 w-2.5 rounded-full bg-red-400"></span>
				<span class="h-2.5 w-2.5 rounded-full bg-amber-400"></span>
				<span class="h-2.5 w-2.5 rounded-full bg-brand"></span>
				<span class="ml-2">terminal</span>
			</div>
			<div class="space-y-2 p-5 font-mono text-sm leading-6">
				<div class="text-faint"># 一键调用 Claude API</div>
				<div class="break-all text-brand">$ curl -X POST {origin}/v1/messages \</div>
				<div class="pl-4 text-faint">-H "Authorization: Bearer sk-xxx" \</div>
				<div class="pl-4 text-faint">-H "Content-Type: application/json" \</div>
				<div class="pl-4 text-faint">-d '&#123;"model":"claude-sonnet-4-6","messages":[&#123;"role":"user","content":"hello"&#125;]&#125;'</div>
				<div class="pt-2 text-brand"># 200 OK</div>
				<div class="text-faint">&#123;"content":"Hello!"&#125;</div>
			</div>
		</div>
	</div>
	<div class="relative border-t border-line">
		<div class="mx-auto grid max-w-6xl divide-y divide-line sm:grid-cols-3 sm:divide-x sm:divide-y-0">
			{#each stats as stat}
				<div class="px-5 py-6 text-center">
					<div class="font-mono text-lg font-semibold text-brand">{stat.value}</div>
					<div class="mt-1 text-sm text-faint">{stat.label}</div>
				</div>
			{/each}
		</div>
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="text-center">
			<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">为什么选择 {BRAND_NAME}</span>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">用户需要的，我们都有</h2>
			<p class="mx-auto mt-3 max-w-2xl text-muted">把 Key、额度、调用记录和工具接入放到一个控制台里，减少开发者处理账号、网络和账单的时间。</p>
		</div>
		<div class="mt-20 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
			{#each features as feature}
				<div class="rounded-lg border border-line bg-card/70 p-5">
					<div class="mb-4 flex h-10 w-10 items-center justify-center rounded-md border border-brand/30 bg-brand/[0.08] text-xl">{feature.icon}</div>
					<h3 class="text-lg font-semibold">{feature.title}</h3>
					<p class="mt-2 text-sm leading-relaxed text-muted">{feature.body}</p>
				</div>
			{/each}
		</div>
	</div>
</section>

	<section class="mx-auto max-w-6xl px-5 py-28">
		<div class="grid gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
			<div>
				<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">新功能 · AI 生图</span>
				<h2 class="mt-3 text-3xl font-bold tracking-tight">文生图，OpenAI 接口兼容</h2>
				<p class="mt-3 text-muted">用你已经在用的 OpenAI Images 接口直接生图。模型名填 gpt-image-1 / dall-e-3 开箱即用——只需新建一个「生图分组」的 Key。按张计费，从余额扣，额度永不过期。</p>
				<div class="mt-5 flex flex-wrap gap-2 text-xs text-faint">
					<span class="rounded-md border border-line bg-card px-3 py-1.5">/v1/images/generations</span>
					<span class="rounded-md border border-line bg-card px-3 py-1.5">生图 Key 分组</span>
					<span class="rounded-md border border-line bg-card px-3 py-1.5">按张计费</span>
				</div>
				<a class="mt-6 inline-flex rounded-md border border-line bg-card px-4 py-2 text-sm hover:border-brand/50" href="{base}/app/images">去创建生图 Key</a>
			</div>
			<pre class="m-0 text-sm leading-6"><code># OpenAI Images 兼容调用
curl -X POST {origin}/v1/images/generations \
	  -H "Authorization: Bearer sk-你的生图Key" \
	  -H "Content-Type: application/json" \
	  -d '&#123;
    "model": "gpt-image-1",
    "prompt": "生成一张赛博朋克风格的编码头像"
  &#125;'</code></pre>
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
			<div class="text-center">
				<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">支持模型</span>
				<h2 class="mt-3 text-3xl font-bold tracking-tight">一个 Key，调用所有模型</h2>
				<p class="mx-auto mt-3 max-w-2xl text-muted">Claude 全系列满血支持，GPT / o3 / Codex 同步可用，按官方定价计费，享 1.4 折汇率优惠。</p>
			</div>
		<div class="mt-20 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
			{#each modelCards as model}
				<div class="rounded-lg border border-line bg-card/70 p-5">
					<div class="flex items-center justify-between gap-3">
						{#if model.provider === 'Anthropic'}
							<span class="rounded-full bg-brand px-2 py-0.5 text-[11px] font-semibold text-black">{model.provider}</span>
						{:else}
							<span class="rounded-full border border-line px-2 py-0.5 font-mono text-[11px] text-faint">{model.provider}</span>
						{/if}
					</div>
					<h3 class="mt-3 text-base font-semibold">{model.name}</h3>
					<p class="mt-2 text-sm text-muted">{model.copy}</p>
					<div class="mt-3 flex flex-wrap gap-1.5">
						{#each model.tags as tag}
							<span class="rounded border border-line bg-black/20 px-1.5 py-0.5 font-mono text-[11px] text-slate-300">{tag}</span>
						{/each}
					</div>
					<div class="mt-4 space-y-1.5 border-t border-line pt-3 text-xs">
						<div class="flex justify-between"><span class="text-faint">输入</span><span class="font-mono text-brand">{model.input}/MTok</span></div>
						<div class="flex justify-between"><span class="text-faint">输出</span><span class="font-mono text-brand">{model.output}/MTok</span></div>
					</div>
				</div>
			{/each}
		</div>
		<div class="mt-6 text-center">
			<div class="mb-3 text-xs uppercase tracking-wider text-faint">同时支持</div>
			<div class="flex flex-wrap justify-center gap-2">
				{#each modelList as model}
					<span class="rounded-full border border-line bg-card px-3 py-1.5 font-mono text-xs text-slate-300">{model}</span>
				{/each}
			</div>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-28">
		<div class="text-center">
			<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">平台支持</span>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">兼容你喜欢的所有工具</h2>
			<p class="mx-auto mt-3 max-w-2xl text-muted">兼容 OpenAI / Anthropic 格式，几乎支持所有主流 AI 编程工具和 IDE。</p>
		</div>
	<div class="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
		{#each platforms as platform}
			<div class="flex flex-col items-center gap-2 rounded-lg border border-line bg-card/70 p-5 text-center">
				<div class="flex h-11 w-11 items-center justify-center rounded-md border border-brand/30 bg-brand/[0.08] text-xl">{platform.icon}</div>
				<h3 class="font-semibold">{platform.name}</h3>
				<p class="text-sm leading-relaxed text-muted">{platform.body}</p>
			</div>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="text-center">
			<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">计费说明</span>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">简单透明，用多少付多少</h2>
		</div>
		<div class="mx-auto mt-16 max-w-3xl rounded-xl border border-brand/40 bg-brand/[0.05] p-8 text-center">
			<div class="text-3xl font-bold text-brand sm:text-4xl">1 人民币 ＝ 1 美元额度</div>
			<p class="mx-auto mt-3 max-w-xl text-sm text-muted">人民币充值后按平台配置兑换为美元额度，调用时按模型官方价格和 token 实际消耗扣减。</p>
		</div>
		<div class="mt-6 grid gap-6 md:grid-cols-3">
			{#each billingHighlights as item}
				<div class="rounded-lg border border-line bg-card/70 p-5 text-center">
					<h3 class="text-lg font-semibold text-brand">{item.title}</h3>
					<p class="mt-2 text-sm leading-relaxed text-muted">{item.body}</p>
				</div>
			{/each}
		</div>
		<div class="mt-6 rounded-lg border border-line bg-card/70 p-6">
			<div class="font-mono text-sm text-faint">举个例子：Claude Sonnet 4.6 编码对话</div>
			<div class="mt-4 grid gap-3 text-sm sm:grid-cols-3">
				<div class="flex justify-between gap-6 sm:flex-col sm:gap-1"><span>输入 5,000 Token × $3 / 1M</span><span class="font-mono text-brand">$0.015</span></div>
				<div class="flex justify-between gap-6 sm:flex-col sm:gap-1"><span>输出 2,000 Token × $15 / 1M</span><span class="font-mono text-brand">$0.030</span></div>
				<div class="flex justify-between gap-6 font-semibold sm:flex-col sm:gap-1"><span>合计费用</span><span class="font-mono text-brand">$0.045</span></div>
			</div>
			<a class="mt-5 inline-flex rounded-md border border-line px-4 py-2 text-sm hover:border-brand/50 hover:text-brand" href="{base}/pricing">查看模型价格表 ›</a>
		</div>
		<div class="mt-6 overflow-hidden rounded-xl border border-line">
			<table class="w-full text-left text-sm">
				<thead class="bg-white/[0.03] font-mono text-xs text-faint">
					<tr>
						<th class="px-5 py-3 font-medium">模型</th>
						<th class="px-5 py-3 font-medium">输入</th>
						<th class="px-5 py-3 font-medium">输出</th>
						<th class="px-5 py-3 font-medium">适用场景</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					<tr><td class="px-5 py-3 font-medium">Claude Opus 4.8</td><td class="px-5 py-3 font-mono text-brand">$5/MTok</td><td class="px-5 py-3 font-mono text-brand">$25/MTok</td><td class="px-5 py-3 text-muted">复杂推理、架构设计</td></tr>
					<tr><td class="px-5 py-3 font-medium">Claude Sonnet 4.6</td><td class="px-5 py-3 font-mono text-brand">$3/MTok</td><td class="px-5 py-3 font-mono text-brand">$15/MTok</td><td class="px-5 py-3 text-muted">日常编码主力</td></tr>
					<tr><td class="px-5 py-3 font-medium">Claude Haiku 4.5</td><td class="px-5 py-3 font-mono text-brand">$0.8/MTok</td><td class="px-5 py-3 font-mono text-brand">$4/MTok</td><td class="px-5 py-3 text-muted">快速补全、轻量任务</td></tr>
					<tr><td class="px-5 py-3 font-medium">GPT-5.4</td><td class="px-5 py-3 font-mono text-brand">$5/MTok</td><td class="px-5 py-3 font-mono text-brand">$15/MTok</td><td class="px-5 py-3 text-muted">复杂编码、多步推理</td></tr>
				</tbody>
			</table>
		</div>
		<p class="mt-3 text-center text-xs text-faint">MTok = 百万 Token。价格与官方一致。</p>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-28">
	<div class="text-center">
		<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">定价方案</span>
		<h2 class="mt-3 text-3xl font-bold tracking-tight">按需付费，灵活选择</h2>
		<p class="mt-3 text-muted">按量付费永不过期，月卡用户每天享受大额度刷新。</p>
	</div>
	<div class="mt-16 grid gap-6 md:grid-cols-3">
		{#each plans as plan}
			<div class={`relative rounded-lg border bg-card/70 p-6 ${plan.featured ? 'border-brand/60 shadow-lg shadow-brand/10' : 'border-line'}`}>
				<div class="flex items-center gap-2">
					<span class={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${plan.featured ? 'bg-brand text-black' : 'border border-line text-faint'}`}>{plan.badge}</span>
				</div>
				<h3 class="mt-3 text-lg font-semibold">{plan.name}</h3>
				<div class="mt-3 text-4xl font-bold text-brand">{plan.price}</div>
				<p class="mt-2 min-h-10 text-sm text-faint">{plan.body}</p>
				<div class="mt-5 space-y-2 text-sm">
					{#each plan.items as item}
						<div class="text-slate-300"><span class="text-brand">✓</span> {item}</div>
					{/each}
				</div>
				<a class="mt-6 flex h-10 items-center justify-center rounded-md bg-brand text-sm font-semibold text-black" href="{base}/pricing">选择方案</a>
			</div>
		{/each}
	</div>
	<div class="mt-16 text-center">
		<a class="inline-flex items-center gap-1 text-sm text-brand hover:underline" href="{base}/pricing">查看全部套餐 ›</a>
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="text-center">
			<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">常见问题</span>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">FAQ</h2>
			<p class="mt-3 text-muted">还有其他问题？欢迎 <a class="text-brand hover:underline" href="{base}/contact">联系我们</a>。</p>
		</div>
		<div class="mx-auto mt-20 max-w-3xl space-y-8">
			{#each faqGroups as group}
				<div>
					<h3 class="mb-3 text-sm font-semibold uppercase tracking-wider text-faint">{group.title}</h3>
					<div class="divide-y divide-line rounded-lg border border-line bg-card/60">
						{#each group.items as faq}
							<details class="group p-5">
								<summary class="flex cursor-pointer items-center justify-between gap-6 font-medium marker:content-none">
									<span>{faq[0]}</span>
									<span class="text-brand transition-transform group-open:rotate-180">⌄</span>
								</summary>
								<p class="mt-3 text-sm leading-relaxed text-muted">{faq[1]}</p>
							</details>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</div>
</section>

<section>
	<div class="mx-auto max-w-6xl px-5 py-28 text-center">
		<h2 class="text-3xl font-bold tracking-tight sm:text-4xl">3 分钟开始你的 AI 编码之旅</h2>
		<p class="mx-auto mt-3 max-w-2xl text-muted">注册账号 → 充值 → 创建 API Key → 开始使用。就是这么简单。</p>
		<div class="mt-7 flex justify-center">
			<a class="rounded-md bg-brand px-10 py-3 text-base font-semibold text-black shadow-lg shadow-brand/20" href="{base}/app/register">立即注册</a>
		</div>
		<p class="mt-4 text-sm text-faint">最低 ¥20 起充 · 额度永不过期 · 支持微信 / 支付宝 / USDT</p>
		<p class="mt-2 text-sm text-faint">对分销合作感兴趣？注册后访问 <a class="text-brand hover:underline" href="{base}/partner">分销中心</a> 即可成为合伙人，佣金 15%-40%。</p>
	</div>
</section>
