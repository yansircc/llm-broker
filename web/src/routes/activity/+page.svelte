<script lang="ts">
	import { api } from '$lib/api';
	import type { DashboardData, DashboardEvent, RecentRequestLog } from '$lib/admin-types';
	import ConfirmAction from '$lib/components/ConfirmAction.svelte';
	import { eventTypeColor, fmtCost, fmtDate, fmtNum, fmtTime, shortModel, statusColor } from '$lib/format';

	let data = $state<DashboardData | null>(null);
	let error = $state('');
	let lastRefresh = $state('');

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		try {
			data = await api<DashboardData>('/dashboard');
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
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
		if (ev.user_id) facts.push({ label: 'user', value: ev.user_id });
		if (ev.account_id) facts.push({ label: 'account', value: ev.account_id });
		if (ev.bucket_key) facts.push({ label: 'bucket', value: ev.bucket_key });
		if (ev.cell_id) facts.push({ label: 'cell', value: ev.cell_id });
		if (ev.upstream_status) facts.push({ label: 'status', value: String(ev.upstream_status) });
		if (ev.cooldown_until) facts.push({ label: 'cooldown', value: fmtDate(ev.cooldown_until) });
		return facts;
	}

	function failureOutcome(log: RecentRequestLog): string {
		const parts: string[] = [];
		if (log.effect_kind) parts.push(log.effect_kind);
		if (log.upstream_status) parts.push(String(log.upstream_status));
		return parts.join(' / ') || log.status;
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else if !data}
	<p class="loading">loading activity...</p>
{:else}
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
	<div class="sub">{data.health.version} &middot; up {data.health.uptime} &middot; sqlite <span class={data.health.sqlite === 'ok' ? 'g' : 'r'}>{data.health.sqlite}</span></div>

	<h2>usage</h2>
	{#if data.usage.length === 0}
		<p class="muted">no usage data yet</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th></th>
					<th class="num">requests</th>
					<th class="num">input</th>
					<th class="num">output</th>
					<th class="num">cache read</th>
					<th class="num">cost</th>
				</tr>
			</thead>
			<tbody>
				{#each data.usage as usage, i (usage.label)}
					<tr>
						<td>{usage.label}</td>
						<td class="num">{fmtNum(usage.requests)}</td>
						<td class="num">{fmtNum(usage.input_tokens)}</td>
						<td class="num">{fmtNum(usage.output_tokens)}</td>
						<td class="num">{fmtNum(usage.cache_read_tokens)}</td>
						<td class="num">{#if i === data.usage.length - 1}<b>{fmtCost(usage.cost_usd)}</b>{:else}{fmtCost(usage.cost_usd)}{/if}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	<div class="section-header">
		<h2>recent failed relays</h2>
	</div>
	{#if !data.recent_failures || data.recent_failures.length === 0}
		<p class="muted">no failed relays yet</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>time</th>
					<th>provider</th>
					<th>surface</th>
					<th>model</th>
					<th>path</th>
					<th>account</th>
					<th>cell</th>
					<th>outcome</th>
					<th>request id</th>
					<th class="num">bytes</th>
					<th class="num">attempt</th>
				</tr>
			</thead>
			<tbody>
				{#each data.recent_failures as log (log.id)}
					<tr>
						<td class="muted">{fmtTime(log.created_at)}</td>
						<td>{log.provider}</td>
						<td>{log.surface || '-'}</td>
						<td>{shortModel(log.model)}</td>
						<td>{log.path || '-'}</td>
						<td>{log.account_id}</td>
						<td>{log.cell_id || 'legacy direct'}</td>
						<td class={statusColor(log.status)}>{failureOutcome(log)}</td>
						<td>{log.upstream_request_id || '-'}</td>
						<td class="num">{fmtNum(log.request_bytes)}</td>
						<td class="num">{fmtNum(log.attempt_count)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	<div class="section-header">
		<h2>recent errors</h2>
		{#if data.events.length > 0}
			<ConfirmAction label="[clear]" cls="r" onclick={clearEvents} />
		{/if}
	</div>
	{#if data.events.length === 0}
		<p class="muted">no errors yet</p>
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
</style>
