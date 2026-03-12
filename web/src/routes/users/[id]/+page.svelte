<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem, UserSurface } from '$lib/admin-types';
	import { timeAgo, fmtNum, fmtCost, fmtDate, tagClass, statusColor, shortModel } from '$lib/format';
	import ConfirmAction from '$lib/components/ConfirmAction.svelte';

	interface UsagePeriod {
		label: string;
		requests: number;
		input_tokens: number;
		output_tokens: number;
		cache_read_tokens: number;
		cost_usd: number;
	}

	interface ModelUsageRow {
		model: string;
		requests: number;
		input_tokens: number;
		output_tokens: number;
		cache_read_tokens: number;
		cost_usd: number;
	}

	interface RecentRequest {
		id: number;
		user_id: string;
		account_id: string;
		model: string;
		input_tokens: number;
		output_tokens: number;
		cache_read_tokens: number;
		cache_create_tokens: number;
		cost_usd: number;
		status: string;
		duration_ms: number;
		created_at: string;
	}

	interface UserDetail {
		id: string;
		name: string;
		token_prefix: string;
		status: string;
		allowed_surface: UserSurface;
		bound_account_id?: string;
		bound_account_email?: string;
		created_at: string;
		last_active_at: string | null;
		usage: UsagePeriod[];
		model_usage: ModelUsageRow[];
		recent_requests: RecentRequest[];
	}

	let user = $state<UserDetail | null>(null);
	let accounts = $state<AccountListItem[]>([]);
	let error = $state('');
	let loading = $state(true);
	let newToken = $state('');
	let actionError = $state('');
	let selectedSurface = $state<UserSurface>('native');
	let selectedBoundAccountID = $state('');
	let savingPolicy = $state(false);
	let policyError = $state('');
	let policyResult = $state('');

	$effect(() => {
		loadUser();
	});

	async function loadUser() {
		error = '';
		try {
			const [userData, accountList] = await Promise.all([
				api<UserDetail>('/users/' + $page.params.id),
				api<AccountListItem[]>('/accounts').catch(() => [])
			]);
			user = userData;
			accounts = [...accountList].sort((a, b) => a.email.localeCompare(b.email));
			selectedSurface = userData.allowed_surface ?? 'native';
			selectedBoundAccountID = userData.bound_account_id ?? '';
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function regenerateToken() {
		if (!user) return;
		actionError = '';
		try {
			const result = await api<{ token: string }>(`/users/${user.id}/regenerate`, { method: 'POST' });
			newToken = result.token;
		} catch (e: any) {
			actionError = e.message;
		}
	}

	async function toggleStatus() {
		if (!user) return;
		const newStatus = user.status === 'active' ? 'disabled' : 'active';
		actionError = '';
		try {
			await api(`/users/${user.id}/status`, {
				method: 'POST',
				body: JSON.stringify({ status: newStatus })
			});
			user.status = newStatus;
		} catch (e: any) {
			actionError = e.message;
		}
	}

	async function deleteUser() {
		if (!user) return;
		actionError = '';
		try {
			await api(`/users/${user.id}`, { method: 'DELETE' });
			goto(`${base}/users`);
		} catch (e: any) {
			actionError = e.message;
		}
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

	function policyChanged(): boolean {
		if (!user) return false;
		return user.allowed_surface !== selectedSurface || (user.bound_account_id ?? '') !== selectedBoundAccountID;
	}

	async function savePolicy() {
		if (!user) return;
		savingPolicy = true;
		policyError = '';
		policyResult = '';
		try {
			const result = await api<{
				id: string;
				allowed_surface: UserSurface;
				bound_account_id?: string;
				bound_account_email?: string;
			}>(`/users/${user.id}/policy`, {
				method: 'POST',
				body: JSON.stringify({
					allowed_surface: selectedSurface,
					bound_account_id: selectedBoundAccountID
				})
			});
			user.allowed_surface = result.allowed_surface;
			user.bound_account_id = result.bound_account_id;
			user.bound_account_email = result.bound_account_email;
			policyResult = 'policy saved';
		} catch (e: any) {
			policyError = e.message;
		} finally {
			savingPolicy = false;
		}
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
	<p><a href="{base}/users">&larr; back to users</a></p>
{:else if loading}
	<p class="loading">loading user...</p>
{:else if user}
	{@const usage = user.usage ?? []}
	{@const models = user.model_usage ?? []}
	{@const requests = user.recent_requests ?? []}

	<h2>{user.name} <span class={tagClass(user.status)}>{user.status}</span></h2>

	<div class="actions">
		<ConfirmAction label="[regenerate token]" onclick={regenerateToken} />
		<button class="link {user.status === 'active' ? 'r' : 'g'}" onclick={toggleStatus}>
			[{user.status === 'active' ? 'disable' : 'enable'}]
		</button>
		<ConfirmAction label="[delete]" cls="r" onclick={deleteUser} />
	</div>

	{#if actionError}<p class="error-msg">{actionError}</p>{/if}

	{#if newToken}
		<div class="bar">
			token: <b>{newToken}</b>
			<br><br>
			<span class="r">Copy now -- this token will not be shown again.</span>
			<br>
			Configure Claude Code:
			<br>
			&nbsp;&nbsp;export ANTHROPIC_API_KEY={newToken}
		</div>
	{/if}

	<dl>
		<dt>token prefix</dt>
		<dd>{user.token_prefix} <span class="muted">(full token not stored)</span></dd>

		<dt>created</dt>
		<dd>{fmtDate(user.created_at)}</dd>

		<dt>last active</dt>
		<dd>{#if user.last_active_at}{timeAgo(user.last_active_at)} ({new Date(user.last_active_at).toLocaleTimeString('en-GB', { hour12: false })}){:else}<span class="muted">-</span>{/if}</dd>
	</dl>

	<h2>policy</h2>
	<dl>
		<dt>surface</dt>
		<dd>
			<select bind:value={selectedSurface} disabled={savingPolicy}>
				<option value="native">native</option>
				<option value="compat">compat</option>
				<option value="all">all</option>
			</select>
		</dd>

		<dt>bound account</dt>
		<dd>
			<select bind:value={selectedBoundAccountID} disabled={savingPolicy} style="max-width:420px;">
				<option value="">[no bound account]</option>
				{#if user.bound_account_id && !accounts.some((account) => account.id === user.bound_account_id)}
					<option value={user.bound_account_id}>{user.bound_account_email || user.bound_account_id}</option>
				{/if}
				{#each accounts as account (account.id)}
					<option value={account.id}>{accountLabel(account)}</option>
				{/each}
			</select>
		</dd>
	</dl>

	<div class="actions" style="margin-top:0">
		<button class="link" onclick={savePolicy} disabled={savingPolicy || !policyChanged()}>
			{savingPolicy ? '[saving...]' : '[save policy]'}
		</button>
		<button
			class="link"
			onclick={() => {
				if (!user) return;
				selectedSurface = user.allowed_surface;
				selectedBoundAccountID = user.bound_account_id ?? '';
				policyError = '';
				policyResult = '';
			}}
			disabled={savingPolicy || !policyChanged()}
		>
			[reset]
		</button>
	</div>

	{#if policyError}<p class="error-msg">{policyError}</p>{/if}
	{#if policyResult}<p class="muted">{policyResult}</p>{/if}

	<!-- Usage periods -->
	<h2>usage</h2>
	{#if usage.length === 0}
		<p class="muted">no usage data</p>
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
			{#each usage as u, i (u.label)}
				<tr>
					<td>{u.label}</td>
					<td class="num">{fmtNum(u.requests)}</td>
					<td class="num">{fmtNum(u.input_tokens)}</td>
					<td class="num">{fmtNum(u.output_tokens)}</td>
					<td class="num">{fmtNum(u.cache_read_tokens)}</td>
					<td class="num">{#if i === usage.length - 1}<b>{fmtCost(u.cost_usd)}</b>{:else}{fmtCost(u.cost_usd)}{/if}</td>
				</tr>
			{/each}
		</tbody></table>
	{/if}

	<!-- Per-model breakdown -->
	<h2>by model (7 days)</h2>
	{#if models.length === 0}
		<p class="muted">no model data</p>
	{:else}
		<table><thead>
			<tr>
				<th>model</th>
				<th class="num">reqs</th>
				<th class="num">input</th>
				<th class="num">output</th>
				<th class="num">cache read</th>
				<th class="num">cost</th>
			</tr></thead><tbody>
			{#each models as m (m.model)}
				<tr>
					<td>{m.model}</td>
					<td class="num">{fmtNum(m.requests)}</td>
					<td class="num">{fmtNum(m.input_tokens)}</td>
					<td class="num">{fmtNum(m.output_tokens)}</td>
					<td class="num">{fmtNum(m.cache_read_tokens)}</td>
					<td class="num">{fmtCost(m.cost_usd)}</td>
				</tr>
			{/each}
		</tbody></table>
	{/if}

	<!-- Recent requests -->
	<h2>recent requests</h2>
	{#if requests.length === 0}
		<p class="muted">no recent requests</p>
	{:else}
		<table><thead>
			<tr>
				<th>time</th>
				<th>model</th>
				<th class="num">input</th>
				<th class="num">output</th>
				<th class="num">cache r/w</th>
				<th>account</th>
				<th>status</th>
				<th class="num">duration</th>
			</tr></thead><tbody>
			{#each requests as r (r.id)}
				<tr>
					<td class="muted">{new Date(r.created_at).toLocaleTimeString('en-GB', { hour12: false })}</td>
					<td>{shortModel(r.model)}</td>
					<td class="num">{fmtNum(r.input_tokens)}</td>
					<td class="num">{fmtNum(r.output_tokens)}</td>
					<td class="num">{fmtNum(r.cache_read_tokens)} / {fmtNum(r.cache_create_tokens)}</td>
					<td>{r.account_id}</td>
					<td class={statusColor(r.status)}>{r.status}</td>
					<td class="num">{r.duration_ms > 0 ? (r.duration_ms / 1000).toFixed(1) + 's' : '-'}</td>
				</tr>
			{/each}
		</tbody></table>
	{/if}

	<p style="margin-top:16px;font-size:12px"><a href="{base}/users">&larr; back</a></p>
{/if}
