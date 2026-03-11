<script lang="ts">
	import { api } from '$lib/api';
	import type { DashboardData } from '$lib/admin-types';
	import { eventTypeColor, fmtCost, fmtNum, fmtTime } from '$lib/format';

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
		<h2>recent errors</h2>
		{#if data.events.length > 0}
			<button class="btn-sm" onclick={clearEvents}>clear</button>
		{/if}
	</div>
	{#if data.events.length === 0}
		<p class="muted">no errors yet</p>
	{:else}
		<ul class="event-list">
			{#each data.events as ev, i (`${ev.ts}|${ev.type}|${ev.account_id}|${i}`)}
				<li>
					<span class="ts">{fmtTime(ev.ts)}</span>
					<span class={eventTypeColor(ev.type)}>{ev.type.toUpperCase()}</span>
					{ev.message}
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
	.btn-sm {
		font-size: 0.75rem;
		padding: 2px 8px;
		background: #333;
		color: #aaa;
		border: 1px solid #555;
		border-radius: 3px;
		cursor: pointer;
	}
	.btn-sm:hover { background: #444; color: #fff; }
</style>
