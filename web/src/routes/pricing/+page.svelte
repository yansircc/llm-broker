<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	type PaygoPlan = {
		name: string;
		tagline: string;
		price: number;
		quota: number;
		hot?: boolean;
		extra?: string;
	};

	type MonthlyPlan = {
		name: string;
		tagline: string;
		price: number;
		daily: number;
		monthly: number;
		perUsd: string;
		discount: string;
		tier: string;
		highlight?: boolean;
	};

	type ModelPrice = {
		model: string;
		group: string;
		input: string;
		output: string;
		cacheWrite: string;
		cacheRead: string;
	};

	type VipLevel = {
		tier: string;
		multiplier: string;
		minSpend: string;
		save: string;
	};

	const paygoPlans: PaygoPlan[] = [
		{ name: '体验', tagline: '适合初次体验', price: 20, quota: 20 },
		{ name: '基础', tagline: '日常轻度使用', price: 50, quota: 50 },
		{ name: '标准', tagline: '开发者常用', price: 100, quota: 100, hot: true },
		{ name: '进阶', tagline: '高频使用', price: 500, quota: 500 },
		{ name: '专业', tagline: '专业开发者', price: 1000, quota: 1000 },
		{ name: '企业', tagline: '企业级用量', price: 5000, quota: 5000, extra: '优先支持' }
	];

	const monthlyPlans: MonthlyPlan[] = [
		{ name: '入门版', tagline: '个人开发者、日常编程辅助', price: 199, daily: 15, monthly: 450, perUsd: '0.44', discount: '0.6折', tier: '入门' },
		{ name: '轻量版', tagline: '个人开发者、日常编程辅助', price: 339, daily: 30, monthly: 900, perUsd: '0.38', discount: '0.5折', tier: '轻量' },
		{ name: '标准版', tagline: '重度代码编写、长文档分析', price: 499, daily: 50, monthly: 1500, perUsd: '0.33', discount: '0.5折', tier: '推荐', highlight: true },
		{ name: '高级版', tagline: '全职独立开发者、AI 极客', price: 1188, daily: 120, monthly: 3600, perUsd: '0.33', discount: '0.5折', tier: '进阶' },
		{ name: '团队版', tagline: '极高频用户、小型工作室', price: 1888, daily: 200, monthly: 6000, perUsd: '0.31', discount: '0.4折', tier: '团队' },
		{ name: '商业版', tagline: '资深高频用户、中大型工作室', price: 4688, daily: 500, monthly: 15000, perUsd: '0.31', discount: '0.4折', tier: '商业' },
		{ name: '企业版', tagline: '资深高频用户、中大型工作室', price: 9188, daily: 1000, monthly: 30000, perUsd: '0.31', discount: '0.4折', tier: '旗舰' }
	];

	const modelPrices: ModelPrice[] = [
		{ model: 'Claude Opus 4.8', group: 'Anthropic', input: '$5', output: '$25', cacheWrite: '$6.25', cacheRead: '$0.5' },
		{ model: 'Claude Opus 4.7', group: 'Anthropic', input: '$5', output: '$25', cacheWrite: '$6.25', cacheRead: '$0.5' },
		{ model: 'Claude Opus 4.6', group: 'Anthropic', input: '$5', output: '$25', cacheWrite: '$6.25', cacheRead: '$0.5' },
		{ model: 'Claude Opus 4.5', group: 'Anthropic', input: '$5', output: '$25', cacheWrite: '$6.25', cacheRead: '$0.5' },
		{ model: 'Claude Sonnet 4.7', group: 'Anthropic', input: '$3', output: '$15', cacheWrite: '$3.75', cacheRead: '$0.3' },
		{ model: 'Claude Sonnet 4.6', group: 'Anthropic', input: '$3', output: '$15', cacheWrite: '$3.75', cacheRead: '$0.3' },
		{ model: 'Claude Sonnet 4.5', group: 'Anthropic', input: '$3', output: '$15', cacheWrite: '$3.75', cacheRead: '$0.3' },
		{ model: 'Claude Haiku 4.5', group: 'Anthropic', input: '$0.8', output: '$4', cacheWrite: '$1', cacheRead: '$0.08' },
		{ model: 'GPT-5.4', group: 'OpenAI', input: '$5', output: '$15', cacheWrite: '—', cacheRead: '—' },
		{ model: 'GPT-5', group: 'OpenAI', input: '$5', output: '$15', cacheWrite: '—', cacheRead: '—' },
		{ model: 'GPT-5 Mini', group: 'OpenAI', input: '$1.5', output: '$6', cacheWrite: '—', cacheRead: '—' },
		{ model: 'GPT-4o', group: 'OpenAI', input: '$2.5', output: '$10', cacheWrite: '—', cacheRead: '—' },
		{ model: 'o3', group: 'OpenAI', input: '$10', output: '$40', cacheWrite: '—', cacheRead: '—' },
		{ model: 'o3-pro', group: 'OpenAI', input: '$20', output: '$80', cacheWrite: '—', cacheRead: '—' },
		{ model: 'o4-mini', group: 'OpenAI', input: '$1.1', output: '$4.4', cacheWrite: '—', cacheRead: '—' },
		{ model: 'Codex Mini', group: 'OpenAI', input: '$1.5', output: '$6', cacheWrite: '—', cacheRead: '—' },
		{ model: 'DeepSeek V4 Pro', group: 'DeepSeek', input: '$3', output: '$15', cacheWrite: '$3.75', cacheRead: '$0.3' },
		{ model: 'DeepSeek V4 Flash', group: 'DeepSeek', input: '$1', output: '$3', cacheWrite: '$1', cacheRead: '$0.1' },
		{ model: 'Gemini 3.5 Flash', group: 'Google', input: '$0.15', output: '$0.6', cacheWrite: '—', cacheRead: '—' },
		{ model: 'Gemini 2.5 Pro', group: 'Google', input: '$1.25', output: '$10', cacheWrite: '—', cacheRead: '—' },
		{ model: 'Gemini 2.5 Flash', group: 'Google', input: '$0.15', output: '$0.6', cacheWrite: '—', cacheRead: '—' },
		{ model: 'Gemini 2.5 Flash Lite', group: 'Google', input: '$0.05', output: '$0.2', cacheWrite: '—', cacheRead: '—' }
	];

	const vipLevels: VipLevel[] = [
		{ tier: '普通', multiplier: '1x', minSpend: '—', save: '—' },
		{ tier: 'VIP1', multiplier: '0.98x', minSpend: '¥299', save: '2%' },
		{ tier: 'VIP2', multiplier: '0.95x', minSpend: '¥1299', save: '5%' },
		{ tier: 'VIP3', multiplier: '0.93x', minSpend: '¥2999', save: '7%' },
		{ tier: 'VIP5', multiplier: '0.9x', minSpend: '¥5999', save: '10%' },
		{ tier: 'VIP8', multiplier: '0.88x', minSpend: '¥9999', save: '12%' }
	];
