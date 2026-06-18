<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { fmtCost, fmtDate } from '$lib/format';

	interface AdminBillingUser {
		id: string;
		email: string;
		status: string;
		plan?: string;
		credits_usd: number;
		usage_usd: number;
		orders: Array<{
			id: string;
			status: string;
			amount_usd: number;
			created_at: string;
			paid_at?: string | null;
		}>;
	}

	let user = $state<AdminBillingUser | null>(null);
	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		loadUser();
	});

	async function loadUser() {
		loading = true;
		error = '';
		try {
			user = await api<AdminBillingUser>(`/billing/users/${$page.params.id}`);
		} catch (e: any) {
			error = e.message || 'failed to load billing user';
		} finally {
			loading = false;
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">billing user</div>
		<h1>User Billing</h1>
		<p class="lede">Per-customer credits, token usage, and recharge order history.</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadUser}>refresh</button>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading billing user...</p>
{:else if user}
	<div class="metric-grid">
		<MetricCard label="credits" value={fmtCost(user.credits_usd)} sub={user.email} />
		<MetricCard label="usage" value={fmtCost(user.usage_usd)} sub="token debit total" />
		<MetricCard label="plan" value={user.plan ?? '-'} sub="customer plan" />
		<MetricCard label="orders" value={user.orders.length} sub={user.status} />
	</div>

	<div class="section-header">
		<h2>Orders</h2>
		<StatusBadge status={user.status} />
	</div>
	{#if user.orders.length === 0}
		<p class="muted">no orders</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr><th>order</th><th>status</th><th class="num">amount</th><th>created</th><th>paid</th></tr>
				</thead>
				<tbody>
					{#each user.orders as order (order.id)}
						<tr>
							<td class="mono">{order.id}</td>
							<td><StatusBadge status={order.status} /></td>
							<td class="num">{fmtCost(order.amount_usd)}</td>
							<td>{fmtDate(order.created_at)}</td>
							<td>{order.paid_at ? fmtDate(order.paid_at) : '-'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{:else}
	<p class="muted">no billing user</p>
{/if}
