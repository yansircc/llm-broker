<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { fmtCost, fmtDate } from '$lib/format';
	import { statusLabel } from '$lib/admin-i18n';

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
			error = e.message || '加载用户计费失败';
		} finally {
			loading = false;
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">用户计费</div>
		<h1>用户计费</h1>
		<p class="lede">查看单个客户的余额、token 用量和充值订单历史。</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadUser}>刷新</button>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">正在加载用户计费...</p>
{:else if user}
	<div class="metric-grid">
		<MetricCard label="余额" value={fmtCost(user.credits_usd)} sub={user.email} />
		<MetricCard label="用量" value={fmtCost(user.usage_usd)} sub="token 扣费总额" />
		<MetricCard label="套餐" value={user.plan ?? '-'} sub="客户套餐" />
		<MetricCard label="订单" value={user.orders.length} sub={statusLabel(user.status)} />
	</div>

	<div class="section-header">
		<h2>订单</h2>
		<StatusBadge status={user.status} />
	</div>
	{#if user.orders.length === 0}
		<p class="muted">暂无订单</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr><th>订单</th><th>状态</th><th class="num">金额</th><th>创建时间</th><th>支付时间</th></tr>
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
	<p class="muted">暂无用户计费数据</p>
{/if}
