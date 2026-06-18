<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
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

<span class="refresh"><button class="link" onclick={loadUser}>[refresh]</button></span>
<h2>billing user</h2>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading billing user...</p>
{:else if user}
	<div class="bar">
		<span>{user.email}</span>
		<span>status {user.status}</span>
		<span>plan {user.plan ?? '-'}</span>
		<span>credits {fmtCost(user.credits_usd)}</span>
		<span>usage {fmtCost(user.usage_usd)}</span>
	</div>

	<h2>orders</h2>
	{#if user.orders.length === 0}
		<p class="muted">no orders</p>
	{:else}
		<table>
			<thead>
				<tr><th>order</th><th>status</th><th class="num">amount</th><th>created</th><th>paid</th></tr>
			</thead>
			<tbody>
				{#each user.orders as order (order.id)}
					<tr>
						<td>{order.id}</td>
						<td>{order.status}</td>
						<td class="num">{fmtCost(order.amount_usd)}</td>
						<td>{fmtDate(order.created_at)}</td>
						<td>{order.paid_at ? fmtDate(order.paid_at) : '-'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
{:else}
	<p class="muted">no billing user</p>
{/if}
