<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { PaymentOrder } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';

	let orders = $state<PaymentOrder[]>([]);
	let error = $state('');
	let loading = $state(false);
	let refreshing = $state('');
	let search = $state('');
	let statusFilter = $state('all');
	let methodFilter = $state('all');
	const filteredOrders = $derived(
		orders.filter((order) => {
			const q = search.trim().toLowerCase();
			const matchesStatus = statusFilter === 'all' || order.status === statusFilter;
			const matchesSearch = !q || [order.id, order.out_trade_no, order.status].some((value) => String(value ?? '').toLowerCase().includes(q));
			return matchesStatus && matchesSearch;
		})
	);

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

	async function refreshOrder(order: PaymentOrder) {
		refreshing = order.id;
		error = '';
		try {
			const updated = await customerApi<PaymentOrder>(`/payments/orders/${order.id}/refresh`, { method: 'POST' });
			orders = orders.map((item) => (item.id === updated.id ? updated : item));
		} catch (e: any) {
			error = e.message || 'failed to refresh order';
		} finally {
			refreshing = '';
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
		<div class="grid gap-3 border-b border-line p-4 sm:grid-cols-[1fr_160px_160px]">
			<input class="h-10 max-w-sm rounded-md border border-line bg-black/30 px-3 text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="搜索订单号..." bind:value={search}>
			<select class="h-10 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" bind:value={statusFilter}>
				<option value="all">全部状态</option>
				<option value="pending">pending</option>
				<option value="paid">paid</option>
				<option value="expired">expired</option>
				<option value="failed">failed</option>
			</select>
			<select class="h-10 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" bind:value={methodFilter} disabled>
				<option value="all">全部方式</option>
				<option value="wechat">微信</option>
				<option value="alipay">支付宝</option>
				<option value="usdt">USDT</option>
			</select>
		</div>
		<table class="w-full min-w-[760px] text-left text-sm">
			<thead class="bg-white/[0.03] font-mono text-xs text-faint">
				<tr>
					<th class="px-5 py-3 font-medium">订单号</th>
					<th class="px-5 py-3 font-medium">商品</th>
					<th class="px-5 py-3 font-medium">金额</th>
					<th class="px-5 py-3 font-medium">支付方式</th>
					<th class="px-5 py-3 font-medium">状态</th>
					<th class="px-5 py-3 font-medium">创建时间</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each filteredOrders as order}
					<tr class="hover:bg-white/[0.02]">
						<td class="px-5 py-3 font-mono">{order.id}</td>
						<td class="px-5 py-3">{fmtCost(order.amount_usd)} 额度</td>
						<td class="px-5 py-3 font-mono">¥{order.amount_cny?.toFixed(2) ?? '-'}</td>
						<td class="px-5 py-3 text-faint">下单返回</td>
						<td class="px-5 py-3">
							<div class="flex items-center gap-2">
								<span class={`rounded-full border px-2 py-1 text-xs ${order.status === 'paid' ? 'border-brand/30 text-brand' : 'border-amber-400/30 text-amber-300'}`}>{order.status}</span>
								<button class="rounded-md border border-line bg-card px-2 py-1 text-xs hover:border-brand/50 disabled:opacity-50" onclick={() => refreshOrder(order)} disabled={refreshing === order.id || order.status === 'paid'}>
									{refreshing === order.id ? '查' : '刷新'}
								</button>
							</div>
						</td>
						<td class="px-5 py-3">{fmtDate(order.created_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
