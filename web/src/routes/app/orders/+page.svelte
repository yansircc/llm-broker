<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { PaymentOrder } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';

	let orders = $state<PaymentOrder[]>([]);
	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		loadOrders();
	});

	async function loadOrders() {
		loading = true;
		error = '';
		try {
			orders = await customerApi<PaymentOrder[]>('/payments/orders?limit=100');
		} catch (e: any) {
			error = e.message || 'failed to load orders';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">orders</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">我的订单</h1>
		<p class="mt-2 text-sm text-muted">查看充值订单和支付状态。</p>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadOrders}>刷新</button>
</div>

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else if orders.length === 0}
	<div class="rounded-lg border border-dashed border-line bg-card/30 px-6 py-16 text-center text-sm text-faint">暂无订单记录</div>
{:else}
	<div class="overflow-x-auto rounded-lg border border-line bg-card/60">
		<table class="w-full min-w-[760px] text-left text-sm">
			<thead class="bg-white/[0.03] font-mono text-xs text-faint">
				<tr>
					<th class="px-5 py-3 font-medium">订单号</th>
					<th class="px-5 py-3 font-medium">额度</th>
					<th class="px-5 py-3 font-medium">金额</th>
					<th class="px-5 py-3 font-medium">状态</th>
					<th class="px-5 py-3 font-medium">创建时间</th>
					<th class="px-5 py-3 font-medium">支付时间</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each orders as order}
					<tr class="hover:bg-white/[0.02]">
						<td class="px-5 py-3 font-mono">{order.id}</td>
						<td class="px-5 py-3 font-mono text-brand">{fmtCost(order.amount_usd)}</td>
						<td class="px-5 py-3 font-mono">¥{order.amount_cny?.toFixed(2) ?? '-'}</td>
						<td class="px-5 py-3">
							<span class={`rounded-full border px-2 py-1 text-xs ${order.status === 'paid' ? 'border-brand/30 text-brand' : 'border-amber-400/30 text-amber-300'}`}>{order.status}</span>
						</td>
						<td class="px-5 py-3">{fmtDate(order.created_at)}</td>
						<td class="px-5 py-3">{order.paid_at ? fmtDate(order.paid_at) : '-'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
