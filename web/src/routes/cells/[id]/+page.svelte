<script lang="ts">
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { EgressCellView } from '$lib/admin-types';
	import { fmtDate, dotClass } from '$lib/format';

	let cell = $state<EgressCellView | null>(null);
	let error = $state('');
	let loading = $state(true);
	let actionError = $state('');
	let clearing = $state(false);

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
			if (!cell) error = 'cell not found';
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
</script>

{#if error}
	<p class="error-msg">{error}</p>
	<p><a href="{base}/dashboard">&larr; back to ops</a></p>
{:else if loading || !cell}
	<p class="loading">loading cell...</p>
{:else}
	<h2>{cell.name} <span class="muted">{cell.id}</span></h2>

	<div class="actions">
		<button class="link" onclick={loadCell}>[refresh]</button>
		{#if cooldownActive(cell)}
			<button class="link o" onclick={clearCooldown} disabled={clearing}>{clearing ? '[clearing...]' : '[clear cooldown]'}</button>
		{/if}
	</div>

	{#if actionError}
		<p class="error-msg">{actionError}</p>
	{/if}

	<dl>
		<dt>status</dt>
		<dd>
			{#if cooldownActive(cell)}
				<span class="tag tag-overloaded">cooling</span>
			{:else}
				<span class={cell.status === 'active' ? 'tag tag-active' : cell.status === 'error' ? 'tag tag-error' : 'tag tag-disabled'}>{cell.status}</span>
			{/if}
		</dd>

		<dt>region</dt>
		<dd>{region(cell)}</dd>

		<dt>proxy</dt>
		<dd>{proxyLabel(cell)}</dd>

		<dt>cooldown until</dt>
		<dd>
			{#if cell.cooldown_until}
				{fmtDate(cell.cooldown_until)}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>labels</dt>
		<dd>
			{#if cell.labels && Object.keys(cell.labels).length > 0}
				{Object.entries(cell.labels).map(([key, value]) => `${key}=${value}`).join(' / ')}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>created</dt>
		<dd>{fmtDate(cell.created_at)}</dd>

		<dt>updated</dt>
		<dd>{fmtDate(cell.updated_at)}</dd>
	</dl>

	<h2>bound accounts</h2>
	{#if cell.accounts.length === 0}
		<p class="muted">no accounts bound</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>email</th>
					<th>provider</th>
					<th>status</th>
				</tr>
			</thead>
			<tbody>
				{#each cell.accounts as account (account.id)}
					<tr>
						<td><a href="{base}/accounts/{account.id}">{account.email}</a></td>
						<td>{account.provider}</td>
						<td><span class={dotClass(account.status)}>{account.status}</span></td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	<p style="margin-top:16px;font-size:12px"><a href="{base}/dashboard">&larr; back to ops</a></p>
{/if}
