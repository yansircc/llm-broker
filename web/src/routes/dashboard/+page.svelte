<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { DashboardData, EgressCellView } from '$lib/admin-types';
	import { fmtDate } from '$lib/format';

	let data = $state<DashboardData | null>(null);
	let cells = $state<EgressCellView[]>([]);
	let error = $state('');
	let lastRefresh = $state('');

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		try {
			const [dashboard, egressCells] = await Promise.all([
				api<DashboardData>('/dashboard'),
				api<EgressCellView[]>('/egress/cells')
			]);
			data = dashboard;
			cells = egressCells;
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	function cellRegion(cell: EgressCellView): string {
		const labels = cell.labels ?? {};
		return [labels.country, labels.city].filter(Boolean).join(' / ') || labels.site || '-';
	}

	function cellProxy(cell: EgressCellView): string {
		if (!cell.proxy) return '-';
		return `${cell.proxy.type}://${cell.proxy.host}:${cell.proxy.port}`;
	}

	function cellStatusLabel(cell: EgressCellView): string {
		if (cell.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now()) return 'cooling';
		return cell.status || '-';
	}

	function cellStatusClass(cell: EgressCellView): string {
		if (cell.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now()) return 'tag tag-overloaded';
		switch (cell.status) {
			case 'active': return 'tag tag-active';
			case 'error': return 'tag tag-error';
			case 'disabled': return 'tag tag-disabled';
			default: return 'tag';
		}
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else if !data}
	<p class="loading">loading ops board...</p>
{:else}
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
	<div class="sub">{data.health.version} &middot; up {data.health.uptime} &middot; sqlite <span class={data.health.sqlite === 'ok' ? 'g' : 'r'}>{data.health.sqlite}</span></div>

	<div class="bar">
		<span>cells {cells.length}</span>
		<span>accounts {data.accounts.length}</span>
		<span>legacy direct {data.accounts.filter((acct) => !acct.cell_id).length}</span>
		<span>cooling cells {cells.filter((cell) => cell.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now()).length}</span>
		<span><a href="{base}/migrations">migration</a></span>
	</div>

	<h2>egress cells</h2>
	{#if cells.length === 0}
		<p class="muted">no cells</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>cell</th>
					<th>region</th>
					<th>proxy</th>
					<th class="num">load</th>
					<th>status</th>
					<th>cooldown</th>
					<th>updated</th>
				</tr>
			</thead>
			<tbody>
				{#each cells as cell (cell.id)}
					<tr>
						<td><a href="{base}/cells/{cell.id}">{cell.name || cell.id}</a><br><span class="muted">{cell.id}</span></td>
						<td>{cellRegion(cell)}</td>
						<td>{cellProxy(cell)}</td>
						<td class="num">{cell.accounts.length}</td>
						<td><span class={cellStatusClass(cell)}>{cellStatusLabel(cell)}</span></td>
						<td>
							{#if cell.cooldown_until}
								{fmtDate(cell.cooldown_until)}
							{:else}
								<span class="muted">-</span>
							{/if}
						</td>
						<td>{fmtDate(cell.updated_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
{/if}
