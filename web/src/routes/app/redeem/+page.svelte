<script lang="ts">
	import { BRAND_SUPPORT_EMAIL } from '$lib/brand';
	import { customerApi } from '$lib/customer-api';
	import type { BillingSummary } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';

	let code = $state('');
	let message = $state('');
	let summary = $state<BillingSummary | null>(null);
	let error = $state('');

	$effect(() => {
		loadSummary();
	});

	async function loadSummary() {
		error = '';
		try {
			summary = await customerApi<BillingSummary>('/billing/summary');
		} catch (e: any) {
			error = e.message || 'failed to load balance';
		}
	}

	function redeem() {
		message = '兑换码后端尚未接入；本页当前仅保持视觉入口。';
	}
</script>

<div class="mb-8">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">redeem</div>
	<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">兑换码</h1>
	<p class="mt-2 text-sm text-muted">使用兑换码充值额度或领取活动奖励。</p>
</div>

<div class="grid gap-6 lg:grid-cols-[1fr_0.9fr]">
	<section class="rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">兑换码</h2>
		{#if error}
			<p class="mt-3 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
		{/if}
		<div class="mt-4 grid gap-3 sm:grid-cols-2">
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="text-xs text-faint">当前余额</div>
				<div class="mt-2 font-mono text-2xl font-bold text-brand">{summary ? fmtCost(summary.balance_usd) : '-'}</div>
			</div>
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="text-xs text-faint">并发数</div>
				<div class="mt-2 font-mono text-2xl font-bold">默认</div>
			</div>
		</div>
		<div class="mt-5 grid gap-3 sm:grid-cols-[1fr_auto]">
			<input class="h-11 rounded-md border border-line bg-black/30 px-3 font-mono text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="请输入兑换码" bind:value={code}>
			<button class="h-11 rounded-md bg-brand px-6 text-sm font-semibold text-black" type="button" onclick={redeem}>兑换</button>
		</div>
		{#if message}
			<p class="mt-4 rounded-md border border-amber-400/30 bg-amber-400/10 px-4 py-3 text-sm text-amber-200">{message}</p>
		{/if}
	</section>
	<section class="rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">关于兑换码</h2>
		<ul class="mt-4 space-y-3 text-sm text-muted">
			<li>兑换码可用于活动奖励、客服补偿或批量发放额度。</li>
			<li>每个兑换码只能使用一次，兑换后额度应自动入账。</li>
			<li>本分支需要后端兑换表和账本写入入口后才能启用。</li>
			<li>兑换异常请联系 {BRAND_SUPPORT_EMAIL}。</li>
		</ul>
	</section>
</div>
