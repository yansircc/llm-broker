<script lang="ts">
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { EgressCellView } from '$lib/admin-types';
	import { fmtDate, dotClass } from '$lib/format';
	import { providerLabel, statusLabel } from '$lib/admin-i18n';

	let cell = $state<EgressCellView | null>(null);
	let error = $state('');
	let loading = $state(true);
	let actionError = $state('');
	let clearing = $state(false);
	let testing = $state(false);
	let testResult = $state<{ ok: boolean; latency_ms?: number; error?: string } | null>(null);

	$effect(() => {
		loadCell();
	});

	async function loadCell() {
		error = '';
		actionError = '';
		loading = true;
		try {
			const cells = await api<EgressCellView[]>('/egress/cells');
			cell = cells.find((item) => item.id === $page.params.id) ?? null;
			if (!cell) error = '节点不存在';
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function region(item: EgressCellView): string {
		const labels = item.labels ?? {};
		return [labels.country, labels.city].filter(Boolean).join(' / ') || labels.site || '-';
	}

	function proxyLabel(item: EgressCellView): string {
		if (!item.proxy) return '-';
		return `${item.proxy.type}://${item.proxy.host}:${item.proxy.port}`;
	}

	function cooldownActive(item: EgressCellView): boolean {
		return !!item.cooldown_until && new Date(item.cooldown_until).getTime() > Date.now();
	}

	function activeCooldownUntil(item: EgressCellView): string | null {
		return cooldownActive(item) ? item.cooldown_until ?? null : null;
	}

	function cellAccounts(item: EgressCellView): NonNullable<EgressCellView['accounts']> {
		return item.accounts ?? [];
	}

	async function clearCooldown() {
		if (!cell) return;
		clearing = true;
		actionError = '';
		try {
			await api(`/egress/cells/${cell.id}/clear-cooldown`, { method: 'POST' });
			await loadCell();
		} catch (e: any) {
			actionError = e.message;
		} finally {
			clearing = false;
		}
	}

	async function testCell() {
		if (!cell) return;
		testing = true;
		testResult = null;
		actionError = '';
		try {
			testResult = await api<{ ok: boolean; latency_ms?: number; error?: string }>(`/egress/cells/${cell.id}/test`, { method: 'POST' });
		} catch (e: any) {
			actionError = e.message;
		} finally {
			testing = false;
		}
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
	<p><a href="{base}/console/dashboard">&larr; 返回控制台</a></p>
{:else if loading || !cell}
	<p class="loading">正在加载节点...</p>
{:else}
	<div class="page-header">
		<div>
			<div class="eyebrow">出口节点</div>
			<h1>{cell.name}</h1>
			<p class="lede mono">{cell.id}</p>
		</div>
		<div class="page-actions">
			<button class="link" onclick={loadCell}>刷新</button>
			<button class="link" onclick={testCell} disabled={testing}>{testing ? '测试中...' : '测试代理'}</button>
		{#if cooldownActive(cell)}
				<button class="link o" onclick={clearCooldown} disabled={clearing}>{clearing ? '清除中...' : '清除冷却'}</button>
		{/if}
		</div>
	</div>

	<div class="bar">
		{#if testResult}
			{#if testResult.ok}
				<span class="g">正常 {testResult.latency_ms}ms</span>
			{:else}
				<span class="r">{testResult.error}</span>
			{/if}
		{/if}
	</div>

	{#if actionError}
		<p class="error-msg">{actionError}</p>
	{/if}

	<dl>
		<dt>状态</dt>
		<dd>
			{#if cooldownActive(cell)}
				<span class="tag tag-overloaded">冷却中</span>
			{:else}
				<span class={cell.status === 'active' ? 'tag tag-active' : cell.status === 'error' ? 'tag tag-error' : 'tag tag-disabled'}>{statusLabel(cell.status)}</span>
			{/if}
		</dd>

		<dt>地区</dt>
		<dd>{region(cell)}</dd>

		<dt>代理</dt>
		<dd>{proxyLabel(cell)}</dd>

		<dt>冷却到</dt>
		<dd>
			{#if activeCooldownUntil(cell)}
				{fmtDate(activeCooldownUntil(cell)!)}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>标签</dt>
		<dd>
			{#if cell.labels && Object.keys(cell.labels).length > 0}
				{Object.entries(cell.labels).map(([key, value]) => `${key}=${value}`).join(' / ')}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>创建时间</dt>
		<dd>{fmtDate(cell.created_at)}</dd>

		<dt>更新时间</dt>
		<dd>{fmtDate(cell.updated_at)}</dd>
	</dl>

	<h2>绑定账号</h2>
	{#if cellAccounts(cell).length === 0}
		<p class="muted">暂无绑定账号</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>邮箱</th>
						<th>上游</th>
						<th>状态</th>
					</tr>
				</thead>
				<tbody>
					{#each cellAccounts(cell) as account (account.id)}
						<tr>
							<td><a href="{base}/console/accounts/{account.id}">{account.email}</a></td>
							<td>{providerLabel(account.provider)}</td>
							<td><span class={dotClass(account.status)}>{statusLabel(account.status)}</span></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<p class="sub"><a href="{base}/console/dashboard">返回控制台</a></p>
{/if}
