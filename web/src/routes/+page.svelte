<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';

	let origin = $state('https://your-domain.example');

	$effect(() => {
		if (browser) origin = window.location.origin;
	});

	const features = [
		['OpenAI-first', '公开产品面向 OpenAI Responses 和 OpenAI-compatible API，适合 Codex 与常见 OpenAI 客户端。'],
		['预付费账本', '人民币充值，USD 额度展示，按模型 token 用量扣费。'],
		['客户 API Key', '每个用户自己创建 relay key，请求按账号余额和权限准入。'],
		['可追溯记录', '订单、额度流水、请求 usage 都可在控制台查看。']
	];

	const steps = ['注册账号', '充值额度', '创建 API Key', '配置客户端'];
</script>

<section class="border-b border-line">
	<div class="mx-auto grid max-w-6xl items-center gap-10 px-5 py-20 lg:grid-cols-[1fr_0.9fr] lg:py-24">
		<div>
			<div class="inline-flex rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 font-mono text-xs text-brand">OpenAI/Codex relay</div>
			<h1 class="mt-6 max-w-3xl text-5xl font-bold leading-tight tracking-tight sm:text-6xl">CDX API 中转服务</h1>
			<p class="mt-5 max-w-2xl text-lg text-muted">为外部用户提供 OpenAI Responses 和兼容 API 转发：账号、API Key、充值、token 计费、订单和邀请奖励都在一个控制台里。</p>
			<div class="mt-8 flex flex-wrap gap-3">
				<a class="rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/app/register">立即注册</a>
				<a class="rounded-md border border-line bg-card px-5 py-3 text-sm hover:border-brand/50" href="{base}/docs">查看接入文档</a>
			</div>
		</div>

		<div class="rounded-xl border border-line bg-[#050505] shadow-2xl">
			<div class="flex items-center gap-2 border-b border-line px-4 py-3 font-mono text-xs text-faint">
				<span class="h-2.5 w-2.5 rounded-full bg-red-400"></span>
				<span class="h-2.5 w-2.5 rounded-full bg-amber-400"></span>
				<span class="h-2.5 w-2.5 rounded-full bg-brand"></span>
				<span class="ml-2">terminal</span>
			</div>
			<div class="space-y-2 p-5 font-mono text-sm">
				<div class="text-brand">$ curl {origin}/v1/responses</div>
				<div class="text-faint">&gt; Authorization: Bearer cdx_live_***</div>
				<div class="text-faint">&gt; model: gpt-5</div>
				<div class="text-brand">&lt; routed: openai/responses</div>
				<div class="text-brand">&lt; charged: input/output tokens</div>
			</div>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-14">
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		{#each [['1 RMB = 1 USD', '默认优惠比例'], ['PAYGO', '额度永不过期'], ['Responses', '主转发表面'], ['7pay', '扫码充值']] as stat}
			<div class="rounded-lg border border-line bg-card/60 p-5 text-center">
				<div class="font-mono text-lg font-semibold text-brand">{stat[0]}</div>
				<div class="mt-1 text-sm text-faint">{stat[1]}</div>
			</div>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">why cdx</div>
		<h2 class="mt-3 text-3xl font-bold tracking-tight">真正可用的 SaaS 控制面</h2>
		<div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each features as feature, i}
				<div class="rounded-lg border border-line bg-card/60 p-5">
					<div class="font-mono text-sm text-brand">0{i + 1}</div>
					<h3 class="mt-3 text-lg font-semibold">{feature[0]}</h3>
					<p class="mt-2 text-sm leading-relaxed text-muted">{feature[1]}</p>
				</div>
			{/each}
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-16">
	<div class="grid gap-8 lg:grid-cols-[0.9fr_1.1fr]">
		<div>
			<div class="font-mono text-xs uppercase tracking-wider text-brand">pricing</div>
			<h2 class="mt-3 text-3xl font-bold tracking-tight">按量充值，用多少扣多少</h2>
			<p class="mt-3 text-muted">价格以 USD 额度展示，充值以 RMB 收款。默认优惠比例为 1 RMB = 1 USD 额度，实际可由后台配置。</p>
			<a class="mt-6 inline-flex rounded-md border border-line bg-card px-4 py-2 text-sm hover:border-brand/50" href="{base}/pricing">查看定价</a>
		</div>
		<div class="grid gap-3 sm:grid-cols-4">
			{#each steps as step, i}
				<div class="rounded-lg border border-line bg-card/60 p-4">
					<div class="font-mono text-sm text-brand">{i + 1}</div>
					<div class="mt-2 font-medium">{step}</div>
				</div>
			{/each}
		</div>
	</div>
</section>

<section class="border-t border-line">
	<div class="mx-auto flex max-w-6xl flex-col gap-4 px-5 py-12 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h2 class="text-2xl font-bold">3 分钟接入 CDX</h2>
			<p class="mt-1 text-sm text-faint">注册账号、充值、创建 API Key，然后替换 base URL。</p>
		</div>
		<a class="rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/app/register">立即注册</a>
	</div>
</section>
