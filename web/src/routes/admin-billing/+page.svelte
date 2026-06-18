<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import { fmtCost } from '$lib/format';

	interface AdminBillingSummary {
		users: number;
		active_users: number;
		open_orders: number;
		revenue_usd: number;
		credits_usd: number;
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
			error = e.message || 'failed to load billing';
		} finally {
			loading = false;
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">billing ops</div>
		<h1>Billing Admin</h1>
		<p class="lede">Customer count, paid revenue, issued credits, and open recharge orders.</p>
	</div>
	<div class="page-actions">
		<a class="secondary-btn fit" href="{base}/admin-billing/orders">orders</a>
		<button class="link" onclick={loadSummary}>refresh</button>
		<span class="muted mono">{lastRefresh}</span>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading billing admin...</p>
{:else if summary}
	<div class="metric-grid">
		<MetricCard label="users" value={summary.users} sub={`${summary.active_users} active`} />
		<MetricCard label="open orders" value={summary.open_orders} sub="pending payment" />
		<MetricCard label="revenue" value={fmtCost(summary.revenue_usd)} sub="paid order total" />
		<MetricCard label="credits" value={fmtCost(summary.credits_usd)} sub="ledger credits issued" />
	</div>
{:else}
	<p class="muted">no billing summary</p>
{/if}
