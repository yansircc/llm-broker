<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { BillingSummary, PaymentOrder } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';

	const packs = [
		{ name: '体验', amount: 20, desc: '适合初次体验' },
		{ name: '基础', amount: 50, desc: '日常轻度使用' },
		{ name: '标准', amount: 100, desc: '开发者常用', badge: '热门' },
		{ name: '进阶', amount: 500, desc: '高频使用' },
		{ name: '专业', amount: 1000, desc: '专业开发者' },
		{ name: '企业', amount: 5000, desc: '企业级用量' }
	];
	const methods = [
		{ label: '支付宝', value: 'alipay' },
		{ label: '微信支付', value: 'wxpay' }
	];

	let summary = $state<BillingSummary | null>(null);
	let order = $state<PaymentOrder | null>(null);
	let method = $state('alipay');
	let error = $state('');
	let loading = $state(false);
	let creating = $state(false);

	$effect(() => {
		loadSummary();
	});

	$effect(() => {
		if (!order || order.status !== 'pending') return;
		const timer = setInterval(pollOrder, 2500);
		return () => clearInterval(timer);
	});

	async function loadSummary() {
		loading = true;
		error = '';
		try {
			summary = await customerApi<BillingSummary>('/billing/summary');
		} catch (e: any) {
			error = e.message || 'failed to load billing';
		} finally {
			loading = false;
		}
	}

	async function createPayment(amount: number) {
		creating = true;
		error = '';
		try {
			order = await customerApi<PaymentOrder>('/payments/create', {
				method: 'POST',
				body: JSON.stringify({ amount_usd: amount, type: method })
			});
		} catch (e: any) {
			error = e.message || 'failed to create payment';
		} finally {
			creating = false;
		}
	}

	async function pollOrder() {
		if (!order?.id) return;
		try {
			const next = await customerApi<PaymentOrder>(`/payments/orders/${order.id}`);
			order = { ...order, ...next };
			if (next.status === 'paid') {
				await loadSummary();
			}
		} catch {
			// The payment notification is authoritative; keep the modal open.
		}
	}

	function closeOrder() {
		order = null;
		loadSummary();
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">billing</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">充值</h1>
		<p class="mt-2 text-sm text-muted">充值人民币，获得 USD 额度；模型调用按 token 计费扣除。</p>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadSummary}>刷新</button>
</div>

