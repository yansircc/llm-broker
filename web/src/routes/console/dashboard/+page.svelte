<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import type { CellRiskStat, DashboardData, EgressCellView, RecentRequestLog, RelayOutcomeStat } from '$lib/admin-types';
	import { fmtDate, fmtJSON, fmtNum, shortModel, statusColor } from '$lib/format';
	import { egressLabel, healthLabel, providerLabel, statusLabel, surfaceLabel } from '$lib/admin-i18n';

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
			addCellError = '请填写名称、主机和有效端口';
			return;
		}
		const id = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
		if (cells.some((c) => c.id === id)) {
			addCellError = `出口节点 "${id}" 已存在`;
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
				addCellError = `代理测试失败：${test.error || '未知错误'}`;
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
			addCellError = e.message || '创建出口节点失败';
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
		if (activeCooldownUntil(cell)) return statusLabel('cooling');
		return statusLabel(cell.status);
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
		return egressLabel(log.cell_id);
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
	<p class="loading">正在加载控制台...</p>
{:else}
	<div class="page-header">
		<div>
			<div class="eyebrow">运行总览</div>
			<h1>控制台</h1>
			<p class="lede">查看 Broker 健康状态、出口节点、转发结果、风险窗口和最近失败证据。</p>
		</div>
		<div class="page-actions">
			<button class="link" onclick={loadAll}>刷新</button>
			<span class="muted mono">{lastRefresh}</span>
		</div>
	</div>

	<div class="bar">
		<span>{data.health.version}</span>
		<span>运行 {data.health.uptime}</span>
		<span>sqlite <span class={data.health.sqlite === 'ok' ? 'g' : 'r'}>{healthLabel(data.health.sqlite)}</span></span>
		<span><a href="{base}/console/migrations">迁移工具</a></span>
	</div>

	<div class="metric-grid">
		<MetricCard label="出口节点" value={cells.length} sub={`${cells.filter((cell) => activeCooldownUntil(cell)).length} 个冷却中`} />
		<MetricCard label="上游账号" value={data.accounts.length} sub={`${activeAccounts()} 个正常`} />
		<MetricCard label="Responses 池" value={availableNativeAccounts()} sub="可用账号" />
		<MetricCard label="兼容层池" value={availableCompatAccounts()} sub={`${data.accounts.filter((acct) => !acct.cell_id).length} 个直连`} />
	</div>

	<div class="section-header">
		<h2>出口节点</h2>
		{#if !showAddCell}
			<button class="link" onclick={() => { resetAddCellForm(); showAddCell = true; }}>新增 socks5 节点</button>
		{:else}
			<div class="add-cell-form">
				<div class="add-cell-row">
					<input type="text" bind:value={addCellName} placeholder="名称" class="add-cell-input" disabled={addCellSaving}>
					<input type="text" bind:value={addCellHost} placeholder="主机" class="add-cell-input" disabled={addCellSaving}>
					<input type="text" bind:value={addCellPort} placeholder="端口" class="add-cell-input add-cell-short" disabled={addCellSaving}>
				</div>
				<div class="add-cell-row">
					<input type="text" bind:value={addCellUser} placeholder="用户名（可选）" class="add-cell-input" disabled={addCellSaving}>
					<input type="password" bind:value={addCellPass} placeholder="密码（可选）" class="add-cell-input" disabled={addCellSaving}>
					<span>
						<button class="link" onclick={saveNewCell} disabled={addCellSaving}>{addCellSaving ? '保存中...' : '保存'}</button>
						<button class="link" onclick={() => { showAddCell = false; }} disabled={addCellSaving}>取消</button>
					</span>
				</div>
				{#if addCellError}
					<p class="error-msg" style="margin:4px 0 0">{addCellError}</p>
				{/if}
			</div>
		{/if}
	</div>
	{#if cells.length === 0}
		<p class="muted">暂无出口节点</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>节点</th>
						<th>地区</th>
						<th>代理</th>
						<th class="num">负载</th>
						<th>状态</th>
						<th>冷却</th>
						<th>更新</th>
					</tr>
				</thead>
				<tbody>
					{#each cells as cell (cell.id)}
						<tr>
							<td><a href="{base}/console/cells/{cell.id}">{cell.name || cell.id}</a></td>
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
		</div>
	{/if}

	<h2>转发结果（24h）</h2>
	{#if !data.outcome_stats || data.outcome_stats.length === 0}
		<p class="muted">暂无转发统计</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>上游</th>
						<th>接口面</th>
						<th>结果</th>
						<th class="num">请求</th>
						<th class="num">用户</th>
						<th class="num">账号</th>
						<th>最近出现</th>
					</tr>
				</thead>
				<tbody>
					{#each data.outcome_stats as stat (`${stat.provider}:${stat.surface}:${stat.effect_kind}:${stat.upstream_status ?? 0}`)}
						<tr>
							<td>{providerLabel(stat.provider)}</td>
							<td>{surfaceLabel(stat.surface)}</td>
							<td>{outcomeLabel(stat)}</td>
							<td class="num">{fmtNum(stat.requests)}</td>
							<td class="num">{fmtNum(stat.distinct_users)}</td>
							<td class="num">{fmtNum(stat.distinct_accounts)}</td>
							<td>{fmtDate(stat.last_seen_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<h2>节点风险（7 天）</h2>
	{#if !data.cell_risk || data.cell_risk.length === 0}
		<p class="muted">暂无节点风险数据</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>节点</th>
						<th>上游</th>
						<th>地区</th>
						<th>传输</th>
						<th class="num">请求</th>
						<th class="num">成功</th>
						<th class="num">400</th>
						<th class="num">403</th>
						<th class="num">429</th>
						<th class="num">阻断</th>
						<th class="num">传输错误</th>
						<th class="num">风险</th>
						<th class="num">用户</th>
						<th>最近出现</th>
					</tr>
				</thead>
				<tbody>
					{#each data.cell_risk as stat (`${stat.provider}:${stat.cell_id ?? 'legacy'}`)}
						<tr>
							<td>{stat.cell_name}</td>
							<td>{providerLabel(stat.provider)}</td>
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
		</div>
	{/if}

	<h2>最近失败转发</h2>
	{#if !data.recent_failures || data.recent_failures.length === 0}
		<p class="muted">暂无失败转发</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>时间</th>
						<th>用户</th>
						<th>上游</th>
						<th>接口面</th>
						<th>模型</th>
						<th>路径</th>
						<th>账号</th>
						<th>节点</th>
						<th>结果</th>
						<th>request id</th>
						<th>错误</th>
						<th class="num">字节</th>
						<th class="num">尝试</th>
						<th>详情</th>
					</tr>
				</thead>
				<tbody>
					{#each data.recent_failures as log (log.id)}
						<tr>
							<td class="muted">{fmtDate(log.created_at)}</td>
							<td>{userLabel(log.user_id)}</td>
							<td>{providerLabel(log.provider)}</td>
							<td>{surfaceLabel(log.surface)}</td>
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
										<summary>查看</summary>
										<div class="detail-block">
											<div><span class="muted">完整账号</span> <span class="mono">{log.account_id}</span></div>
											<div><span class="muted">session</span> <span class="mono">{log.session_uuid || '-'}</span></div>
											<div><span class="muted">绑定</span> {log.binding_source || '-'}</div>
											<div><span class="muted">错误</span> {failureError(log)}</div>
											<div><span class="muted">客户端 body</span><pre>{log.client_body_excerpt || '-'}</pre></div>
											<div><span class="muted">请求 meta</span><pre>{fmtJSON(log.request_meta)}</pre></div>
											<div><span class="muted">客户端 headers</span><pre>{fmtJSON(log.client_headers)}</pre></div>
											<div><span class="muted">上游 URL</span> <span class="mono">{log.upstream_url || '-'}</span></div>
											<div><span class="muted">上游请求 headers</span><pre>{fmtJSON(log.upstream_request_headers)}</pre></div>
											<div><span class="muted">上游请求 meta</span><pre>{fmtJSON(log.upstream_request_meta)}</pre></div>
											<div><span class="muted">上游请求 body</span><pre>{log.upstream_request_body_excerpt || '-'}</pre></div>
											<div><span class="muted">上游响应 headers</span><pre>{fmtJSON(log.upstream_headers)}</pre></div>
											<div><span class="muted">上游响应 meta</span><pre>{fmtJSON(log.upstream_response_meta)}</pre></div>
											<div><span class="muted">上游响应 body</span><pre>{log.upstream_response_body_excerpt || '-'}</pre></div>
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
		</div>
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
