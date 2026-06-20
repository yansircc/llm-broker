<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { BillingLedgerResponse } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';

	let data = $state<BillingLedgerResponse | null>(null);
	let error = $state('');
	let loading = $state(false);
	let kindFilter = $state<'all' | 'credit' | 'debit'>('all');
	const kindFilters: { value: 'all' | 'credit' | 'debit'; label: string }[] = [
		{ value: 'all', label: '全部' },
		{ value: 'credit', label: '充值' },
		{ value: 'debit', label: '消费' }
	];
	const entries = $derived(data?.entries ?? []);
	const credits = $derived(entries.filter((entry) => entry.amount_usd > 0).reduce((sum, entry) => sum + entry.amount_usd, 0));
	const debits = $derived(entries.filter((entry) => entry.amount_usd < 0).reduce((sum, entry) => sum + Math.abs(entry.amount_usd), 0));
	const filteredEntries = $derived(
		entries.filter((entry) => {
			if (kindFilter === 'credit') return entry.amount_usd > 0;
			if (kindFilter === 'debit') return entry.amount_usd < 0;
			return true;
		})
	);

	$effect(() => {
		loadLedger();
	});

	async function loadLedger() {
		loading = true;
		error = '';
		try {
			data = await customerApi<BillingLedgerResponse>('/billing/ledger?limit=100');
		} catch (e: any) {
			error = e.message || 'failed to load balance history';
		} finally {
			loading = false;
		}
	}

	function label(kind: string) {
		switch (kind) {
			case 'payment_credit': return '充值';
			case 'usage_debit': return '消费';
			case 'referral_signup_credit': return '注册奖励';
			case 'referral_paid_credit': return '邀请奖励';
			case 'admin_adjustment': return '调整';
			default: return kind;
		}
	}

	function setKindFilter(value: 'all' | 'credit' | 'debit') {
		kindFilter = value;
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">ledger</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">额度记录</h1>
		<p class="mt-2 text-sm text-muted">充值、邀请奖励和 token 消费的完整账本流水。</p>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadLedger}>刷新</button>
</div>

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else}
	<div class="mb-6 grid gap-4 sm:grid-cols-3">
		<div class="relative rounded-lg border border-line bg-card/70 p-5">
			<Icon name="arrow-down-left" size={20} class="absolute right-4 top-4 text-brand" />
			<div class="text-sm text-faint">累计充值</div>
			<div class="mt-3 font-mono text-2xl font-bold text-brand">{fmtCost(credits)}</div>
			<div class="mt-1 text-xs text-faint">充值与兑换合计</div>
		</div>
		<div class="relative rounded-lg border border-line bg-card/70 p-5">
			<Icon name="arrow-up-right" size={20} class="absolute right-4 top-4 text-faint" />
			<div class="text-sm text-faint">累计消费</div>
			<div class="mt-3 font-mono text-2xl font-bold">{fmtCost(debits)}</div>
			<div class="mt-1 text-xs text-faint">近 30 天按天聚合</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">净额度</div>
			<div class="mt-3 font-mono text-2xl font-bold">{fmtCost(credits - debits)}</div>
			<div class="mt-1 text-xs text-faint">充值 - 消费</div>
		</div>
	</div>
	<div class="mb-4 inline-flex rounded-md border border-line bg-black/20 p-1">
		{#each kindFilters as item}
			<button class={`rounded px-4 py-1.5 text-sm ${kindFilter === item.value ? 'bg-brand font-semibold text-black' : 'text-slate-300 hover:bg-white/[0.04]'}`} onclick={() => setKindFilter(item.value)}>{item.label}</button>
		{/each}
	</div>
	{#if !filteredEntries.length}
		<div class="rounded-lg border border-dashed border-line bg-card/30 px-6 py-16 text-center text-sm text-faint">暂无额度记录</div>
	{:else}
		<div class="overflow-x-auto rounded-lg border border-line bg-card/60">
			<table class="w-full min-w-[680px] text-left text-sm">
				<thead class="bg-white/[0.03] font-mono text-xs text-faint">
					<tr>
						<th class="px-5 py-3 font-medium">类型</th>
						<th class="px-5 py-3 font-medium">详情</th>
						<th class="px-5 py-3 font-medium">金额</th>
						<th class="px-5 py-3 font-medium">时间</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each filteredEntries as entry}
						<tr class="hover:bg-white/[0.02]">
							<td class="px-5 py-3">{label(entry.kind)}</td>
							<td class="px-5 py-3">{entry.description || '-'}</td>
							<td class={`px-5 py-3 font-mono ${entry.amount_usd >= 0 ? 'text-brand' : 'text-red-300'}`}>{fmtCost(entry.amount_usd)}</td>
							<td class="px-5 py-3">{fmtDate(entry.created_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}
