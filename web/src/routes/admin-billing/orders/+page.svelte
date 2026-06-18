<script lang="ts">
	import { api } from '$lib/api';
	import { fmtCost, fmtDate } from '$lib/format';

	interface AdminBillingOrder {
		id: string;
		user_id: string;
		user_email: string;
		status: string;
		amount_usd: number;
		provider?: string;
		created_at: string;
		paid_at?: string | null;
	}

	let orders = $state<AdminBillingOrder[]>([]);
	let error = $state('');
	let loading = $state(false);
	let lastRefresh = $state('');

	$effect(() => {
		loadOrders();
	});

	async function loadOrders() {
		loading = true;
		error = '';
		try {
			orders = await api<AdminBillingOrder[]>('/billing/orders');
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message || 'failed to load orders';
		} finally {
			loading = false;
		}
	}
</script>

<span class="refresh"><button class="link" onclick={loadOrders}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
<h2>billing orders</h2>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading orders...</p>
{:else if orders.length === 0}
	<p class="muted">no orders</p>
{:else}
	<table>
		<thead>
			<tr><th>order</th><th>user</th><th>status</th><th class="num">amount</th><th>provider</th><th>created</th><th>paid</th></tr>
		</thead>
		<tbody>
			{#each orders as order (order.id)}
				<tr>
					<td>{order.id}</td>
					<td>{order.user_email || order.user_id}</td>
					<td>{order.status}</td>
					<td class="num">{fmtCost(order.amount_usd)}</td>
					<td>{order.provider ?? '-'}</td>
					<td>{fmtDate(order.created_at)}</td>
					<td>{order.paid_at ? fmtDate(order.paid_at) : '-'}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}
