<script lang="ts">
	import { api } from '$lib/api';
	import type { CapacityData } from '$lib/admin-types';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { fmtDate, fmtNum } from '$lib/format';
	import { providerLabel } from '$lib/admin-i18n';

	let data = $state<CapacityData | null>(null);
	let error = $state('');
	let loading = $state(false);
	let lastRefresh = $state('');

	$effect(() => {
		loadCapacity();
	});

	async function loadCapacity() {
		loading = true;
		error = '';
		try {
			data = await api<CapacityData>('/capacity');
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message || '加载容量数据失败';
		} finally {
			loading = false;
		}
	}

	function failureRate(requests: number, failures: number) {
		if (!requests) return '0%';
		return `${Math.round((failures / requests) * 100)}%`;
	}

	function availability(native: boolean, compat: boolean) {
		if (native && compat) return 'Responses / Compat';
		if (native) return 'Responses';
		if (compat) return 'Compat';
		return '不可用';
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">容量</div>
		<h1>账号池容量观测</h1>
		<p class="lede">按账号池健康、冷却状态和近 1 小时请求失败分布判断当前承载风险。</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadCapacity}>刷新</button>
		<span class="muted mono">{lastRefresh}</span>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">正在加载容量数据...</p>
{:else if data}
	<div class="metric-grid">
		<MetricCard label="账号" value={data.summary.accounts} sub={`${data.summary.active_accounts} 个 active`} />
		<MetricCard label="Responses 可用" value={data.summary.available_native} sub={`${data.summary.cooling_accounts} 个冷却中`} />
		<MetricCard label="近 1 小时请求" value={fmtNum(data.summary.requests_1h)} sub={`${data.summary.failures_1h} 个失败`} />
		<MetricCard label="处理中请求" value={data.summary.active_requests} sub={`连接 active: ${data.connections.active ?? 0}`} />
	</div>

	<section class="panel">
		<div class="page-header compact">
			<div>
				<h2>上游账号</h2>
				<p class="sub">失败率只用于观测，不参与自动调度。</p>
			</div>
		</div>
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>账号</th>
						<th>Provider</th>
						<th>状态</th>
						<th>可用面</th>
						<th class="num">权重</th>
						<th class="num">1h 请求</th>
						<th class="num">失败率</th>
						<th>冷却</th>
						<th>窗口</th>
					</tr>
				</thead>
				<tbody>
					{#each data.accounts as acct (acct.id)}
						<tr>
							<td>
								<div>{acct.email}</div>
								<div class="muted mono">{acct.id}</div>
							</td>
							<td>{providerLabel(acct.provider)}</td>
							<td><StatusBadge status={acct.status} /></td>
							<td>{availability(acct.available_native, acct.available_compat)}</td>
							<td class="num">{acct.weight}</td>
							<td class="num">{fmtNum(acct.requests_1h)}</td>
							<td class="num">{failureRate(acct.requests_1h, acct.failures_1h)}</td>
							<td>{acct.cooldown_until ? fmtDate(acct.cooldown_until) : '-'}</td>
							<td>
								{#if acct.windows?.length}
									<div class="stack-tight">
										{#each acct.windows as window}
											<div class="muted">{window.label}: {window.pct}%</div>
										{/each}
									</div>
								{:else}
									<span class="muted">-</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>

	<section class="panel">
		<div class="page-header compact">
			<div>
				<h2>当前请求</h2>
				<p class="sub">进程内正在处理的 HTTP 请求。</p>
			</div>
		</div>
		{#if data.active_requests.length === 0}
			<p class="muted">暂无处理中请求。</p>
		{:else}
			<div class="table-wrap">
				<table>
					<thead>
						<tr><th>ID</th><th>方法</th><th>路径</th><th>耗时</th><th>开始时间</th></tr>
					</thead>
					<tbody>
						{#each data.active_requests as req}
							<tr>
								<td class="mono">{req.id}</td>
								<td>{req.method}</td>
								<td class="mono">{req.path}</td>
								<td>{req.age}</td>
								<td>{req.started}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</section>
{:else}
	<p class="muted">暂无容量数据</p>
{/if}
