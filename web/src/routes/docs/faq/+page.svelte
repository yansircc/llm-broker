<script lang="ts">
	import { base } from '$app/paths';

	const faqs = [
		['充值后得到什么？', '得到 USD 额度。页面显示 USD，支付按 RMB 收款，默认比例为 1 RMB = 1 USD 额度。'],
		['余额为 0 能请求吗？', '不能。请求进入 relay 前余额必须大于 0。已准入请求完成后可能把余额扣到短暂负数。'],
		['支持哪些接口？', '主接口是 OpenAI Responses，同时保留 OpenAI compatible 兼容层。'],
		['邀请奖励什么时候发？', '受邀方注册后自动获得奖励；邀请方在受邀方首次成功付费后获得奖励。'],
		['为什么没有月卡？', '当前 MVP 只做 PAYGO 按量余额，月卡每日额度是另一套余额模型，后续再单独设计。']
	];
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<header class="border-b border-line">
		<div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
			<a href="{base}/" class="font-semibold">CDX</a>
			<a class="rounded-md border border-line px-3 py-2 text-sm hover:border-brand/50" href="{base}/docs">文档</a>
		</div>
	</header>

	<section class="mx-auto max-w-4xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">faq</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight">常见问题</h1>
		<div class="mt-8 space-y-4">
			{#each faqs as faq}
				<section class="rounded-lg border border-line bg-card/60 p-5">
					<h2 class="text-lg font-semibold">{faq[0]}</h2>
					<p class="mt-2 text-sm leading-relaxed text-muted">{faq[1]}</p>
				</section>
			{/each}
		</div>
	</section>
</main>
