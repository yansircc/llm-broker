<script lang="ts">
	import { api } from '$lib/api';
	import type { ActivityData, DashboardEvent, RecentRequestLog, UsagePeriod } from '$lib/admin-types';
	import ConfirmAction from '$lib/components/ConfirmAction.svelte';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import { eventTypeColor, fmtCost, fmtDate, fmtJSON, fmtNum, fmtTime, shortModel, statusColor } from '$lib/format';
	import { egressLabel, healthLabel, providerLabel, surfaceLabel } from '$lib/admin-i18n';

	let data = $state<ActivityData | null>(null);
	let usage = $state<UsagePeriod[]>([]);
	let error = $state('');
	let usageError = $state('');
	let usageLoading = $state(false);
	let lastRefresh = $state('');

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		usageError = '';
		try {
			data = await api<ActivityData>('/activity');
			void loadUsage();
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	async function loadUsage() {
		usageLoading = true;
		try {
			// Remote SQLite analytics can still exceed the default 15s fetch timeout.
			// Remove this override after indexed/preaggregated usage queries are consistently <15s.
			usage = await api<UsagePeriod[]>('/activity/usage', { timeout: 30000 });
		} catch (e: any) {
			usage = [];
			usageError = e.message;
		} finally {
			usageLoading = false;
		}
	}

	async function clearEvents() {
		try {
			await api('/events', { method: 'DELETE' });
			if (data) data.events = [];
		} catch (e: any) {
			error = e.message;
		}
	}

	function eventKey(ev: DashboardEvent, i: number): string {
		return [ev.ts, ev.type, ev.account_id, ev.user_id, ev.bucket_key, ev.cell_id, i].join('|');
	}

	function eventFacts(ev: DashboardEvent): Array<{ label: string; value: string }> {
		const facts: Array<{ label: string; value: string }> = [];
		if (ev.user_id) facts.push({ label: '用户', value: ev.user_id });
		if (ev.account_id) facts.push({ label: '账号', value: ev.account_id });
		if (ev.bucket_key) facts.push({ label: 'bucket', value: ev.bucket_key });
		if (ev.cell_id) facts.push({ label: '节点', value: ev.cell_id });
		if (ev.upstream_status) facts.push({ label: '状态', value: String(ev.upstream_status) });
		if (ev.upstream_error_type) facts.push({ label: '错误类型', value: ev.upstream_error_type });
		if (ev.upstream_error_message) facts.push({ label: '错误', value: ev.upstream_error_message });
		if (ev.cooldown_until) facts.push({ label: '冷却', value: fmtDate(ev.cooldown_until) });
		return facts;
	}

	function failureOutcome(log: RecentRequestLog): string {
		const parts: string[] = [];
		if (log.effect_kind) parts.push(log.effect_kind);
		if (log.upstream_status) parts.push(String(log.upstream_status));
		return parts.join(' / ') || log.status;
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

	function hasDetails(log: RecentRequestLog): boolean {
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
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else if !data}
	<p class="loading">正在加载运行事件...</p>
{:else}
	<div class="page-header">
		<div>
			<div class="eyebrow">观测</div>
			<h1>运行事件</h1>
			<p class="lede">查看用量周期、失败转发证据和 Broker 事件，用于事后排障。</p>
		</div>
		<div class="page-actions">
			<button class="link" onclick={loadAll}>刷新</button>
			<span class="muted mono">{lastRefresh}</span>
		</div>
	</div>

	<div class="metric-grid">
		<MetricCard label="版本" value={data.health.version} sub="broker build" />
		<MetricCard label="运行时间" value={data.health.uptime} sub="当前进程" />
		<MetricCard label="SQLite" value={healthLabel(data.health.sqlite)} sub="本地存储健康状态" />
		<MetricCard label="事件" value={data.events.length} sub={`${data.recent_failures?.length ?? 0} 条最近失败`} />
	</div>

	<h2>用量</h2>
	{#if usageLoading}
		<p class="muted">正在加载用量...</p>
	{:else if usageError}
		<p class="error-msg">{usageError}</p>
	{:else if usage.length === 0}
		<p class="muted">暂无用量数据</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th></th>
						<th class="num">请求</th>
						<th class="num">输入</th>
						<th class="num">输出</th>
						<th class="num">缓存读取</th>
						<th class="num">成本</th>
					</tr>
				</thead>
				<tbody>
					{#each usage as period, i (period.label)}
						<tr>
							<td>{period.label}</td>
							<td class="num">{fmtNum(period.requests)}</td>
							<td class="num">{fmtNum(period.input_tokens)}</td>
							<td class="num">{fmtNum(period.output_tokens)}</td>
							<td class="num">{fmtNum(period.cache_read_tokens)}</td>
							<td class="num">{#if i === usage.length - 1}<b>{fmtCost(period.cost_usd)}</b>{:else}{fmtCost(period.cost_usd)}{/if}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<div class="section-header">
		<h2>最近失败转发</h2>
	</div>
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
							<td class="muted">{fmtTime(log.created_at)}</td>
							<td>{userLabel(log.user_id)}</td>
							<td>{providerLabel(log.provider)}</td>
							<td>{surfaceLabel(log.surface)}</td>
							<td>{shortModel(log.model)}</td>
							<td>{log.path || '-'}</td>
							<td>{accountLabel(log.account_id)}</td>
							<td>{egressLabel(log.cell_id)}</td>
							<td class={statusColor(log.status)}>{failureOutcome(log)}</td>
							<td>{log.upstream_request_id || '-'}</td>
							<td>{failureError(log)}</td>
							<td class="num">{fmtNum(log.request_bytes)}</td>
							<td class="num">{fmtNum(log.attempt_count)}</td>
							<td>
								{#if hasDetails(log)}
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

	<div class="section-header">
		<h2>最近错误</h2>
		{#if data.events.length > 0}
			<ConfirmAction label="清空" cls="r" onclick={clearEvents} />
		{/if}
	</div>
	{#if data.events.length === 0}
		<p class="muted">暂无错误</p>
	{:else}
		<ul class="event-list">
			{#each data.events as ev, i (eventKey(ev, i))}
				{@const facts = eventFacts(ev)}
				<li>
					<div class="event-line">
						<span class="ts">{fmtTime(ev.ts)}</span>
						<span class={eventTypeColor(ev.type)}>{ev.type.toUpperCase()}</span>
						<span>{ev.message}</span>
					</div>
					{#if facts.length > 0}
						<div class="event-meta">
							{#each facts as fact, j (`${fact.label}:${fact.value}`)}
								{#if j > 0}<span class="muted">|</span>{/if}
								<span class="muted">{fact.label}=</span><span class="event-value">{fact.value}</span>
							{/each}
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
{/if}

<style>
	.section-header {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.section-header h2 { margin: 0; }
	.event-line {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: baseline;
	}
	.event-meta {
		padding-left: 72px;
	}
	.event-value {
		word-break: break-all;
	}
	.detail-block {
		min-width: 320px;
		max-width: 560px;
	}
	pre {
		margin: 4px 0 0;
		white-space: pre-wrap;
		word-break: break-word;
	}
</style>
