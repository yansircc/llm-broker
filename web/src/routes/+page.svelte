<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_DESCRIPTION, BRAND_NAME, BRAND_TAGLINE } from '$lib/brand';

	let origin = $state('https://your-domain.example');

	$effect(() => {
		if (browser) origin = window.location.origin;
	});

	const campaignSteps = [
		'登录赠 1 枚印章',
		'每充 ¥49 得 1 枚',
		'好友首充双方各 +1 枚',
		'集满 4 枚开盲盒'
	];

	const stats = [
		{ value: '1 RMB = 1 USD', label: '官方 1.4 折' },
		{ value: '多线路智能调度', label: '自动故障切换' },
		{ value: '99.9% 稳定率', label: '7x24 监控' }
	];

	const features = [
		{
			title: '满血不掺水',
			body: '企业级通道直连，拒绝逆向阉割和以次充好。内置一键 Key 测试功能，可验证模型、余额和连通性。'
		},
		{
			title: '一键接入',
			body: 'Codex CLI 与 OpenAI 兼容工具当前可直接接入；Claude Code、OpenClaw、Cursor 等按同一 Base URL / Key 形态预留。'
		},
		{
			title: '极致性价比',
			body: '1 人民币 = 1 美元额度，按真实消耗扣费。余额长期有效，不用不扣。'
		},
		{
			title: '稳定透明',
			body: '多线路调度、失败自动切换。每笔调用、扣费、订单和推广收益都可追溯。'
		}
	];

	const modelCards = [
		{ name: 'Claude Opus 4.8', provider: 'Anthropic', status: '预留接入', copy: '旗舰级复杂推理与架构设计模型', input: '$5', output: '$25' },
		{ name: 'Claude Sonnet 4.6', provider: 'Anthropic', status: '预留接入', copy: '日常主力编码模型，速度与质量均衡', input: '$3', output: '$15' },
		{ name: 'Claude Haiku 4.5', provider: 'Anthropic', status: '预留接入', copy: '快速轻量，适合补全、摘要和批量任务', input: '$0.8', output: '$4' },
		{ name: 'GPT-5.4', provider: 'OpenAI', status: '预留接入', copy: '下一代通用推理、编码与多模态能力', input: '$5', output: '$15' }
	];

	const modelList = ['GPT-4o', 'o3', 'o3-pro', 'o4-mini', 'Codex 当前可用'];

	const platforms = [
		{ name: 'Claude Code', body: 'Claude 家族接入预留后可按 Anthropic 兼容格式使用。' },
		{ name: 'Codex', body: '当前可用的核心中转能力，适合 CLI 与编码工作流。' },
		{ name: 'OpenCode', body: '接入 OpenAI 兼容端点，适合本地代理式开发。' },
		{ name: 'OpenClaw', body: '统一 Key 管理，按项目拆分调用与账单。' },
		{ name: 'Cursor', body: '配置自定义 Base URL 后复用同一余额。' },
		{ name: 'VS Code', body: '配合插件或 CLI，在编辑器内直接调用模型。' },
		{ name: 'Windsurf', body: '为 AI IDE 预留统一模型入口和账本。' },
		{ name: 'CherryStudio', body: '适合桌面客户端测试模型、Key 和多会话。' }
	];

	const billingPoints = [
		'1 人民币 = 1 美元额度',
		'按量计费，用多少扣多少',
		'月卡用户每日享有固定额度',
		'VIP 等级享阶梯折扣',
		'支持模型价格表查询'
	];

	const plans = [
		{
			name: '基础',
			price: '¥20 起',
			body: '适合试用、低频调用和个人小项目。',
			items: ['$20 起充额度', '余额永不过期', '创建多个 API Key']
		},
		{
			name: '标准版月卡',
			price: '¥499/月',
			body: '适合高频个人开发者和连续编码任务。',
			items: ['$50/天额度', '每日自动刷新', '支持按量余额叠加'],
			featured: true
		},
		{
			name: '团队版月卡',
			price: '¥1,888/月',
			body: '适合小团队、工作室和批量自动化任务。',
			items: ['$200/天额度', '团队项目分 Key 管理', '更高并发预留']
		}
	];

	const faqGroups = [
		{
			title: '基础入门',
			items: [
				[`${BRAND_NAME} 是什么？`, `${BRAND_NAME} 是面向开发者的 AI API 中转服务。当前 Codex 中转可用，Claude 家族按同一产品形态预留接入。`],
				['国内能直接用吗？需要代理吗？', '目标体验是开箱即用：在工具里配置平台提供的 Base URL 和 API Key，即可走中转服务。'],
				['怎么开始使用？', '注册账号、充值、创建 API Key，再把工具的 API 地址改成当前站点地址即可。']
			]
		},
		{
			title: '定价与计费',
			items: [
				['为什么写 1 人民币 = 1 美元额度？', '余额用美元额度展示，人民币充值后按平台配置兑换，调用时按模型价格和 token 消耗扣减。'],
				['额度会过期吗？', '按量余额长期有效；月卡额度按日刷新，到期后不再刷新。'],
				['能不能查看每次扣费？', '可以。调用记录、订单、余额流水会保留用于追溯。']
			]
		},
		{
			title: '功能与能力',
			items: [
				['支持哪些工具？', 'Codex 当前可用；页面所列 Claude Code、OpenCode、OpenClaw、Cursor、VS Code、Windsurf、CherryStudio 走统一兼容接入设计。'],
				['支持 AI 生图吗？', '页面预留 OpenAI Images 兼容入口，支持按生图 Key 分组和按张计费的产品形态。'],
				['可以多个项目分开用吗？', '可以按项目、工具或用途创建不同 Key，便于统计消耗和定位问题。']
			]
		},
		{
			title: '安全与稳定',
			items: [
				['Key 安全吗？', '平台只展示必要的 Key 信息，敏感凭据按后端策略保存，不在页面明文泄露。'],
				['失败会自动切换吗？', '中转层按账号池和线路状态做调度，失败路径会进入可观测记录，便于排查。'],
				['服务是否透明？', '订单、余额、调用日志和模型价格表都提供查询入口，避免黑盒扣费。']
			]
		}
	];
