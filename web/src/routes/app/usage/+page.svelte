<script lang="ts">
	import { BRAND_NAME } from '$lib/brand';
	import { customerApi } from '$lib/customer-api';
	import type { CustomerUsageResponse } from '$lib/customer-types';
	import { fmtCost, fmtDate, fmtNum } from '$lib/format';

	let usage = $state<CustomerUsageResponse | null>(null);
	let range = $state('7d');
	let model = $state('');
	let keyFilter = $state('');
	let groupFilter = $state('all');
	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		loadUsage();
	});

	async function loadUsage() {
		loading = true;
		error = '';
		try {
			const qs = new URLSearchParams({ range, limit: '100' });
			if (model.trim()) qs.set('model', model.trim());
			usage = await customerApi<CustomerUsageResponse>(`/usage?${qs.toString()}`);
		} catch (e: any) {
			error = e.message || 'failed to load usage';
		} finally {
			loading = false;
		}
	}

	const current = $derived(usage?.periods?.find((p) => p.label === (range === '30d' ? '30 days' : range === 'today' ? 'today' : '7 days')));
	const avgDuration = $derived(
		usage?.logs?.length ? Math.round(usage.logs.reduce((sum, log) => sum + log.duration_ms, 0) / usage.logs.length) : 0
	);
	const cacheCreateTokens = $derived(usage?.logs?.reduce((sum, log) => sum + log.cache_create_tokens, 0) ?? 0);
	const filteredLogs = $derived(
		(usage?.logs ?? []).filter((log) => {
			const q = keyFilter.trim().toLowerCase();
			if (!q) return true;
			return [log.api_key_name, log.api_key_id].some((value) => String(value ?? '').toLowerCase().includes(q));
		})
	);

	function exportCSV() {
		if (!filteredLogs.length) return;
		const headers = ['request_id', 'api_key', 'model', 'input_tokens', 'output_tokens', 'cache_read_tokens', 'cache_create_tokens', 'cost_usd', 'duration_ms', 'status', 'created_at'];
		const rows = filteredLogs.map((log) => [
			log.request_id || String(log.id),
			log.api_key_name || log.api_key_id || '',
			log.model || '',
			String(log.input_tokens),
			String(log.output_tokens),
			String(log.cache_read_tokens),
			String(log.cache_create_tokens),
			String(log.cost_usd),
			String(log.duration_ms),
			log.status,
			log.created_at
		]);
		const csv = [headers, ...rows].map((row) => row.map((cell) => `"${cell.replaceAll('"', '""')}"`).join(',')).join('\n');
		const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${BRAND_NAME.toLowerCase()}-usage-${range}.csv`;
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">usage</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">使用记录</h1>
		<p class="mt-2 text-sm text-muted">查看每次 API 调用、Token、费用和耗时。</p>
	</div>
	<div class="flex gap-2">
		<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={exportCSV} disabled={!filteredLogs.length}>导出 CSV</button>
		<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadUsage}>刷新</button>
	</div>
</div>

<div class="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
	<div class="rounded-lg border border-line bg-card/70 p-5">
		<div class="text-sm text-faint">总请求数</div>
		<div class="mt-3 font-mono text-3xl font-bold">{fmtNum(current?.requests ?? 0)}</div>
	</div>
	<div class="rounded-lg border border-line bg-card/70 p-5">
		<div class="text-sm text-faint">总 Token</div>
		<div class="mt-3 font-mono text-3xl font-bold">{fmtNum((current?.input_tokens ?? 0) + (current?.output_tokens ?? 0))}</div>
	</div>
	<div class="rounded-lg border border-line bg-card/70 p-5">
		<div class="text-sm text-faint">总消费</div>
		<div class="mt-3 font-mono text-3xl font-bold text-brand">{fmtCost(current?.cost_usd ?? 0)}</div>
	</div>
	<div class="rounded-lg border border-line bg-card/70 p-5">
		<div class="text-sm text-faint">平均耗时</div>
		<div class="mt-3 font-mono text-3xl font-bold">{fmtNum(avgDuration)} ms</div>
	</div>
</div>

<div class="mb-5 rounded-lg border border-line bg-card/50 p-4 font-mono text-sm text-faint">
	输入: {fmtNum(current?.input_tokens ?? 0)} / 输出: {fmtNum(current?.output_tokens ?? 0)} / 缓存读: {fmtNum(current?.cache_read_tokens ?? 0)} / 缓存写: {fmtNum(cacheCreateTokens)}
</div>

<section class="mb-5 rounded-lg border border-line bg-card/60 p-4">
	<div class="grid gap-3 lg:grid-cols-[auto_160px_1fr_1fr_auto] lg:items-center">
		<div class="inline-flex rounded-md border border-line bg-black/20 p-1">
			{#each [{ value: 'today', label: '今天' }, { value: '7d', label: '近 7 天' }, { value: '30d', label: '近 30 天' }] as item}
				<button
					class={`rounded px-4 py-1.5 text-sm ${range === item.value ? 'bg-brand font-semibold text-black' : 'text-slate-300 hover:bg-white/[0.04]'}`}
					onclick={() => (range = item.value)}
				>
					{item.label}
				</button>
			{/each}
		</div>
		<select class="h-10 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" bind:value={groupFilter} disabled>
			<option value="all">全部分组</option>
			<option value="default">默认分组</option>
		</select>
		<input class="h-10 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" placeholder="按密钥过滤" bind:value={keyFilter}>
		<input class="h-10 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" placeholder="按模型过滤，如 gpt-5" bind:value={model}>
		<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadUsage}>应用</button>
	</div>
</section>

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else if !filteredLogs.length}
	<div class="rounded-lg border border-dashed border-line bg-card/30 px-6 py-16 text-center text-sm text-faint">
		暂无使用记录。输入: 0 / 输出: 0 / 缓存读: 0 / 缓存写: 0
	</div>
{:else}
	<div class="overflow-x-auto rounded-lg border border-line bg-card/60">
		<table class="w-full min-w-[820px] text-left text-sm">
			<thead class="bg-white/[0.03] font-mono text-xs text-faint">
				<tr>
					<th class="px-5 py-3 font-medium">API 密钥</th>
					<th class="px-5 py-3 font-medium">模型</th>
					<th class="px-5 py-3 font-medium">Token</th>
					<th class="px-5 py-3 font-medium">费用</th>
					<th class="px-5 py-3 font-medium">首 Token</th>
					<th class="px-5 py-3 font-medium">耗时</th>
					<th class="px-5 py-3 font-medium">时间</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-line">
				{#each filteredLogs as log}
					<tr class="hover:bg-white/[0.02]">
						<td class="px-5 py-3">{log.api_key_name || log.api_key_id || '-'}</td>
						<td class="px-5 py-3">{log.model || '-'}</td>
						<td class="px-5 py-3 font-mono">
							{fmtNum(log.input_tokens + log.output_tokens)}
							<span class="text-faint">({fmtNum(log.input_tokens)} / {fmtNum(log.output_tokens)} / 缓存读 {fmtNum(log.cache_read_tokens)})</span>
						</td>
						<td class="px-5 py-3 font-mono text-brand">{fmtCost(log.cost_usd)}</td>
						<td class="px-5 py-3 font-mono text-faint">-</td>
						<td class="px-5 py-3 font-mono">{log.duration_ms} ms</td>
						<td class="px-5 py-3">{fmtDate(log.created_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
