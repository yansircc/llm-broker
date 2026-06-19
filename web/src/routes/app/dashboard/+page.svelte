<script lang="ts">
	import { base } from '$app/paths';
	import { customerApi } from '$lib/customer-api';
	import type {
		BillingSummary,
		CustomerApiKey,
		CustomerMe,
		CustomerUsageResponse,
		PaymentOrder,
		ReferralSummary
	} from '$lib/customer-types';
	import { fmtCost, fmtDate, fmtNum } from '$lib/format';

	let me = $state<CustomerMe | null>(null);
	let keys = $state<CustomerApiKey[]>([]);
	let billing = $state<BillingSummary | null>(null);
	let referrals = $state<ReferralSummary | null>(null);
	let usage = $state<CustomerUsageResponse | null>(null);
	let orders = $state<PaymentOrder[]>([]);
	let error = $state('');
	let loading = $state(true);

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		loading = true;
		error = '';
		try {
			const [meResult, keyResult, billingResult, referralResult, usageResult, orderResult] = await Promise.all([
				customerApi<CustomerMe>('/me'),
				customerApi<CustomerApiKey[]>('/keys').catch(() => []),
				customerApi<BillingSummary>('/billing/summary').catch(() => null),
				customerApi<ReferralSummary>('/referrals').catch(() => null),
				customerApi<CustomerUsageResponse>('/usage?range=7d&limit=5').catch(() => null),
				customerApi<PaymentOrder[]>('/payments/orders?limit=5').catch(() => [])
			]);
			me = meResult;
			keys = keyResult;
			billing = billingResult;
			referrals = referralResult;
			usage = usageResult;
			orders = orderResult;
		} catch (e: any) {
			error = e.message || 'failed to load dashboard';
		} finally {
			loading = false;
		}
	}

	const today = $derived(usage?.periods?.find((p) => p.label === 'today'));
	const sevenDays = $derived(usage?.periods?.find((p) => p.label === '7 days'));
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">customer console</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">仪表盘</h1>
		<p class="mt-2 text-sm text-muted">账户余额、API Key、Token 消耗和邀请奖励概览。</p>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadAll}>刷新</button>
</div>

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else}
	{#if billing?.low_balance}
		<p class="mb-5 rounded-md border border-amber-400/30 bg-amber-400/10 px-4 py-3 text-sm text-amber-200">
			余额已低于 ${billing.low_balance_threshold_usd?.toFixed(2) ?? '5.00'}，建议及时充值，避免请求被拒绝。
		</p>
	{/if}
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">余额</div>
			<div class="mt-3 font-mono text-3xl font-bold text-brand">{billing ? fmtCost(billing.balance_usd) : '-'}</div>
			<div class="mt-2 text-xs text-faint">请求时余额需大于 0</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">今日请求</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtNum(today?.requests ?? 0)}</div>
			<div class="mt-2 text-xs text-faint">7 天: {fmtNum(sevenDays?.requests ?? 0)}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">今日消费</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtCost(today?.cost_usd ?? 0)}</div>
			<div class="mt-2 text-xs text-faint">总消费: {billing ? fmtCost(billing.usage_usd) : '-'}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">API Key</div>
			<div class="mt-3 font-mono text-3xl font-bold">{keys.length}</div>
			<div class="mt-2 text-xs text-faint">邀请注册: {referrals?.signups ?? 0}</div>
		</div>
	</div>

	<div class="mt-6 grid gap-4 lg:grid-cols-2">
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<div class="flex items-center justify-between gap-3">
				<div>
					<h2 class="m-0 text-base font-semibold">账户信息</h2>
					<p class="mt-1 text-sm text-faint">{me?.user.email}</p>
				</div>
				<span class="rounded-full border border-brand/40 bg-brand/10 px-2.5 py-1 text-xs text-brand">{me?.user.status}</span>
			</div>
			<div class="mt-5 grid gap-3 text-sm">
				<div class="flex justify-between gap-4 border-b border-line-soft pb-2">
					<span class="text-faint">累计充值/奖励</span>
					<span class="font-mono">{billing ? fmtCost(billing.credits_usd) : '-'}</span>
				</div>
				<div class="flex justify-between gap-4 border-b border-line-soft pb-2">
					<span class="text-faint">7 天 Token</span>
					<span class="font-mono">{fmtNum((sevenDays?.input_tokens ?? 0) + (sevenDays?.output_tokens ?? 0))}</span>
				</div>
				<div class="flex justify-between gap-4">
					<span class="text-faint">创建时间</span>
					<span class="font-mono">{me?.user.created_at ? fmtDate(me.user.created_at) : '-'}</span>
				</div>
			</div>
		</section>

		<section class="rounded-lg border border-brand/20 bg-brand/[0.04] p-5">
			<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
				<div>
					<h2 class="m-0 text-base font-semibold">邀请奖励</h2>
					<p class="mt-1 text-sm text-faint">受邀方注册即得奖励，邀请方在受邀方首次付费后获得奖励。</p>
				</div>
				<a href="{base}/app/referrals" class="rounded-md border border-line bg-card px-3 py-2 text-sm hover:border-brand/50">查看</a>
			</div>
			<div class="mt-4 break-all rounded-md border border-line bg-black/30 p-3 font-mono text-sm text-brand">{referrals?.url ?? '-'}</div>
		</section>
	</div>

	<div class="mt-6 grid gap-4 lg:grid-cols-2">
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="m-0 text-base font-semibold">最近订单</h2>
				<a class="text-sm text-brand hover:underline" href="{base}/app/orders">全部</a>
			</div>
			{#if orders.length === 0}
				<p class="text-sm text-faint">暂无订单</p>
			{:else}
				<div class="space-y-3">
					{#each orders as order}
						<div class="flex items-center justify-between gap-4 rounded-md border border-line bg-black/20 p-3 text-sm">
							<div class="min-w-0">
								<div class="truncate font-mono">{order.id}</div>
								<div class="text-xs text-faint">{fmtDate(order.created_at)}</div>
							</div>
							<div class="text-right">
								<div class="font-mono">{fmtCost(order.amount_usd)}</div>
								<div class="text-xs text-faint">{order.status}</div>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="rounded-lg border border-line bg-card/60 p-5">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="m-0 text-base font-semibold">模型分布</h2>
				<a class="text-sm text-brand hover:underline" href="{base}/app/usage">使用记录</a>
			</div>
			{#if !usage?.model_usage?.length}
				<p class="text-sm text-faint">暂无调用数据</p>
			{:else}
				<div class="space-y-3">
					{#each usage.model_usage.slice(0, 5) as row}
						<div>
							<div class="flex justify-between gap-3 text-sm">
								<span class="truncate">{row.model}</span>
								<span class="font-mono text-brand">{fmtCost(row.cost_usd)}</span>
							</div>
							<div class="mt-1 h-1.5 overflow-hidden rounded-full bg-line">
								<div class="h-full rounded-full bg-brand" style={`width: ${Math.min(100, Math.max(6, row.requests * 12))}%`}></div>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>
	</div>
{/if}