</script>

<section class="border-b border-brand/20 bg-brand/[0.07]">
	<div class="mx-auto flex max-w-6xl flex-col gap-5 px-5 py-6 lg:flex-row lg:items-center lg:justify-between">
		<div>
			<div class="font-mono text-xs uppercase tracking-wider text-brand">六月限时活动 · 截止 06/30</div>
			<h2 class="mt-2 text-2xl font-bold tracking-tight sm:text-3xl">集印章开盲盒，最高随机 $500 额度</h2>
			<p class="mt-2 text-sm text-muted">集满 4 枚即可开一次盲盒，随机获得 $20 / $50 / $100，1% 概率获得 $500。</p>
		</div>
		<div class="grid gap-2 text-sm sm:grid-cols-2 lg:min-w-[430px]">
			{#each campaignSteps as step}
				<div class="rounded-md border border-brand/25 bg-black/25 px-3 py-2 text-slate-200">{step}</div>
			{/each}
		</div>
		<a class="inline-flex h-11 items-center justify-center rounded-md bg-brand px-6 font-semibold text-black shadow-lg shadow-brand/20" href="{base}/app/register">立即领取</a>
	</div>
</section>

<section class="relative overflow-hidden border-b border-line">
	<div class="absolute inset-0 bg-[radial-gradient(circle_at_80%_10%,rgba(0,255,65,0.12),transparent_28rem)]"></div>
	<div class="relative mx-auto grid max-w-6xl items-center gap-10 px-5 py-20 lg:grid-cols-[1fr_0.9fr] lg:py-24">
		<div>
			<div class="inline-flex rounded-full border border-brand/40 bg-brand/[0.08] px-3 py-1 font-mono text-xs text-brand">{BRAND_TAGLINE}</div>
			<h1 class="mt-6 max-w-3xl text-5xl font-bold leading-tight tracking-tight sm:text-7xl">{BRAND_NAME}</h1>
			<p class="mt-5 max-w-2xl text-xl text-slate-200">{BRAND_DESCRIPTION}</p>
			<p class="mt-3 max-w-2xl text-sm leading-6 text-faint">CC = Claude Code。当前 Codex 中转可用；Claude Opus / Sonnet / Haiku 家族按同一套 Key、账本和控制台预留接入。</p>
			<div class="mt-8 flex flex-wrap gap-3">
				<a class="rounded-md bg-brand px-8 py-3 text-base font-semibold text-black shadow-lg shadow-brand/20" href="{base}/app/register">立即体验</a>
				<a class="rounded-md border border-line bg-black/20 px-8 py-3 font-mono text-base hover:border-brand/50 hover:text-brand" href="{base}/docs">查看文档</a>
			</div>
		</div>

		<div class="rounded-xl border border-line bg-[#050505] shadow-2xl shadow-black/40">
			<div class="flex items-center gap-2 border-b border-line px-4 py-3 font-mono text-xs text-faint">
				<span class="h-2.5 w-2.5 rounded-full bg-red-400"></span>
				<span class="h-2.5 w-2.5 rounded-full bg-amber-400"></span>
				<span class="h-2.5 w-2.5 rounded-full bg-brand"></span>
				<span class="ml-2">Claude API 调用示例</span>
			</div>
			<div class="space-y-2 p-5 font-mono text-sm leading-6">
				<div class="text-faint"># Claude 家族预留接入格式</div>
				<div class="break-all text-brand">$ curl -X POST {origin}/v1/messages \</div>
				<div class="pl-4 text-faint">-H "Authorization: Bearer sk-xxx" \</div>
				<div class="pl-4 text-faint">-H "Content-Type: application/json" \</div>
				<div class="pl-4 text-faint">-d '&#123;"model":"claude-sonnet-4-6","messages":[&#123;"role":"user","content":"hello"&#125;]&#125;'</div>
				<div class="pt-2 text-brand"># Codex 中转当前可用</div>
				<div class="text-faint">&#123;"content":"Hello!"&#125;</div>
			</div>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-12">
	<div class="grid gap-4 md:grid-cols-3">
		{#each stats as stat}
			<div class="rounded-lg border border-line bg-card/70 p-6 text-center">
				<div class="font-mono text-xl font-semibold text-brand">{stat.value}</div>
				<div class="mt-1 text-sm text-faint">{stat.label}</div>
			</div>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-16">
		<div class="text-center">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">why choose us</div>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">用户需要的，我们都有</h2>
			<p class="mx-auto mt-3 max-w-2xl text-muted">把 Key、额度、调用记录和工具接入放到一个控制台里，减少开发者处理账号、网络和账单的时间。</p>
		</div>
		<div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each features as feature}
				<div class="rounded-lg border border-line bg-card/70 p-5">
					<div class="mb-4 flex h-9 w-9 items-center justify-center rounded-md border border-brand/30 bg-brand/[0.08] font-mono text-brand">✓</div>
					<h3 class="text-lg font-semibold">{feature.title}</h3>
					<p class="mt-2 text-sm leading-relaxed text-muted">{feature.body}</p>
				</div>
			{/each}
		</div>
	</div>
</section>

	<section class="mx-auto max-w-6xl px-5 py-16">
		<div class="grid gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
			<div>
				<div class="font-mono text-xs uppercase tracking-wider text-brand">新功能 · AI 生图</div>
				<h2 class="mt-3 text-3xl font-bold tracking-tight">OpenAI Images 兼容入口，视觉与计费形态已对齐</h2>
				<p class="mt-3 text-muted">预留 gpt-image-1 / dall-e-3 的接入形态，生图 Key 分组、按张计费和余额扣除将在后端驱动接入后启用。</p>
				<div class="mt-5 flex flex-wrap gap-2 text-xs text-faint">
					<span class="rounded-md border border-line bg-card px-3 py-1.5">预留 /v1/images/generations</span>
					<span class="rounded-md border border-line bg-card px-3 py-1.5">生图 Key 分组</span>
					<span class="rounded-md border border-line bg-card px-3 py-1.5">按张计费规则</span>
				</div>
				<a class="mt-6 inline-flex rounded-md border border-line bg-card px-4 py-2 text-sm hover:border-brand/50" href="{base}/app/images">查看预留入口</a>
			</div>
			<pre class="m-0 text-sm leading-6"><code># 生图功能预留接入格式
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
	<div class="mx-auto max-w-6xl px-5 py-16">
			<div class="text-center">
				<div class="font-mono text-xs uppercase tracking-wider text-brand">models</div>
				<h2 class="mt-3 text-3xl font-bold tracking-tight">一个 Key，统一管理模型接入</h2>
				<p class="mx-auto mt-3 max-w-2xl text-muted">Codex 当前可用；Claude 家族与更多 OpenAI/GPT 模型按统一 Key、余额和价格表预留接入。</p>
			</div>
		<div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each modelCards as model}
				<div class="rounded-lg border border-line bg-card/70 p-5">
					<div class="flex items-center justify-between gap-3">
						<div class="font-mono text-xs text-brand">{model.provider}</div>
						<div class="rounded-full border border-line px-2 py-1 font-mono text-[11px] text-faint">{model.status}</div>
					</div>
					<h3 class="mt-3 text-base font-semibold">{model.name}</h3>
					<p class="mt-2 min-h-12 text-sm text-muted">{model.copy}</p>
					<div class="mt-4 grid grid-cols-2 gap-2 text-xs">
						<div class="rounded-md border border-line bg-black/20 p-2">输入 <span class="font-mono text-brand">{model.input}</span>/MTok</div>
						<div class="rounded-md border border-line bg-black/20 p-2">输出 <span class="font-mono text-brand">{model.output}</span>/MTok</div>
					</div>
				</div>
			{/each}
		</div>
		<div class="mt-6 flex flex-wrap justify-center gap-2">
			{#each modelList as model}
				<span class="rounded-full border border-line bg-card px-3 py-1.5 font-mono text-xs text-slate-300">{model}</span>
			{/each}
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-16">
		<div class="text-center">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">platform support</div>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">覆盖常用开发工具接入形态</h2>
		</div>
	<div class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		{#each platforms as platform}
			<div class="rounded-lg border border-line bg-card/70 p-5">
				<h3 class="font-semibold">{platform.name}</h3>
				<p class="mt-2 text-sm leading-relaxed text-muted">{platform.body}</p>
			</div>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-16">
		<div class="grid gap-8 lg:grid-cols-[0.85fr_1.15fr] lg:items-start">
			<div>
				<div class="font-mono text-xs uppercase tracking-wider text-brand">billing</div>
				<h2 class="mt-3 text-3xl font-bold tracking-tight">简单透明，用多少付多少</h2>
				<p class="mt-3 text-muted">按量余额和月卡额度共用同一账本。模型价格、调用消耗、余额变动都能在控制台追溯。</p>
				<div class="mt-6 grid gap-2 text-sm">
					{#each billingPoints as point}
						<div class="rounded-md border border-line bg-card/70 px-3 py-2 text-slate-300">{point}</div>
					{/each}
				</div>
			</div>
			<div class="rounded-lg border border-line bg-card/70 p-5">
				<div class="font-mono text-sm text-faint">示例：Claude Sonnet 4.6 编码对话</div>
				<div class="mt-4 space-y-3 text-sm">
					<div class="flex justify-between gap-4"><span>输入 5,000 Token × $3 / 1M</span><span class="font-mono text-brand">$0.015</span></div>
					<div class="flex justify-between gap-4"><span>输出 2,000 Token × $15 / 1M</span><span class="font-mono text-brand">$0.030</span></div>
					<div class="flex justify-between gap-4 border-t border-line pt-3 font-semibold"><span>合计费用</span><span class="font-mono text-brand">$0.045</span></div>
				</div>
				<div class="mt-5 rounded-md border border-line bg-black/20 p-3 font-mono text-xs text-faint">公式：(5000 / 1,000,000 × 3) + (2000 / 1,000,000 × 15) = $0.045</div>
				<a class="mt-5 inline-flex rounded-md border border-line px-4 py-2 text-sm hover:border-brand/50 hover:text-brand" href="{base}/pricing">查看模型价格表</a>
			</div>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-16">
	<div class="text-center">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">pricing</div>
		<h2 class="mt-3 text-3xl font-bold tracking-tight">定价方案</h2>
		<p class="mt-3 text-muted">按需付费，灵活选择。轻量试用走基础余额，高频任务走月卡。</p>
	</div>
	<div class="mt-8 grid gap-4 md:grid-cols-3">
		{#each plans as plan}
			<div class={`relative rounded-lg border bg-card/70 p-6 ${plan.featured ? 'border-brand/60 shadow-lg shadow-brand/10' : 'border-line'}`}>
				{#if plan.featured}
					<div class="absolute right-4 top-4 rounded-full bg-brand px-2 py-1 font-mono text-[11px] font-semibold text-black">推荐</div>
				{/if}
				<h3 class="text-lg font-semibold">{plan.name}</h3>
				<div class="mt-4 text-3xl font-bold">{plan.price}</div>
				<p class="mt-2 min-h-10 text-sm text-faint">{plan.body}</p>
				<div class="mt-5 space-y-2 text-sm">
					{#each plan.items as item}
						<div class="text-slate-300"><span class="text-brand">✓</span> {item}</div>
					{/each}
				</div>
				<a class="mt-6 flex h-10 items-center justify-center rounded-md bg-brand font-mono text-sm font-semibold text-black" href="{base}/pricing">选择方案</a>
			</div>
		{/each}
	</div>
	<div class="mt-8 text-center">
		<a class="inline-flex rounded-md border border-line bg-black/20 px-5 py-2.5 text-sm hover:border-brand/50 hover:text-brand" href="{base}/pricing">查看全部套餐</a>
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-16">
		<div class="text-center">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">faq</div>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">常见问题</h2>
		</div>
		<div class="mt-10 grid gap-5 lg:grid-cols-2">
			{#each faqGroups as group}
				<div class="rounded-lg border border-line bg-card/60">
					<div class="border-b border-line px-5 py-4">
						<h3 class="font-semibold">{group.title}</h3>
					</div>
					<div class="divide-y divide-line">
						{#each group.items as faq}
							<div class="p-5">
								<div class="flex items-start justify-between gap-4">
									<h4 class="font-medium">{faq[0]}</h4>
									<span class="font-mono text-brand">+</span>
								</div>
								<p class="mt-2 text-sm leading-relaxed text-muted">{faq[1]}</p>
							</div>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</div>
</section>

<section>
	<div class="mx-auto max-w-6xl px-5 py-16">
		<div class="rounded-xl border border-brand/30 bg-brand/[0.08] p-8 text-center shadow-2xl shadow-black/30">
			<h2 class="text-3xl font-bold tracking-tight sm:text-4xl">3 分钟开始你的 AI 编码之旅</h2>
			<p class="mx-auto mt-3 max-w-2xl text-muted">最低 ¥20 起充，额度永不过期。支持微信、支付宝、USDT，分销中心佣金 15%-40%。</p>
			<div class="mt-6 flex flex-wrap justify-center gap-3">
				<a class="rounded-md bg-brand px-8 py-3 text-base font-semibold text-black shadow-lg shadow-brand/20" href="{base}/app/register">立即体验</a>
				<a class="rounded-md border border-line bg-black/20 px-8 py-3 font-mono text-base hover:border-brand/50 hover:text-brand" href="{base}/partner">查看分销中心</a>
			</div>
		</div>
	</div>
</section>
