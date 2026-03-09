<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import { fmtNum, fmtCost, fmtTime, timeAgo, remainClass, remainTime, eventTypeColor, dotClass } from '$lib/format';
	import Countdown from '$lib/components/Countdown.svelte';
	import { addAccountPath, type ProviderOption } from '$lib/providers';

	interface UsagePeriod {
		label: string;
		requests: number;
		input_tokens: number;
		output_tokens: number;
		cache_read_tokens: number;
		cost_usd: number;
	}

	interface AccountView {
		id: string;
		email: string;
		provider: string;
		status: string;
		priority_mode: string;
		priority: number;
		cooldown_until: string | null;
		last_used_at: string | null;
		windows: { label: string; pct: number; reset?: number }[];
	}

	interface UserView {
		id: string;
		name: string;
		status: string;
		last_active_at: string | null;
		total_cost: number;
	}

	interface EventInfo {
		type: string;
		account_id: string;
		message: string;
		ts: string;
	}

	interface DashboardData {
		health: {
			sqlite: string;
			uptime: string;
			version: string;
		};
		usage: UsagePeriod[];
		accounts: AccountView[];
		users: UserView[];
		events: EventInfo[];
	}

	interface AccountGroup {
		provider: string;
		accounts: AccountView[];
		window_labels: string[];
	}

	let data = $state<DashboardData | null>(null);
	let providers = $state<ProviderOption[]>([]);
	let error = $state('');
	let providerError = $state('');
	let lastRefresh = $state('');

	let showAddUser = $state(false);
	let newUserName = $state('');
	let creatingUser = $state(false);
	let addUserError = $state('');
	let createdUser = $state<{ name: string; token: string } | null>(null);
	let copied = $state(false);

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		providerError = '';
		try {
			data = await api<DashboardData>('/dashboard');
			providers = [];
			if (data.accounts.length === 0) {
				try {
					providers = await api<ProviderOption[]>('/providers');
				} catch (e: any) {
					providerError = e.message;
				}
			}
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	async function clearEvents() {
		try {
			await api('/events', { method: 'DELETE' });
			data.events = [];
		} catch (e: any) {
			error = e.message;
		}
	}

	function cancelAddUser() {
		showAddUser = false;
		newUserName = '';
		addUserError = '';
	}

	async function createUser() {
		const name = newUserName.trim();
		if (!name) { addUserError = 'username required'; return; }
		creatingUser = true;
		addUserError = '';
		try {
			const res = await api<{ id: string; name: string; token: string }>('/users', {
				method: 'POST',
				body: JSON.stringify({ name })
			});
			createdUser = { name: res.name, token: res.token };
			if (data) {
				data.users = [...data.users, { id: res.id, name: res.name, status: 'active', last_active_at: null, total_cost: 0 }];
			}
			showAddUser = false;
			newUserName = '';
		} catch (e: any) {
			addUserError = e.message;
		} finally {
			creatingUser = false;
		}
	}

	function buildTestCmd(token: string): string {
		const origin = typeof window !== 'undefined' ? window.location.origin : '';
		return `ANTHROPIC_BASE_URL="${origin}" \\
ANTHROPIC_AUTH_TOKEN="${token}" \\
claude -p --model haiku \\
  --system-prompt "You can only output bool" \\
  --tools "StructuredOutput" \\
  --setting-sources "" \\
  --strict-mcp-config \\
  --mcp-config '{"mcpServers":{}}' \\
  --disable-slash-commands \\
  --output-format stream-json \\
  --verbose \\
  --json-schema '{"type":"object","properties":{"result":{"type":"boolean"}},"required":["result"]}' \\
  "Print 'true' for testing purpose"`;
	}

	async function copyCmd() {
		if (!createdUser) return;
		await navigator.clipboard.writeText(buildTestCmd(createdUser.token));
		copied = true;
		setTimeout(() => { copied = false; }, 2000);
	}

	function windowAt(a: AccountView, index: number) {
		return a.windows[index] ?? null;
	}

	function groupAccounts(accounts: AccountView[]): AccountGroup[] {
		const groups = new Map<string, AccountGroup>();
		for (const account of accounts) {
			let group = groups.get(account.provider);
			if (!group) {
				group = {
					provider: account.provider,
					accounts: [],
					window_labels: []
				};
				groups.set(account.provider, group);
			}
			group.accounts.push(account);
			account.windows.forEach((window, index) => {
				if (!group!.window_labels[index]) {
					group!.window_labels[index] = window.label;
				}
			});
		}
		return [...groups.values()].sort((a, b) => a.provider.localeCompare(b.provider));
	}

</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else if !data}
	<p class="loading">loading dashboard...</p>
{:else}
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
	<div class="sub">{data.health.version} &middot; up {data.health.uptime} &middot; sqlite <span class={data.health.sqlite === 'ok' ? 'g' : 'r'}>{data.health.sqlite}</span></div>

	<!-- Usage -->
	<h2>usage</h2>
	{#if data.usage.length === 0}
		<p class="muted">no usage data yet</p>
	{:else}
		<table><thead>
			<tr>
				<th></th>
				<th class="num">requests</th>
				<th class="num">input</th>
				<th class="num">output</th>
				<th class="num">cache read</th>
				<th class="num">cost</th>
			</tr></thead><tbody>
			{#each data.usage as u, i (u.label)}
				<tr>
					<td>{u.label}</td>
					<td class="num">{fmtNum(u.requests)}</td>
					<td class="num">{fmtNum(u.input_tokens)}</td>
					<td class="num">{fmtNum(u.output_tokens)}</td>
					<td class="num">{fmtNum(u.cache_read_tokens)}</td>
					<td class="num">{#if i === data.usage.length - 1}<b>{fmtCost(u.cost_usd)}</b>{:else}{fmtCost(u.cost_usd)}{/if}</td>
				</tr>
			{/each}
		</tbody></table>
	{/if}

	{@const accountGroups = groupAccounts(data.accounts)}
	{#if accountGroups.length === 0}
		<h2>accounts</h2>
		<p class="muted">no accounts</p>
		{#if providerError}
			<p class="error-msg">{providerError}</p>
		{:else if providers.length > 0}
			<div class="bar">
				{#each providers as provider (provider.id)}
					<div style="margin-bottom:8px">
						<a href={addAccountPath(base, provider.id)}>[+ {provider.label}]</a>
					</div>
				{/each}
			</div>
		{/if}
	{:else}
		{#each accountGroups as group (group.provider)}
			<h2>{group.provider} accounts <a href={addAccountPath(base, group.provider)} class="add-link">[+ add]</a></h2>
			<table><thead>
				<tr>
					<th>email</th>
					<th>status</th>
					<th>pri</th>
					<th>cooldown</th>
					<th>last used</th>
					<th class="num">{group.window_labels[0] || 'window 1'}</th>
					<th class="num">{group.window_labels[1] || 'window 2'}</th>
				</tr></thead><tbody>
				{#each group.accounts as a (a.id)}
					{@const primary = windowAt(a, 0)}
					{@const secondary = windowAt(a, 1)}
					<tr>
						<td><a href="{base}/accounts/{a.id}">{a.email}</a></td>
						<td><span class={dotClass(a.status)}>{a.status}</span></td>
						<td>{a.priority}{#if a.priority_mode === 'auto'} <span class="muted">(a)</span>{/if}</td>
						<Countdown until={a.cooldown_until} tag="td" variant="cooldown" />
						<td>{timeAgo(a.last_used_at ?? '')}</td>
						<td class="num">{#if a.status === 'blocked' || a.status === 'disabled'}<span class="muted">&ndash;</span>{:else if primary}{@const remain = 100 - primary.pct}<span class={remainClass(remain)}>{remain}%</span> <span class="muted">{remainTime(primary.reset ?? null)}</span>{:else}<span class="muted">&ndash;</span>{/if}</td>
						<td class="num">{#if a.status === 'blocked' || a.status === 'disabled'}<span class="muted">&ndash;</span>{:else if secondary}{@const remain = 100 - secondary.pct}<span class={remainClass(remain)}>{remain}%</span> <span class="muted">{remainTime(secondary.reset ?? null)}</span>{:else}<span class="muted">&ndash;</span>{/if}</td>
					</tr>
				{/each}
			</tbody></table>
		{/each}
	{/if}

	<!-- Users -->
	<h2>users
		{#if !showAddUser && !createdUser}
			<button class="link add-link" onclick={() => { showAddUser = true; }}>[+ add]</button>
		{/if}
	</h2>

	{#if showAddUser}
		<div class="bar">
			<input
				type="text"
				placeholder="username"
				bind:value={newUserName}
				onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') createUser(); if (e.key === 'Escape') cancelAddUser(); }}
				disabled={creatingUser}
				style="width:160px;max-width:160px;margin-right:6px;"
			/>
			<button class="link" onclick={createUser} disabled={creatingUser}>[create]</button>
			<button class="link" onclick={cancelAddUser} disabled={creatingUser}>[cancel]</button>
			{#if addUserError}<span class="r" style="margin-left:8px;">{addUserError}</span>{/if}
		</div>
	{/if}

	{#if createdUser}
		<div class="bar">
			<div><span class="g">&#10003; created</span> <b>{createdUser.name}</b> &mdash; token: <span style="user-select:all;">{createdUser.token}</span> <button class="link" onclick={() => { createdUser = null; copied = false; }}>[dismiss]</button></div>
			<div class="r" style="font-size:11px;margin-top:2px;">copy now — this token will not be shown again</div>
			<div class="test-cmd-wrap">
				<button class="copy-btn" onclick={copyCmd}>{copied ? '[copied]' : '[copy]'}</button>
				<pre class="test-cmd">{buildTestCmd(createdUser.token)}</pre>
			</div>
		</div>
	{/if}

	{#if data.users.length === 0}
		<p class="muted">no users</p>
	{:else}
		<table><thead>
			<tr>
				<th>name</th>
				<th>status</th>
				<th>last active</th>
				<th class="num">total cost</th>
			</tr></thead><tbody>
			{#each data.users as u (u.id)}
				<tr>
					<td><a href="{base}/users/{u.id}">{u.name}</a></td>
					<td><span class={dotClass(u.status)}>{u.status}</span></td>
					<td>{timeAgo(u.last_active_at ?? '')}</td>
					<td class="num {u.total_cost === 0 ? 'muted' : ''}">{fmtCost(u.total_cost)}</td>
				</tr>
			{/each}
		</tbody></table>
	{/if}

	<!-- Recent errors -->
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
	.test-cmd-wrap {
		position: relative;
		margin-top: 6px;
	}
	.test-cmd {
		background: #1a1a1a;
		color: #aaa;
		padding: 8px 10px;
		font: 12px/1.6 monospace;
		overflow-x: auto;
		white-space: pre-wrap;
		word-break: break-all;
		border-radius: 3px;
	}
	.copy-btn {
		position: absolute;
		top: 4px;
		right: 4px;
		background: none;
		border: none;
		color: #6b9bff;
		font: 11px monospace;
		cursor: pointer;
		padding: 2px 6px;
		margin: 0;
	}
	.copy-btn:hover {
		text-decoration: underline;
	}
</style>
