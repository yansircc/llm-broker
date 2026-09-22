<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { SpendData, SpendCosts } from '$lib/admin-types';
	import { fmtCost, shortModel } from '$lib/format';

	type CostKey = keyof SpendCosts;
	type SortKey = 'user' | 'model' | CostKey;

	interface FlatRow {
		userId: string;
		userName: string;
		model: string;
		// shortModel() strips the claude-/gpt- prefixes, so the raw id and the
		// rendered label sort differently. Sort on what the eye actually reads.
		label: string;
		cost: SpendCosts;
	}

	let data = $state<SpendData | null>(null);
	let error = $state('');
	let lastRefresh = $state('');

	// null = the default grouped view (per-user subtotals). Clicking any header
	// flattens the table, because a global ordering and per-user subtotals
	// cannot both be true at once.
	let sortKey = $state<SortKey | null>(null);
	let sortDir = $state<'asc' | 'desc'>('desc');

	const columns: Array<{ key: SortKey; label: string; num: boolean }> = [
		{ key: 'user', label: 'user', num: false },
		{ key: 'model', label: 'model', num: false },
		{ key: 'd1', label: '1d', num: true },
		{ key: 'd3', label: '3d', num: true },
		{ key: 'd7', label: '7d', num: true },
		{ key: 'd30', label: '30d', num: true }
	];

	$effect(() => {
		load();
	});

	async function load() {
		error = '';
		try {
			// A 30-day GROUP BY over request_log can exceed the default 15s fetch
			// timeout on remote SQLite. Same temporary widening as /activity/usage.
			// Remove once the query is indexed or pre-aggregated.
			data = await api<SpendData>('/spend', { timeout: 60000 });
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	function sortBy(key: SortKey) {
		if (sortKey === key) {
			sortDir = sortDir === 'desc' ? 'asc' : 'desc';
			return;
		}
		sortKey = key;
		sortDir = 'desc';
	}

	function resetSort() {
		sortKey = null;
		sortDir = 'desc';
	}

	function indicator(key: SortKey): string {
		if (sortKey !== key) return '';
		return sortDir === 'desc' ? ' ↓' : ' ↑';
	}

	const flatRows = $derived.by<FlatRow[]>(() => {
		if (!data || sortKey === null) return [];
		const rows: FlatRow[] = [];
		for (const group of data.users) {
			for (const m of group.models) {
				rows.push({
					userId: group.user_id,
					userName: group.user_name,
					model: m.model,
					label: shortModel(m.model),
					cost: m.cost
				});
			}
		}
		const key = sortKey;
		const flip = sortDir === 'desc' ? -1 : 1;
		rows.sort((a, b) => {
			let cmp: number;
			if (key === 'user') cmp = a.userName.localeCompare(b.userName);
			else if (key === 'model') cmp = a.label.localeCompare(b.label);
			else cmp = a.cost[key] - b.cost[key];
			// Stable, readable tie-break so equal values (often $0) do not jitter.
			if (cmp === 0) cmp = a.userName.localeCompare(b.userName) || a.label.localeCompare(b.label);
			return cmp * flip;
		});
		return rows;
	});
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else if !data}
	<p class="loading">loading spend...</p>
{:else}
	<span class="refresh">
		{#if sortKey !== null}
			<button class="link" onclick={resetSort}>[grouped]</button>
		{/if}
		<button class="link" onclick={load}>[refresh]</button>
		<span class="muted">{lastRefresh}</span>
	</span>
	<div class="sub">cost per user per model &middot; rolling windows from now &middot; successful relays only &middot; gemini excluded</div>

	{#if data.users.length === 0}
		<p class="muted">no spend recorded in the last 30 days</p>
	{:else}
		<table>
			<thead>
				<tr>
					{#each columns as col (col.key)}
						<th class={col.num ? 'num' : ''}>
							<button class="sort" onclick={() => sortBy(col.key)}
								>{col.label}{indicator(col.key)}</button
							>
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#if sortKey === null}
					{#each data.users as group (group.user_id)}
						{#each group.models as row (row.model)}
							<tr>
								<td><a href="{base}/users/{group.user_id}">{group.user_name}</a></td>
								<td>{shortModel(row.model)}</td>
								<td class="num">{fmtCost(row.cost.d1)}</td>
								<td class="num">{fmtCost(row.cost.d3)}</td>
								<td class="num">{fmtCost(row.cost.d7)}</td>
								<td class="num">{fmtCost(row.cost.d30)}</td>
							</tr>
						{/each}
						<tr class="subtotal">
							<td><a href="{base}/users/{group.user_id}">{group.user_name}</a></td>
							<td class="muted">subtotal</td>
							<td class="num">{fmtCost(group.subtotal.d1)}</td>
							<td class="num">{fmtCost(group.subtotal.d3)}</td>
							<td class="num">{fmtCost(group.subtotal.d7)}</td>
							<td class="num">{fmtCost(group.subtotal.d30)}</td>
						</tr>
					{/each}
				{:else}
					{#each flatRows as row (row.userId + '/' + row.model)}
						<tr>
							<td><a href="{base}/users/{row.userId}">{row.userName}</a></td>
							<td>{row.label}</td>
							<td class="num">{fmtCost(row.cost.d1)}</td>
							<td class="num">{fmtCost(row.cost.d3)}</td>
							<td class="num">{fmtCost(row.cost.d7)}</td>
							<td class="num">{fmtCost(row.cost.d30)}</td>
						</tr>
					{/each}
				{/if}
				<tr class="grand-total">
					<td colspan="2">TOTAL</td>
					<td class="num">{fmtCost(data.total.d1)}</td>
					<td class="num">{fmtCost(data.total.d3)}</td>
					<td class="num">{fmtCost(data.total.d7)}</td>
					<td class="num">{fmtCost(data.total.d30)}</td>
				</tr>
			</tbody>
		</table>
		<p class="hint">
			{#if sortKey === null}
				Click any header to sort across all users; per-user subtotals are dropped while sorted.
			{:else}
				Sorted across all users &mdash; per-user subtotals hidden. Use [grouped] to restore them.
			{/if}
			Totals cover the users listed above only. Requests made with the shared admin token, and
			requests from users that have since been deleted, are not included.
		</p>
	{/if}
{/if}

<style>
	th button.sort {
		background: none;
		border: none;
		padding: 0;
		font: inherit;
		color: inherit;
		cursor: pointer;
	}
	th button.sort:hover {
		text-decoration: underline;
	}
	tr.subtotal td {
		font-weight: bold;
		border-bottom: 1px solid #ccc;
	}
	tr.grand-total td {
		font-weight: bold;
		border-top: 2px solid #888;
		padding-top: 4px;
	}
	@media (prefers-color-scheme: dark) {
		tr.subtotal td {
			border-bottom-color: #444;
		}
		tr.grand-total td {
			border-top-color: #777;
		}
	}
</style>
