<script lang="ts">
	import { api } from '$lib/api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { fmtCost, fmtDate } from '$lib/format';
	import { providerLabel } from '$lib/admin-i18n';

	interface AdminBillingOrder {
		id: string;
		user_id: string;
		user_email: string;
		status: string;
		amount_usd: number;
		amount_cny?: number;
		provider?: string;
		created_at: string;
		paid_at?: string | null;
	}

	let orders = $state<AdminBillingOrder[]>([]);
	let error = $state('');
	let loading = $state(false);
	let refreshing = $state('');
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
			error = e.message || '加载订单失败';
		} finally {
			loading = false;
		}
	}

	async function refreshOrder(order: AdminBillingOrder) {
		refreshing = order.id;
		error = '';
		try {
			const updated = await api<AdminBillingOrder>(`/billing/orders/${order.id}/refresh`, { method: 'POST' });
			orders = orders.map((item) => (item.id === updated.id ? updated : item));
		} catch (e: any) {
			error = e.message || '查询订单失败';
		} finally {
			refreshing = '';
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">支付</div>
		<h1>充值订单</h1>
		<p class="lede">用户端计费页面创建的充值订单。</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadOrders}>刷新</button>
		<span class="muted mono">{lastRefresh}</span>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">正在加载订单...</p>
{:else if orders.length === 0}
	<p class="muted">暂无订单</p>
{:else}
	<div class="table-wrap">
		<table>
			<thead>
				<tr><th>订单</th><th>用户</th><th>状态</th><th class="num">额度</th><th class="num">收款</th><th>渠道</th><th>创建时间</th><th>支付时间</th><th>操作</th></tr>
			</thead>
			<tbody>
				{#each orders as order (order.id)}
					<tr>
						<td class="mono">{order.id}</td>
						<td>{order.user_email || order.user_id}</td>
						<td><StatusBadge status={order.status} /></td>
						<td class="num">{fmtCost(order.amount_usd)}</td>
						<td class="num">¥{order.amount_cny?.toFixed(2) ?? '-'}</td>
						<td>{providerLabel(order.provider)}</td>
						<td>{fmtDate(order.created_at)}</td>
						<td>{order.paid_at ? fmtDate(order.paid_at) : '-'}</td>
						<td>
							<button class="secondary-btn fit" onclick={() => refreshOrder(order)} disabled={refreshing === order.id || order.status === 'paid'}>
								{refreshing === order.id ? '查询中' : '查询'}
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