{#if error}
	<p class="mb-4 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="mb-4 text-sm text-faint">正在加载...</p>
{/if}

{#if summary}
	<div class="mb-6 grid gap-4 sm:grid-cols-3">
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">当前余额</div>
			<div class="mt-3 font-mono text-3xl font-bold text-brand">{fmtCost(summary.balance_usd)}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">累计充值/奖励</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtCost(summary.credits_usd)}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">累计消费</div>
			<div class="mt-3 font-mono text-3xl font-bold">{fmtCost(summary.usage_usd)}</div>
		</div>
	</div>
{/if}

<section class="mb-6 rounded-lg border border-line bg-card/60 p-5">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h2 class="m-0 text-base font-semibold">支付方式</h2>
			<p class="mt-1 text-sm text-faint">7pay 扫码支付，支付成功后自动入账。</p>
		</div>
		<div class="flex gap-2">
			{#each methods as item}
				<button
					class={`rounded-md border px-4 py-2 text-sm ${method === item.value ? 'border-brand bg-brand/10 text-brand' : 'border-line bg-black/20 text-slate-300 hover:border-brand/50'}`}
					onclick={() => (method = item.value)}
				>
					{item.label}
				</button>
			{/each}
		</div>
	</div>
</section>

<div class="mb-5 text-sm text-faint">
	兑换比例：<span class="font-mono text-brand">1 RMB = 1 USD 额度</span>，实际比例以后端配置为准。额度永不过期。
</div>

<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
	{#each packs as pack}
		<section class={`relative flex flex-col rounded-lg border bg-card/70 p-6 ${pack.badge ? 'border-brand/50' : 'border-line'}`}>
			{#if pack.badge}
				<span class="absolute right-4 top-4 rounded-full bg-brand px-2 py-0.5 text-xs font-medium text-black">{pack.badge}</span>
			{/if}
			<h2 class="m-0 text-lg font-semibold">{pack.name}</h2>
			<p class="mt-1 text-sm text-faint">{pack.desc}</p>
			<div class="mt-5 text-3xl font-bold">¥{pack.amount.toLocaleString()}</div>
			<div class="mt-1 font-mono text-sm text-brand">获得 ${pack.amount.toLocaleString()} 额度</div>
			<ul class="mt-5 flex-1 space-y-2 text-sm text-slate-300">
				<li><span class="text-brand">✓</span> 支持 OpenAI Responses</li>
				<li><span class="text-brand">✓</span> 支持 OpenAI compatible endpoint</li>
				<li><span class="text-brand">✓</span> 按实际 token 扣费</li>
			</ul>
			<button
				class="mt-6 h-11 rounded-md bg-brand text-sm font-semibold text-black disabled:opacity-50"
				onclick={() => createPayment(pack.amount)}
				disabled={creating}
			>
				{creating ? '创建订单中...' : '立即充值'}
			</button>
		</section>
	{/each}
</div>

{#if order}
	<div class="fixed inset-0 z-[100] flex items-center justify-center p-4">
		<button class="absolute inset-0 bg-black/75 backdrop-blur-sm" aria-label="关闭支付弹窗" onclick={closeOrder}></button>
		<section class="relative w-full max-w-md rounded-xl border border-line bg-[#0c0c0c] p-7 text-center shadow-2xl">
			<button class="absolute right-4 top-4 rounded-md border border-line bg-card px-2 py-1 text-sm text-faint hover:text-white" onclick={closeOrder}>关闭</button>
			<h2 class="m-0 text-lg font-bold">{method === 'wxpay' ? '微信扫码支付' : '支付宝扫码支付'}</h2>
			<p class="mt-4 text-sm text-faint">请支付精确金额</p>
			<div class="mt-1 font-mono text-4xl font-bold text-brand">¥{order.amount_cny?.toFixed(2) ?? order.amount_usd.toFixed(2)}</div>
			<p class="mt-2 text-sm text-muted">获得 {fmtCost(order.amount_usd)} 额度</p>

			{#if order.status === 'paid'}
				<div class="mx-auto mt-7 flex h-16 w-16 items-center justify-center rounded-xl bg-brand text-3xl text-black">✓</div>
				<h3 class="mt-5 text-xl font-bold">支付成功</h3>
				<p class="mt-2 text-sm text-muted">额度已到账。</p>
				<button class="mt-7 h-11 rounded-md bg-brand px-10 text-sm font-semibold text-black" onclick={closeOrder}>返回</button>
			{:else}
				<div class="mt-6 flex justify-center">
					{#if order.qr_image}
						<img class="h-56 w-56 rounded-lg border border-line bg-white p-2" src={order.qr_image} alt="支付二维码">
					{:else if order.qrcode}
						<div class="flex h-56 w-56 items-center justify-center break-all rounded-lg border border-line bg-white p-3 font-mono text-xs text-black">{order.qrcode}</div>
					{:else}
						<div class="flex h-56 w-56 items-center justify-center rounded-lg border border-line bg-black/30 text-sm text-faint">等待二维码</div>
					{/if}
				</div>
				<p class="mt-5 text-sm text-slate-200">扫码完成支付后会自动确认。</p>
				<p class="mt-2 text-xs text-faint">订单号：<span class="font-mono">{order.id}</span></p>
				<div class="mt-4 flex items-center justify-center gap-2 text-sm text-faint">
					<span class="h-2.5 w-2.5 animate-pulse rounded-full bg-brand"></span>
					等待支付确认中...
				</div>
				{#if order.checkout_url}
					<a class="mt-4 inline-flex h-10 items-center rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" href={order.checkout_url} target="_blank" rel="noreferrer">打开支付页面</a>
				{/if}
			{/if}
			<p class="mt-5 text-xs text-faint">创建时间：{fmtDate(order.created_at)}</p>
		</section>
	</div>
{/if}
