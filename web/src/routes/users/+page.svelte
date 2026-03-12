<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem, DashboardData, UserSummary, UserSurface } from '$lib/admin-types';
	import { dotClass, fmtCost, timeAgo } from '$lib/format';

	let users = $state<UserSummary[]>([]);
	let accounts = $state<AccountListItem[]>([]);
	let error = $state('');
	let lastRefresh = $state('');

	let showAddUser = $state(false);
	let newUserName = $state('');
	let newAllowedSurface = $state<UserSurface>('native');
	let newBoundAccountID = $state('');
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
			const [dashboard, accountList] = await Promise.all([
				api<DashboardData>('/dashboard'),
				api<AccountListItem[]>('/accounts').catch(() => [])
			]);
			users = dashboard.users;
			accounts = [...accountList].sort((a, b) => a.email.localeCompare(b.email));
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	function cancelAddUser() {
		showAddUser = false;
		newUserName = '';
		newAllowedSurface = 'native';
		newBoundAccountID = '';
		addUserError = '';
	}

	function accountLabel(account: AccountListItem): string {
		const parts = [account.email, account.provider];
		if (account.cell?.name) {
			parts.push(account.cell.name);
		} else if (account.cell_id) {
			parts.push(account.cell_id);
		} else {
			parts.push('legacy direct');
		}
		return parts.join(' / ');
	}

	function boundAccountText(user: UserSummary): string {
		if (user.bound_account_email) return user.bound_account_email;
		if (user.bound_account_id) return user.bound_account_id;
		return '-';
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
			const res = await api<{
				id: string;
				name: string;
				token: string;
				allowed_surface: UserSurface;
				bound_account_id?: string;
				bound_account_email?: string;
			}>('/users', {
				method: 'POST',
				body: JSON.stringify({
					name,
					allowed_surface: newAllowedSurface,
					bound_account_id: newBoundAccountID
				})
			});
			createdUser = { name: res.name, token: res.token };
			users = [
				{
					id: res.id,
					name: res.name,
					status: 'active',
					allowed_surface: res.allowed_surface,
					bound_account_id: res.bound_account_id,
					bound_account_email: res.bound_account_email,
					last_active_at: null,
					total_cost: 0
				},
				...users
			];
			showAddUser = false;
			newUserName = '';
			newAllowedSurface = 'native';
			newBoundAccountID = '';
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
			<select bind:value={newAllowedSurface} disabled={creatingUser} style="margin-right:6px;">
				<option value="native">native</option>
				<option value="compat">compat</option>
				<option value="all">all</option>
			</select>
			<select bind:value={newBoundAccountID} disabled={creatingUser} style="max-width:320px;margin-right:6px;">
				<option value="">[no bound account]</option>
				{#each accounts as account (account.id)}
					<option value={account.id}>{accountLabel(account)}</option>
				{/each}
			</select>
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
					<th>surface</th>
					<th>bound</th>
					<th>last active</th>
					<th class="num">total cost</th>
				</tr>
			</thead>
			<tbody>
				{#each users as user (user.id)}
					<tr>
						<td><a href="{base}/users/{user.id}">{user.name}</a></td>
						<td><span class={dotClass(user.status)}>{user.status}</span></td>
						<td>{user.allowed_surface}</td>
						<td class:muted={!user.bound_account_id}>{boundAccountText(user)}</td>
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
