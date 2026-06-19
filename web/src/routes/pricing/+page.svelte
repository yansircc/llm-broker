<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	type PaygoPlan = {
		name: string;
		amount: number;
		description: string;
	};

	type MonthlyPlan = {
		name: string;
		price: string;
		quota: string;
		description: string;
		highlight?: boolean;
	};

	type ModelPrice = {
		group: string;
		model: string;
		modelId: string;
		input: string;
		output: string;
		cache: string;
		status: string;
	};

	const paygoPlans: PaygoPlan[] = [
		{ name: '体验', amount: 20, description: '首次验证接口、CLI 配置和账单扣费' },
		{ name: '基础', amount: 50, description: '个人低频调用和短期项目调试' },
		{ name: '标准', amount: 100, description: '开发者日常 Codex 中转使用' },
		{ name: '进阶', amount: 500, description: '高频编码、批量任务和团队预存' },
		{ name: '专业', amount: 1000, description: '稳定开发工作流和多成员共享' },
		{ name: '企业', amount: 5000, description: '企业采购、专项额度和发票沟通' }
	];

	const monthlyPlans: MonthlyPlan[] = [
		{ name: '入门版', price: '¥99/月', quota: '每日 USD 10 额度', description: '轻量体验和学习配置' },
		{ name: '轻量版', price: '¥199/月', quota: '每日 USD 20 额度', description: '个人日常编码任务' },
		{ name: '标准版', price: '¥499/月', quota: '每日 USD 50 额度', description: '高频 Codex 工作流', highlight: true },
		{ name: '高级版', price: '¥999/月', quota: '每日 USD 100 额度', description: '重度开发者和自动化任务' },
		{ name: '团队版', price: '¥1,888/月', quota: '每日 USD 200 额度', description: '小团队共享额度池' },
		{ name: '商业版', price: '¥3,888/月', quota: '每日 USD 500 额度', description: '商业团队持续调用' },
		{ name: '企业版', price: '联系销售', quota: '专属额度和结算', description: '采购、合规、专属支持' }
	];

	const modelPrices: ModelPrice[] = [
		{ group: 'Anthropic', model: 'Claude Opus 4.8', modelId: 'claude-opus-4-8', input: '$15.00/MTok', output: '$75.00/MTok', cache: 'write $18.75 · read $1.50', status: '即将接入同一套餐表达' },
		{ group: 'Anthropic', model: 'Claude Sonnet 4.7', modelId: 'claude-sonnet-4-7', input: '$3.00/MTok', output: '$15.00/MTok', cache: 'write $3.75 · read $0.30', status: '即将接入同一套餐表达' },
		{ group: 'OpenAI', model: 'GPT-5 Mini', modelId: 'gpt-5-mini', input: '$0.25/MTok', output: '$2.00/MTok', cache: 'read $0.025', status: '目标套餐价格位' },
		{ group: 'OpenAI', model: 'Codex Mini', modelId: 'codex-mini-latest', input: '$1.50/MTok', output: '$6.00/MTok', cache: '按后端账单记录', status: '当前 Codex 中转' },
		{ group: 'Google', model: 'Gemini 2.5 Flash', modelId: 'gemini-2.5-flash', input: '$0.30/MTok', output: '$2.50/MTok', cache: 'read $0.075', status: '目标套餐价格位' },
		{ group: 'Image', model: 'OpenAI 图像生成', modelId: 'gpt-image-1', input: '按张计费', output: '按尺寸计费', cache: '不适用', status: '图像入口保留' }
	];

	const vipLevels = [
		['VIP 1', '累计充值 ¥100+', '基础排障支持、余额提醒'],
		['VIP 2', '累计充值 ¥500+', '更高并发建议、优先处理账单问题'],
		['VIP 3', '累计充值 ¥1,000+', '团队配置建议、发票/采购协助'],
		['VIP 4', '累计充值 ¥5,000+', '专属额度沟通、企业方案评估']
	];
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-16 text-center">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">pricing</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">透明定价，按需选择</h1>
		<p class="mx-auto mt-4 max-w-3xl text-muted">
			{BRAND_NAME} 当前提供 Codex 中转。Claude 家族将接入同一套餐表达；GPT、Gemini 和图像模型为目标商业页价格位展示，实际可用模型以控制台和接口返回为准。
		</p>
		<div class="mt-8 flex flex-wrap justify-center gap-3">
			<a class="rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/app/billing">充值余额</a>
			<a class="rounded-md border border-line px-5 py-3 text-sm hover:border-brand/50" href="{base}/models">查看模型</a>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-14">
	<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
		<div>
			<div class="font-mono text-xs uppercase tracking-wider text-brand">pay as you go</div>
			<h2 class="mt-2 text-2xl font-bold">按量付费</h2>
			<p class="mt-2 text-sm text-faint">充值获得预付额度，用多少扣多少，适合先验证再扩大用量。</p>
		</div>
		<div class="rounded-md border border-line bg-card/60 px-3 py-2 font-mono text-xs text-faint">人民币充值 · USD 额度记账</div>
	</div>
	<div class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each paygoPlans as plan (plan.name)}
			<article class="flex min-h-[210px] flex-col rounded-lg border border-line bg-card/60 p-6">
				<div class="flex items-center justify-between gap-3">
					<h3 class="text-lg font-semibold">{plan.name}</h3>
					<span class="rounded-md border border-line bg-black/20 px-2 py-1 font-mono text-xs text-faint">PAYGO</span>
				</div>
				<div class="mt-5 text-4xl font-bold text-brand">¥{plan.amount}</div>
				<p class="mt-3 min-h-10 text-sm text-faint">{plan.description}</p>
				<p class="mt-auto pt-5 font-mono text-xs text-muted">到账额度：USD {plan.amount}</p>
			</article>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-14">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">monthly cards</div>
		<h2 class="mt-2 text-2xl font-bold">月卡订阅</h2>
		<p class="mt-2 text-sm text-faint">月卡用于目标商业页展示；正式购买入口以后端订阅能力和运营开关为准。</p>
		<div class="mt-8 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
			{#each monthlyPlans as plan (plan.name)}
				<article class={`flex min-h-[230px] flex-col rounded-lg border bg-card/70 p-6 ${plan.highlight ? 'border-brand/60 shadow-[0_0_0_1px_rgba(0,255,65,0.18)]' : 'border-line'}`}>
					<div class="flex items-start justify-between gap-3">
						<h3 class="text-lg font-semibold">{plan.name}</h3>
						{#if plan.highlight}
							<span class="rounded-md bg-brand px-2 py-1 text-xs font-semibold text-black">推荐</span>
						{/if}
					</div>
					<div class="mt-5 text-3xl font-bold">{plan.price}</div>
					<p class="mt-3 font-mono text-xs text-brand">{plan.quota}</p>
					<p class="mt-3 text-sm text-faint">{plan.description}</p>
					<span class="mt-auto pt-5 text-sm text-muted">当前为方案展示</span>
				</article>
			{/each}
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-14">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">model pricing</div>
	<h2 class="mt-2 text-2xl font-bold">模型定价明细</h2>
	<p class="mt-2 text-sm text-faint">明细用于解释输入、输出和 cache 计费维度；实际请求准入和最终扣费以服务端模型目录、账单记录为准。</p>
	<div class="mt-6 overflow-x-auto rounded-lg border border-line bg-card/60">
		<table class="w-full min-w-[920px] text-left text-sm">
			<thead class="bg-white/[0.03] font-mono text-xs text-faint">
				<tr>
					<th class="px-5 py-3 font-medium">分组</th>
					<th class="px-5 py-3 font-medium">模型</th>
					<th class="px-5 py-3 font-medium">模型 ID</th>
					<th class="px-5 py-3 font-medium">输入</th>
					<th class="px-5 py-3 font-medium">输出</th>
					<th class="px-5 py-3 font-medium">Cache</th>
					<th class="px-5 py-3 font-medium">状态</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each modelPrices as price (`${price.group}-${price.modelId}`)}
					<tr>
						<td class="px-5 py-3 text-muted">{price.group}</td>
						<td class="px-5 py-3 font-semibold">{price.model}</td>
						<td class="px-5 py-3 font-mono text-brand">{price.modelId}</td>
						<td class="px-5 py-3">{price.input}</td>
						<td class="px-5 py-3">{price.output}</td>
						<td class="px-5 py-3 text-muted">{price.cache}</td>
						<td class="px-5 py-3 text-faint">{price.status}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto grid max-w-6xl gap-8 px-5 py-14 lg:grid-cols-[1fr_0.9fr]">
		<div>
			<div class="font-mono text-xs uppercase tracking-wider text-brand">vip levels</div>
			<h2 class="mt-2 text-2xl font-bold">VIP 等级</h2>
			<p class="mt-2 text-sm text-faint">VIP 按累计充值和实际运营策略确认，用于支持优先级、采购协助和额度沟通。</p>
			<div class="mt-6 grid gap-3 sm:grid-cols-2">
				{#each vipLevels as level (level[0])}
					<div class="rounded-lg border border-line bg-card/60 p-5">
						<div class="font-mono text-sm text-brand">{level[0]}</div>
						<h3 class="mt-3 text-base font-semibold">{level[1]}</h3>
						<p class="mt-2 text-sm text-faint">{level[2]}</p>
					</div>
				{/each}
			</div>
		</div>
		<div class="rounded-lg border border-brand/40 bg-card/70 p-6">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">enterprise</div>
			<h2 class="mt-3 text-2xl font-bold">企业定制方案</h2>
			<p class="mt-4 text-sm leading-relaxed text-muted">
				面向团队采购、额度预存、发票、专属支持和内部工具接入。企业版不承诺绕过上游限制，容量、可用模型和结算方式需要按实际账号池与运营策略确认。
			</p>
			<ul class="mt-5 space-y-3 text-sm text-faint">
				<li>专属额度和账单口径说明</li>
				<li>团队成员使用建议和 API Key 管理方案</li>
				<li>Codex 当前接入，Claude 家族接入后沿用同一套餐沟通</li>
			</ul>
			<a class="mt-6 inline-flex rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/contact">联系企业方案</a>
		</div>
	</div>
</section>
