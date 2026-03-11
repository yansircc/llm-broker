<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { DashboardData, UserSummary } from '$lib/admin-types';
	import { dotClass, fmtCost, timeAgo } from '$lib/format';

	let users = $state<UserSummary[]>([]);
	let error = $state('');
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
		try {
			const dashboard = await api<DashboardData>('/dashboard');
			users = dashboard.users;
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
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
		if (!name) {
			addUserError = 'username required';
			return;
		}
		creatingUser = true;
		addUserError = '';
		try {
			const res = await api<{ id: string; name: string; token: string }>('/users', {
				method: 'POST',
				body: JSON.stringify({ name })
			});
			createdUser = { name: res.name, token: res.token };
			users = [{ id: res.id, name: res.name, status: 'active', last_active_at: null, total_cost: 0 }, ...users];
			showAddUser = false;
			newUserName = '';
		} catch (e: any) {
			addUserError = e.message;
		} finally {
			creatingUser = false;
		}
	}

	function buildKeyCheckCmd(token: string): string {
		const origin = typeof window !== 'undefined' ? window.location.origin : '';
		return `BASE_URL="${origin}"
API_KEY="${token}"

curl -fsS "$BASE_URL/v1/models" \\
  -H "Authorization: Bearer $API_KEY" \\
  >/dev/null && echo "key ok"`;
	}

	async function copyCmd() {
		if (!createdUser) return;
		await navigator.clipboard.writeText(buildKeyCheckCmd(createdUser.token));
		copied = true;
		setTimeout(() => { copied = false; }, 2000);
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else}
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
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
				<pre class="test-cmd">{buildKeyCheckCmd(createdUser.token)}</pre>
			</div>
		</div>
	{/if}

	{#if users.length === 0}
		<p class="muted">no users</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>name</th>
					<th>status</th>
					<th>last active</th>
					<th class="num">total cost</th>
				</tr>
			</thead>
			<tbody>
				{#each users as user (user.id)}
					<tr>
						<td><a href="{base}/users/{user.id}">{user.name}</a></td>
						<td><span class={dotClass(user.status)}>{user.status}</span></td>
						<td>{timeAgo(user.last_active_at ?? '')}</td>
						<td class="num {user.total_cost === 0 ? 'muted' : ''}">{fmtCost(user.total_cost)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
{/if}

<style>
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
