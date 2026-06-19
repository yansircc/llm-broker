<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';
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
	const todayTokens = $derived((today?.input_tokens ?? 0) + (today?.output_tokens ?? 0) + (today?.cache_read_tokens ?? 0));
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">账户概览与使用统计</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">仪表盘</h1>
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
	<section class="mb-5 rounded-xl border border-brand/30 bg-brand/[0.06] p-5">
		<div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
			<div>
				<div class="font-mono text-xs text-brand">六月印章活动 · 进行中</div>
				<h2 class="mt-2 text-xl font-semibold">登录赠 1 枚 · 每充 ¥49 得 1 枚 · 邀好友首充各 +1 枚</h2>
				<p class="mt-2 text-sm text-faint">已集 1 / 4 枚，还差 3 枚。盲盒奖励：$20 / $50 / $100 额度随机。</p>
			</div>
			<div class="grid min-w-72 grid-cols-4 gap-2 text-center font-mono">
				{#each [1, 2, 3, 4] as stamp}
					<div class={`rounded-lg border p-3 ${stamp === 1 ? 'border-brand bg-brand text-black' : 'border-line bg-black/25 text-faint'}`}>{stamp}</div>
				{/each}
			</div>
		</div>
		<div class="mt-4 flex flex-col gap-2 sm:flex-row sm:items-center">
			<div class="min-w-0 flex-1 truncate rounded-md border border-line bg-black/25 px-3 py-2 font-mono text-sm text-brand">{referrals?.url ?? 'https://your-domain.example/register?ref=...'}</div>
			<a class="rounded-md border border-line bg-card px-3 py-2 text-sm hover:border-brand/50" href="{base}/app/referrals">我的推广链接</a>
		</div>
	</section>
	<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">余额</div>
			<div class="mt-3 font-mono text-3xl font-bold text-brand">{billing ? fmtCost(billing.balance_usd) : '-'}</div>
			<div class="mt-2 text-xs text-faint">请求时余额需大于 0</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">API 密钥</div>
			<div class="mt-3 font-mono text-3xl font-bold">{keys.length}</div>
			<div class="mt-2 text-xs text-faint">启用: {keys.filter((key) => key.status === 'active').length}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">今日请求</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtNum(today?.requests ?? 0)}</div>
			<div class="mt-2 text-xs text-faint">7 天: {fmtNum(sevenDays?.requests ?? 0)}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">今日 Token</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtNum(todayTokens)}</div>
			<div class="mt-2 text-xs text-faint">输入 + 输出 + 缓存读</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">今日消费</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtCost(today?.cost_usd ?? 0)}</div>
			<div class="mt-2 text-xs text-faint">总消费: {billing ? fmtCost(billing.usage_usd) : '-'}</div>
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
					<h2 class="m-0 text-base font-semibold">分销奖励</h2>
					<p class="mt-1 text-sm text-faint">推广 {BRAND_NAME}，受邀用户首充后获得奖励。</p>
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

	<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
		<div class="mb-4 flex items-center justify-between gap-3">
			<h2 class="m-0 text-base font-semibold">Token 使用趋势（7 天）</h2>
			<a class="text-sm text-brand hover:underline" href="{base}/app/usage">查看明细</a>
		</div>
		<div class="grid gap-3 sm:grid-cols-3">
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="text-xs text-faint">输入 Token</div>
				<div class="mt-2 font-mono text-2xl font-bold">{fmtNum(sevenDays?.input_tokens ?? 0)}</div>
			</div>
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="text-xs text-faint">输出 Token</div>
				<div class="mt-2 font-mono text-2xl font-bold">{fmtNum(sevenDays?.output_tokens ?? 0)}</div>
			</div>
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="text-xs text-faint">缓存读 Token</div>
				<div class="mt-2 font-mono text-2xl font-bold text-brand">{fmtNum(sevenDays?.cache_read_tokens ?? 0)}</div>
			</div>
		</div>
	</section>

	<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">VIP 等级</h2>
		<div class="mt-4 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
			<div>
				<div class="text-sm text-faint">普通 · 1x 倍率</div>
				<div class="mt-1 font-mono text-sm">累计充值 {billing ? fmtCost(billing.credits_usd).replace('$', '¥') : '¥0'}</div>
			</div>
			<div class="grid flex-1 gap-2 sm:grid-cols-5">
				{#each [['普通', '1x'], ['VIP1', '0.98x'], ['VIP2', '0.95x'], ['VIP3', '0.93x'], ['VIP8', '0.88x']] as vip, i}
					<div class={`rounded-md border p-3 text-center text-xs ${i === 0 ? 'border-brand bg-brand text-black' : 'border-line bg-black/20 text-faint'}`}>
						<div class="font-semibold">{vip[0]}</div>
						<div class="font-mono">{vip[1]}</div>
					</div>
				{/each}
			</div>
		</div>
		<p class="mt-3 text-xs text-faint">VIP 等级根据累计充值金额自动升级，倍率越低费用越省。</p>
	</section>

	<section class="mt-6">
		<h2 class="mb-3 text-base font-semibold">快捷操作</h2>
		<div class="grid gap-3 md:grid-cols-3">
			<a class="rounded-lg border border-line bg-card/60 p-4 hover:border-brand/50" href="{base}/app/keys">
				<div class="font-medium">创建 API 密钥</div>
				<p class="mt-1 text-sm text-faint">管理您的 API 访问凭证</p>
			</a>
			<a class="rounded-lg border border-line bg-card/60 p-4 hover:border-brand/50" href="{base}/app/usage">
				<div class="font-medium">查看使用记录</div>
				<p class="mt-1 text-sm text-faint">详细的 API 调用和消费统计</p>
			</a>
			<a class="rounded-lg border border-line bg-card/60 p-4 hover:border-brand/50" href="{base}/app/redeem">
				<div class="font-medium">兑换码</div>
				<p class="mt-1 text-sm text-faint">使用兑换码充值额度</p>
			</a>
		</div>
	</section>
{/if}