</script>

<svelte:head>
	<title>透明定价 · {BRAND_NAME}</title>
</svelte:head>

<!-- Hero -->
<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-28 text-center">
		<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">透明定价</span>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">透明定价，按需选择</h1>
		<p class="mx-auto mt-4 max-w-3xl text-muted">PAYGO 按量付费 + 月卡订阅，满足不同场景需求</p>
		<div class="mt-16 flex flex-wrap justify-center gap-3">
			<a class="rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/app/register">立即体验</a>
			<a class="rounded-md border border-line px-5 py-3 text-sm hover:border-brand/50" href="{base}/docs">查看文档</a>
		</div>
	</div>
</section>

<!-- PAYGO -->
<section id="personal" class="mx-auto max-w-6xl px-5 py-28">
	<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
		<div>
			<div class="font-mono text-xs uppercase tracking-wider text-brand">pay as you go</div>
			<h2 class="mt-2 text-2xl font-bold">按量付费 (Pay-As-You-Go)</h2>
			<p class="mt-2 text-sm text-faint">1 RMB = 1 USD 额度 · 永不过期</p>
		</div>
	</div>
	<div class="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
		{#each paygoPlans as plan (plan.name)}
			<article class={`relative flex flex-col rounded-lg border bg-card/60 p-6 ${plan.hot ? 'border-brand/60 shadow-[0_0_0_1px_rgba(0,255,65,0.18)]' : 'border-line'}`}>
				<div class="flex items-center justify-between gap-3">
					<div>
						<h3 class="text-lg font-semibold">{plan.name}</h3>
						<p class="mt-1 text-sm text-faint">{plan.tagline}</p>
					</div>
					<div class="flex flex-col items-end gap-1.5">
						<span class="rounded-md border border-brand/40 bg-brand/[0.08] px-2 py-1 text-xs font-semibold text-brand">省86%</span>
						{#if plan.hot}
							<span class="rounded-md bg-brand px-2 py-1 text-xs font-semibold text-black">热门</span>
						{/if}
					</div>
				</div>
				<div class="mt-5 text-4xl font-bold text-brand">¥{plan.price}</div>
				<p class="mt-2 text-sm text-muted">${plan.quota} USD 额度</p>
				<div class="mt-4 space-y-1.5 text-sm text-slate-300">
					<div><span class="text-brand">✓</span> ${plan.quota} 额度</div>
					<div><span class="text-brand">✓</span> 永不过期</div>
					<div><span class="text-brand">✓</span> 支持全部模型</div>
					{#if plan.extra}
						<div><span class="text-brand">✓</span> {plan.extra}</div>
					{/if}
				</div>
				<a class="mt-6 flex h-10 items-center justify-center rounded-md bg-brand text-sm font-semibold text-black" href="{base}/app/billing">立即充值</a>
			</article>
		{/each}
	</div>
</section>

<!-- Monthly -->
<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">monthly cards</div>
		<h2 class="mt-2 text-2xl font-bold">月卡订阅</h2>
		<p class="mt-2 text-sm text-faint">每日固定额度，每日刷新，性价比更高</p>
		<div class="mt-16 grid gap-6 md:grid-cols-2 xl:grid-cols-4">
			{#each monthlyPlans as plan (plan.name)}
				<article class={`relative flex flex-col rounded-lg border bg-card/70 p-6 ${plan.highlight ? 'border-brand/60 shadow-[0_0_0_1px_rgba(0,255,65,0.18)]' : 'border-line'}`}>
					<div class="flex items-center justify-between gap-3">
						<span class="rounded-md border border-brand/40 bg-brand/[0.08] px-2 py-1 text-xs font-semibold text-brand">{plan.discount}</span>
						{#if plan.highlight}
							<span class="rounded-md bg-brand px-2 py-1 text-xs font-semibold text-black">推荐</span>
						{:else}
							<span class="rounded-md border border-line bg-black/20 px-2 py-1 font-mono text-xs text-faint">{plan.tier}</span>
						{/if}
					</div>
					<h3 class="mt-4 text-lg font-semibold">{plan.name}</h3>
					<p class="mt-1 text-sm text-faint">{plan.tagline}</p>
					<div class="mt-4 text-3xl font-bold text-brand">¥{plan.price} <span class="text-base font-medium text-muted">/月</span></div>
					<div class="mt-4 space-y-1 rounded-md border border-line bg-black/20 p-3 text-sm">
						<div class="flex justify-between"><span class="text-faint">每日额度</span><span class="font-semibold">${plan.daily}</span></div>
						<div class="flex justify-between"><span class="text-faint">月总额度</span><span class="font-semibold">${plan.monthly}</span></div>
						<div class="flex justify-between"><span class="text-faint">折合</span><span class="font-semibold text-brand">¥{plan.perUsd}/USD</span></div>
					</div>
					<div class="mt-4 space-y-1.5 text-sm text-slate-300">
						<div><span class="text-brand">✓</span> ${plan.daily}/天额度</div>
						<div><span class="text-brand">✓</span> 每日刷新</div>
						<div><span class="text-brand">✓</span> 支持全部模型</div>
						<div><span class="text-brand">✓</span> ≈ ¥{plan.perUsd}/USD</div>
					</div>
					<a class="mt-auto pt-5" href="{base}/app/billing">
						<span class="flex h-10 items-center justify-center rounded-md bg-brand text-sm font-semibold text-black">立即订阅</span>
					</a>
				</article>
			{/each}
		</div>
	</div>
</section>

<!-- Model pricing -->
<section class="mx-auto max-w-6xl px-5 py-28">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">model pricing</div>
	<h2 class="mt-2 text-2xl font-bold">模型定价明细</h2>
	<p class="mt-2 text-sm text-faint">按 Token 计费，用多少付多少</p>
	<div class="mt-6 overflow-x-auto rounded-lg border border-line bg-card/60">
		<table class="w-full min-w-[860px] text-left text-sm">
			<thead class="bg-white/[0.03] font-mono text-xs text-faint">
				<tr>
					<th class="px-5 py-3 font-medium">模型</th>
					<th class="px-5 py-3 font-medium">分组</th>
					<th class="px-5 py-3 font-medium">INPUT ($/MTOK)</th>
					<th class="px-5 py-3 font-medium">OUTPUT ($/MTOK)</th>
					<th class="px-5 py-3 font-medium">CACHE WRITE</th>
					<th class="px-5 py-3 font-medium">CACHE READ</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each modelPrices as price (price.model)}
					<tr>
						<td class="px-5 py-3 font-semibold">{price.model}</td>
						<td class="px-5 py-3 text-muted">{price.group}</td>
						<td class="px-5 py-3">{price.input}</td>
						<td class="px-5 py-3">{price.output}</td>
						<td class="px-5 py-3 text-muted">{price.cacheWrite}</td>
						<td class="px-5 py-3 text-muted">{price.cacheRead}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</section>

<!-- VIP -->
<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">vip levels</div>
		<h2 class="mt-2 text-2xl font-bold">VIP 等级</h2>
		<p class="mt-2 text-sm text-faint">累计消费越多，倍率越优惠</p>
		<div class="mt-16 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each vipLevels as level (level.tier)}
				<div class="rounded-lg border border-line bg-card/60 p-5">
					<div class="flex items-center justify-between">
						<div class="font-mono text-sm text-brand">{level.tier}</div>
						<div class="text-lg font-bold">{level.multiplier}</div>
					</div>
					<div class="mt-4 space-y-1 text-sm">
						<div class="flex justify-between"><span class="text-faint">最低累计消费</span><span>{level.minSpend}</span></div>
						<div class="flex justify-between"><span class="text-faint">节省比例</span><span class="text-brand">{level.save}</span></div>
					</div>
				</div>
			{/each}
		</div>
		<div class="mt-8 overflow-x-auto rounded-lg border border-line bg-card/60">
			<table class="w-full min-w-[640px] text-left text-sm">
				<thead class="bg-white/[0.03] font-mono text-xs text-faint">
					<tr>
						<th class="px-5 py-3 font-medium">等级</th>
						<th class="px-5 py-3 font-medium">费率倍数</th>
						<th class="px-5 py-3 font-medium">最低累计消费</th>
						<th class="px-5 py-3 font-medium">节省比例</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each vipLevels as level (level.tier)}
						<tr>
							<td class="px-5 py-3 font-mono text-brand">{level.tier}</td>
							<td class="px-5 py-3 font-semibold">{level.multiplier}</td>
							<td class="px-5 py-3 text-muted">{level.minSpend}</td>
							<td class="px-5 py-3 text-brand">{level.save}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
</section>

<!-- Enterprise -->
<section class="mx-auto max-w-6xl px-5 py-28 text-center">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">enterprise</div>
	<h2 class="mt-2 text-2xl font-bold">企业定制方案</h2>
	<p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-muted">支持企业对公转账、开具发票、定制 API 部署</p>
	<div class="mt-8 flex flex-wrap justify-center gap-3">
		<a class="rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/contact">联系我们</a>
		<a class="rounded-md border border-line px-5 py-3 text-sm hover:border-brand/50" href="#personal">查看个人方案</a>
	</div>
</section>
