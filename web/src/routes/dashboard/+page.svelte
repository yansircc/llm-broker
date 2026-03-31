<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { CellRiskStat, DashboardData, EgressCellView, RecentRequestLog, RelayOutcomeStat } from '$lib/admin-types';
	import { fmtDate, fmtJSON, fmtNum, shortModel, statusColor } from '$lib/format';

	let data = $state<DashboardData | null>(null);
	let cells = $state<EgressCellView[]>([]);
	let error = $state('');
	let lastRefresh = $state('');

	// add-cell form
	let showAddCell = $state(false);
	let addCellName = $state('');
	let addCellHost = $state('');
	let addCellPort = $state('');
	let addCellUser = $state('');
	let addCellPass = $state('');
	let addCellSaving = $state(false);
	let addCellError = $state('');

	function resetAddCellForm() {
		addCellName = '';
		addCellHost = '';
		addCellPort = '';
		addCellUser = '';
		addCellPass = '';
		addCellError = '';
		addCellSaving = false;
	}

	async function saveNewCell() {
		addCellError = '';
		const name = addCellName.trim();
		const host = addCellHost.trim();
		const port = parseInt(addCellPort, 10);
		if (!name || !host || !port || port <= 0) {
			addCellError = 'name, host, and valid port are required';
			return;
		}
		const id = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
		if (cells.some((c) => c.id === id)) {
			addCellError = `cell "${id}" already exists`;
			return;
		}
		const proxyConfig = {
			type: 'socks5',
			host,
			port,
			username: addCellUser.trim() || undefined,
			password: addCellPass.trim() || undefined
		};
		addCellSaving = true;
		addCellError = '';
		try {
			const test = await api<{ ok: boolean; latency_ms?: number; error?: string }>('/egress/cells/test-proxy', {
				method: 'POST',
				body: JSON.stringify({ proxy: proxyConfig })
			});
			if (!test.ok) {
				addCellError = `proxy test failed: ${test.error || 'unknown error'}`;
				return;
			}
			await api('/egress/cells', {
				method: 'POST',
				body: JSON.stringify({ id, name, status: 'active', proxy: proxyConfig, create_only: true })
			});
			resetAddCellForm();
			showAddCell = false;
			await loadAll();
		} catch (e: any) {
			addCellError = e.message || 'failed to create cell';
		} finally {
			addCellSaving = false;
		}
	}

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

	function cellAccounts(cell: EgressCellView) {
		return cell.accounts ?? [];
	}

	function activeCooldownUntil(cell: EgressCellView): string | null {
		return cell.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now() ? cell.cooldown_until : null;
	}

	function cellStatusLabel(cell: EgressCellView): string {
		if (activeCooldownUntil(cell)) return 'cooling';
		return cell.status || '-';
	}

	function cellStatusClass(cell: EgressCellView): string {
		if (activeCooldownUntil(cell)) return 'tag tag-overloaded';
		switch (cell.status) {
			case 'active': return 'tag tag-active';
			case 'error': return 'tag tag-error';
			case 'disabled': return 'tag tag-disabled';
			default: return 'tag';
		}
	}

	function activeAccounts() {
		return data?.accounts.filter((acct) => acct.status === 'active').length ?? 0;
	}

	function availableNativeAccounts() {
		return data?.accounts.filter((acct) => acct.available_native).length ?? 0;
	}

	function availableCompatAccounts() {
		return data?.accounts.filter((acct) => acct.available_compat).length ?? 0;
	}

	function outcomeLabel(stat: RelayOutcomeStat): string {
		const parts = [stat.effect_kind || 'none'];
		if (stat.upstream_status) parts.push(String(stat.upstream_status));
		return parts.join(' / ');
	}

	function failureCell(log: RecentRequestLog): string {
		return log.cell_id || 'legacy direct';
	}

	function failureUpstream(log: RecentRequestLog): string {
		const parts: string[] = [];
		if (log.effect_kind) parts.push(log.effect_kind);
		if (log.upstream_status) parts.push(String(log.upstream_status));
		return parts.join(' / ') || '-';
	}

	function failureRequestID(log: RecentRequestLog): string {
		return log.upstream_request_id || '-';
	}

	function accountInfo(accountID: string) {
		return data?.accounts.find((account) => account.id === accountID);
	}

	function accountLabel(accountID: string): string {
		return accountInfo(accountID)?.email || '-';
	}

	function userInfo(userID: string) {
		return data?.users.find((user) => user.id === userID);
	}

	function userLabel(userID: string): string {
		return userInfo(userID)?.name || userID || '-';
	}

	function failureError(log: RecentRequestLog): string {
		const parts: string[] = [];
		if (log.upstream_error_type) parts.push(log.upstream_error_type);
		if (log.upstream_error_message) parts.push(log.upstream_error_message);
		return parts.join(': ') || '-';
	}

	function hasFailureDetails(log: RecentRequestLog): boolean {
		return !!(
			log.session_uuid ||
			log.binding_source ||
			log.upstream_error_type ||
			log.upstream_error_message ||
			log.client_body_excerpt ||
			log.request_meta ||
			log.client_headers ||
			log.upstream_url ||
			log.upstream_request_headers ||
			log.upstream_request_meta ||
			log.upstream_request_body_excerpt ||
			log.upstream_headers ||
			log.upstream_response_meta ||
			log.upstream_response_body_excerpt
		);
	}

	function cellRiskFailureCount(stat: CellRiskStat): number {
		return stat.status_400 + stat.status_403 + stat.status_429 + stat.blocks + stat.transport_errors;
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
		<span>active {activeAccounts()}</span>
		<span>available native {availableNativeAccounts()}</span>
		<span>available compat {availableCompatAccounts()}</span>
		<span>legacy direct {data.accounts.filter((acct) => !acct.cell_id).length}</span>
		<span>cooling cells {cells.filter((cell) => activeCooldownUntil(cell)).length}</span>
		<span><a href="{base}/migrations">migration</a></span>
	</div>

	<h2>egress cells</h2>
	<div class="bar" style="margin-bottom:8px">
		{#if !showAddCell}
			<button class="link" onclick={() => { resetAddCellForm(); showAddCell = true; }}>[+ add socks5 cell]</button>
		{:else}
			<div class="add-cell-form">
				<div class="add-cell-row">
					<input type="text" bind:value={addCellName} placeholder="name" class="add-cell-input" disabled={addCellSaving}>
					<input type="text" bind:value={addCellHost} placeholder="host" class="add-cell-input" disabled={addCellSaving}>
					<input type="text" bind:value={addCellPort} placeholder="port" class="add-cell-input add-cell-short" disabled={addCellSaving}>
				</div>
				<div class="add-cell-row">
					<input type="text" bind:value={addCellUser} placeholder="username (optional)" class="add-cell-input" disabled={addCellSaving}>
					<input type="password" bind:value={addCellPass} placeholder="password (optional)" class="add-cell-input" disabled={addCellSaving}>
					<span>
						<button class="link" onclick={saveNewCell} disabled={addCellSaving}>{addCellSaving ? '[saving...]' : '[save]'}</button>
						<button class="link" onclick={() => { showAddCell = false; }} disabled={addCellSaving}>[cancel]</button>
					</span>
				</div>
				{#if addCellError}
					<p class="error-msg" style="margin:4px 0 0">{addCellError}</p>
				{/if}
			</div>
		{/if}
	</div>
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
						<td><a href="{base}/cells/{cell.id}">{cell.name || cell.id}</a></td>
						<td>{cellRegion(cell)}</td>
						<td>{cellProxy(cell)}</td>
						<td class="num">{cellAccounts(cell).length}</td>
						<td><span class={cellStatusClass(cell)}>{cellStatusLabel(cell)}</span></td>
						<td>
							{#if activeCooldownUntil(cell)}
								{fmtDate(activeCooldownUntil(cell)!)}
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

	<h2>relay outcomes (24h)</h2>
	{#if !data.outcome_stats || data.outcome_stats.length === 0}
		<p class="muted">no relay stats yet</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>provider</th>
					<th>surface</th>
					<th>outcome</th>
					<th class="num">requests</th>
					<th class="num">users</th>
					<th class="num">accounts</th>
					<th>last seen</th>
				</tr>
			</thead>
			<tbody>
				{#each data.outcome_stats as stat (`${stat.provider}:${stat.surface}:${stat.effect_kind}:${stat.upstream_status ?? 0}`)}
					<tr>
						<td>{stat.provider}</td>
						<td>{stat.surface || '-'}</td>
						<td>{outcomeLabel(stat)}</td>
						<td class="num">{fmtNum(stat.requests)}</td>
						<td class="num">{fmtNum(stat.distinct_users)}</td>
						<td class="num">{fmtNum(stat.distinct_accounts)}</td>
						<td>{fmtDate(stat.last_seen_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	<h2>cell risk (7 days)</h2>
	{#if !data.cell_risk || data.cell_risk.length === 0}
		<p class="muted">no cell risk data yet</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>cell</th>
					<th>provider</th>
					<th>region</th>
					<th>transport</th>
					<th class="num">reqs</th>
					<th class="num">ok</th>
					<th class="num">400</th>
					<th class="num">403</th>
					<th class="num">429</th>
					<th class="num">block</th>
					<th class="num">transport</th>
					<th class="num">risk</th>
					<th class="num">users</th>
					<th>last seen</th>
				</tr>
			</thead>
			<tbody>
				{#each data.cell_risk as stat (`${stat.provider}:${stat.cell_id ?? 'legacy'}`)}
					<tr>
						<td>{stat.cell_name}</td>
						<td>{stat.provider}</td>
						<td>{stat.region}</td>
						<td>{stat.transport}</td>
						<td class="num">{fmtNum(stat.requests)}</td>
						<td class="num">{fmtNum(stat.successes)}</td>
						<td class="num">{fmtNum(stat.status_400)}</td>
						<td class="num">{fmtNum(stat.status_403)}</td>
						<td class="num">{fmtNum(stat.status_429)}</td>
						<td class="num">{fmtNum(stat.blocks)}</td>
						<td class="num">{fmtNum(stat.transport_errors)}</td>
						<td class="num">{fmtNum(cellRiskFailureCount(stat))}</td>
						<td class="num">{fmtNum(stat.distinct_users)}</td>
						<td>{fmtDate(stat.last_seen_at)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	<h2>recent failed relays</h2>
	{#if !data.recent_failures || data.recent_failures.length === 0}
		<p class="muted">no failed relays yet</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>time</th>
					<th>key</th>
					<th>provider</th>
					<th>surface</th>
					<th>model</th>
					<th>path</th>
					<th>account</th>
					<th>cell</th>
					<th>outcome</th>
					<th>request id</th>
					<th>error</th>
					<th class="num">bytes</th>
					<th class="num">attempt</th>
					<th>details</th>
				</tr>
			</thead>
			<tbody>
				{#each data.recent_failures as log (log.id)}
					<tr>
						<td class="muted">{fmtDate(log.created_at)}</td>
						<td>{userLabel(log.user_id)}</td>
						<td>{log.provider}</td>
						<td>{log.surface || '-'}</td>
						<td>{shortModel(log.model)}</td>
						<td>{log.path || '-'}</td>
						<td>{accountLabel(log.account_id)}</td>
						<td>{failureCell(log)}</td>
						<td class={statusColor(log.status)}>{failureUpstream(log)}</td>
						<td>{failureRequestID(log)}</td>
						<td>{failureError(log)}</td>
						<td class="num">{fmtNum(log.request_bytes)}</td>
						<td class="num">{fmtNum(log.attempt_count)}</td>
						<td>
							{#if hasFailureDetails(log)}
								<details>
									<summary>view</summary>
									<div class="detail-block">
										<div><span class="muted">full account</span> <span class="mono">{log.account_id}</span></div>
										<div><span class="muted">session</span> <span class="mono">{log.session_uuid || '-'}</span></div>
										<div><span class="muted">binding</span> {log.binding_source || '-'}</div>
										<div><span class="muted">error</span> {failureError(log)}</div>
										<div><span class="muted">client body</span><pre>{log.client_body_excerpt || '-'}</pre></div>
										<div><span class="muted">request meta</span><pre>{fmtJSON(log.request_meta)}</pre></div>
										<div><span class="muted">client headers</span><pre>{fmtJSON(log.client_headers)}</pre></div>
										<div><span class="muted">upstream url</span> <span class="mono">{log.upstream_url || '-'}</span></div>
										<div><span class="muted">upstream request headers</span><pre>{fmtJSON(log.upstream_request_headers)}</pre></div>
										<div><span class="muted">upstream request meta</span><pre>{fmtJSON(log.upstream_request_meta)}</pre></div>
										<div><span class="muted">upstream request body</span><pre>{log.upstream_request_body_excerpt || '-'}</pre></div>
										<div><span class="muted">upstream response headers</span><pre>{fmtJSON(log.upstream_headers)}</pre></div>
										<div><span class="muted">upstream response meta</span><pre>{fmtJSON(log.upstream_response_meta)}</pre></div>
										<div><span class="muted">upstream response body</span><pre>{log.upstream_response_body_excerpt || '-'}</pre></div>
									</div>
								</details>
							{:else}
								<span class="muted">-</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
{/if}

<style>
	.detail-block {
		min-width: 320px;
		max-width: 560px;
	}
	pre {
		margin: 4px 0 0;
		white-space: pre-wrap;
		word-break: break-word;
	}
	.add-cell-form {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.add-cell-row {
		display: flex;
		gap: 6px;
		align-items: center;
		flex-wrap: wrap;
	}
	.add-cell-input {
		width: 160px;
	}
	.add-cell-short {
		width: 80px;
	}
</style>
