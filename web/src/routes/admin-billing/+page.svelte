<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
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

<span class="refresh"><button class="link" onclick={loadSummary}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
<h2>billing admin</h2>
<div class="topnav">
	<a href="{base}/admin-billing/orders">[orders]</a>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading billing admin...</p>
{:else if summary}
	<div class="bar">
		<span>users {summary.users}</span>
		<span>active {summary.active_users}</span>
		<span>open orders {summary.open_orders}</span>
		<span>revenue {fmtCost(summary.revenue_usd)}</span>
		<span>credits {fmtCost(summary.credits_usd)}</span>
	</div>
{:else}
	<p class="muted">no billing summary</p>
{/if}
