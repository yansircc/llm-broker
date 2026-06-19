<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import { fmtCost } from '$lib/format';

	interface AdminBillingSummary {
		users: number;
		active_users: number;
		open_orders: number;
		paid_orders?: number;
		revenue_usd: number;
		revenue_cny?: number;
		credits_usd: number;
		pending_credits_usd?: number;
	}

	let summary = $state<AdminBillingSummary | null>(null);
	let error = $state('');
	let loading = $state(false);
	let lastRefresh = $state('');

	$effect(() => {
		loadSummary();
	});

	async function loadSummary() {
		loading = true;
		error = '';
		try {
			summary = await api<AdminBillingSummary>('/billing/summary');
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message || '加载计费数据失败';
		} finally {
			loading = false;
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">计费运营</div>
		<h1>充值计费</h1>
		<p class="lede">查看客户数量、已支付收入、已发放余额和待支付充值订单。</p>
	</div>
	<div class="page-actions">
		<a class="secondary-btn fit" href="{base}/console/billing/orders">订单</a>
		<button class="link" onclick={loadSummary}>刷新</button>
		<span class="muted mono">{lastRefresh}</span>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">正在加载计费后台...</p>
{:else if summary}
	<div class="metric-grid">
		<MetricCard label="用户" value={summary.users} sub={`${summary.active_users} 个正常`} />
		<MetricCard label="待支付订单" value={summary.open_orders} sub={`${fmtCost(summary.pending_credits_usd ?? 0)} 待入账`} />
		<MetricCard label="已支付订单" value={summary.paid_orders ?? 0} sub={`¥${(summary.revenue_cny ?? 0).toFixed(2)} 实收`} />
		<MetricCard label="已发放额度" value={fmtCost(summary.credits_usd)} sub="支付订单入账额度" />
	</div>
{:else}
	<p class="muted">暂无计费汇总</p>
{/if}
