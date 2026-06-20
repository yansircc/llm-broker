<script lang="ts">
	import { BRAND_SUPPORT_EMAIL } from '$lib/brand';
	import { customerApi } from '$lib/customer-api';
	import type { BillingSummary } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';

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
		if (!code.trim()) {
			message = '请输入兑换码。';
			return;
		}
		message = '兑换码无效或已使用，请确认后重试。';
	}
</script>

<div class="mb-8">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">redeem</div>
	<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">兑换码</h1>
	<p class="mt-2 text-sm text-muted">使用兑换码充值余额或激活服务。</p>
	<div class="mt-4 flex gap-6 text-sm text-muted">
		<span>当前余额 <span class="font-mono font-bold text-brand">{summary ? fmtCost(summary.balance_usd) : '-'}</span></span>
		<span class="text-faint">·</span>
		<span>并发数 <span class="font-mono font-bold">默认</span></span>
	</div>
</div>

<div class="max-w-2xl space-y-6">
	<section class="rounded-lg border border-line bg-card/60 p-5">
		<div class="flex items-center gap-2">
			<Icon name="gift" size={16} class="text-brand" />
			<h2 class="m-0 text-base font-semibold">兑换码</h2>
		</div>
		<p class="mt-2 text-sm text-faint">兑换码区分大小写。</p>
		{#if error}
			<p class="mt-3 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
		{/if}
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
			<li>每个兑换码只能使用一次，兑换后额度自动入账。</li>
			<li>兑换码区分大小写，请完整复制粘贴。</li>
			<li>兑换异常请联系 {BRAND_SUPPORT_EMAIL}。</li>
		</ul>
	</section>
</div>
