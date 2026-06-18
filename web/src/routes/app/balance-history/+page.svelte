<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { BillingLedgerResponse } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';

	let data = $state<BillingLedgerResponse | null>(null);
	let error = $state('');
	let loading = $state(false);

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
{:else if !data?.entries?.length}
	<div class="rounded-lg border border-dashed border-line bg-card/30 px-6 py-16 text-center text-sm text-faint">暂无额度记录</div>
{:else}
	<div class="overflow-x-auto rounded-lg border border-line bg-card/60">
		<table class="w-full min-w-[760px] text-left text-sm">
			<thead class="bg-white/[0.03] font-mono text-xs text-faint">
				<tr>
					<th class="px-5 py-3 font-medium">类型</th>
					<th class="px-5 py-3 font-medium">金额</th>
					<th class="px-5 py-3 font-medium">详情</th>
					<th class="px-5 py-3 font-medium">来源</th>
					<th class="px-5 py-3 font-medium">时间</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each data.entries as entry}
					<tr class="hover:bg-white/[0.02]">
						<td class="px-5 py-3">{label(entry.kind)}</td>
						<td class={`px-5 py-3 font-mono ${entry.amount_usd >= 0 ? 'text-brand' : 'text-red-300'}`}>{fmtCost(entry.amount_usd)}</td>
						<td class="px-5 py-3">{entry.description || '-'}</td>
						<td class="px-5 py-3 font-mono text-xs text-faint">{entry.source_type}:{entry.source_id}</td>
						<td class="px-5 py-3">{fmtDate(entry.created_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
