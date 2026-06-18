<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem, UserSummary, UserSurface, UserTotalCostsResponse } from '$lib/admin-types';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { fmtCost, timeAgo } from '$lib/format';
	import { egressLabel, providerLabel, roleLabel, surfaceLabel } from '$lib/admin-i18n';

	let users = $state<UserSummary[]>([]);
	let accounts = $state<AccountListItem[]>([]);
	let totalCosts = $state<Record<string, number>>({});
	let error = $state('');
	let lastRefresh = $state('');
	let loadingTotalCosts = $state(false);
	let totalCostRequestSeq = 0;

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
			const [userList, accountList] = await Promise.all([
				api<UserSummary[]>('/users'),
				api<AccountListItem[]>('/accounts').catch(() => [])
			]);
			users = userList;
			accounts = [...accountList].sort((a, b) => a.email.localeCompare(b.email));
			void loadUserTotalCosts(userList.map((user) => user.id));
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	async function loadUserTotalCosts(userIDs: string[]) {
		const requestSeq = ++totalCostRequestSeq;
		if (userIDs.length === 0) {
			totalCosts = {};
			loadingTotalCosts = false;
			return;
		}
		loadingTotalCosts = true;
		try {
			const params = new URLSearchParams({ ids: userIDs.join(',') });
			// Remote SQLite analytics can still exceed the default 15s fetch timeout.
			// Remove this override after indexed/preaggregated user-cost queries are consistently <15s.
			const result = await api<UserTotalCostsResponse>(`/users/total-costs?${params.toString()}`, { timeout: 30000 });
			if (requestSeq !== totalCostRequestSeq) return;
			totalCosts = result.totals;
		} catch {
			if (requestSeq !== totalCostRequestSeq) return;
			totalCosts = {};
		} finally {
			if (requestSeq === totalCostRequestSeq) {
				loadingTotalCosts = false;
			}
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
		const parts = [account.email, providerLabel(account.provider)];
		if (account.cell?.name) {
			parts.push(account.cell.name);
		} else if (account.cell_id) {
			parts.push(account.cell_id);
		} else {
			parts.push(egressLabel(account.cell_id));
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
			addUserError = '请填写用户名';
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
					last_active_at: null
				},
				...users
			];
			void loadUserTotalCosts(users.map((user) => user.id));
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

	function hasTotalCost(userID: string): boolean {
		return Object.prototype.hasOwnProperty.call(totalCosts, userID);
	}

	function totalCostText(userID: string): string {
		if (hasTotalCost(userID)) {
			return fmtCost(totalCosts[userID] ?? 0);
		}
		return loadingTotalCosts ? '...' : '-';
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else}
	<div class="page-header">
		<div>
			<div class="eyebrow">用户与密钥</div>
			<h1>用户</h1>
			<p class="lede">管理 API 用户、角色、接口面权限、绑定账号和历史转发成本。</p>
		</div>
		<div class="page-actions">
			{#if !showAddUser && !createdUser}
				<button class="primary-btn" onclick={() => { showAddUser = true; }}>新增用户</button>
			{/if}
			<button class="link" onclick={loadAll}>刷新</button>
			<span class="muted mono">{lastRefresh}</span>
		</div>
	</div>

	<div class="metric-grid">
		<MetricCard label="用户" value={users.length} sub={`${users.filter((user) => user.status === 'active').length} 个正常`} />
		<MetricCard label="管理员" value={users.filter((user) => user.role === 'admin').length} sub="ADMIN_EMAILS 命中" />
		<MetricCard label="Responses" value={users.filter((user) => user.allowed_surface === 'native').length} sub="仅 Responses 用户" />
		<MetricCard label="全部接口面" value={users.filter((user) => user.allowed_surface === 'all').length} sub="不限接口面" />
	</div>

	{#if showAddUser}
		<section class="panel form-panel">
			<div class="section-header flush">
				<h2>创建用户</h2>
			</div>
			<div class="form-row add-user-grid">
				<div>
					<label for="new-user-name">名称</label>
					<input
						id="new-user-name"
						type="text"
						placeholder="用户名"
						bind:value={newUserName}
						onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') createUser(); if (e.key === 'Escape') cancelAddUser(); }}
						disabled={creatingUser}
					/>
				</div>
				<div>
					<label for="new-user-surface">接口面</label>
					<select id="new-user-surface" bind:value={newAllowedSurface} disabled={creatingUser}>
						<option value="native">Responses</option>
						<option value="compat">兼容层</option>
						<option value="all">全部</option>
					</select>
				</div>
				<div>
					<label for="new-user-account">绑定账号</label>
					<select id="new-user-account" bind:value={newBoundAccountID} disabled={creatingUser}>
						<option value="">不绑定账号</option>
						{#each accounts as account (account.id)}
							<option value={account.id}>{accountLabel(account)}</option>
						{/each}
					</select>
				</div>
				<div class="page-actions form-actions">
					<button class="primary-btn" onclick={createUser} disabled={creatingUser}>{creatingUser ? '创建中...' : '创建'}</button>
					<button class="link" onclick={cancelAddUser} disabled={creatingUser}>取消</button>
				</div>
			</div>
			{#if addUserError}<p class="error-msg">{addUserError}</p>{/if}
		</section>
	{/if}

	{#if createdUser}
		<section class="panel">
			<div class="section-header flush">
				<div>
					<h2>用户已创建</h2>
					<p class="hint">{createdUser.name} 的 token 只显示一次。</p>
				</div>
				<div class="page-actions">
					<button class="link" onclick={copyCmd}>{copied ? '已复制' : '复制测试命令'}</button>
					<button class="link" onclick={() => { createdUser = null; copied = false; }}>关闭</button>
				</div>
			</div>
			<div class="copy-value mono">{createdUser.token}</div>
			<div class="test-cmd-wrap">
				<pre class="test-cmd">{buildKeyCheckCmd(createdUser.token)}</pre>
			</div>
		</section>
	{/if}

	{#if users.length === 0}
		<p class="muted">暂无用户</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>名称</th>
						<th>角色</th>
						<th>状态</th>
						<th>接口面</th>
						<th>绑定账号</th>
						<th>最近活跃</th>
						<th class="num">总成本</th>
					</tr>
				</thead>
				<tbody>
					{#each users as user (user.id)}
						<tr>
							<td><a href="{base}/console/users/{user.id}">{user.name}</a></td>
							<td>{roleLabel(user.role)}</td>
							<td><StatusBadge status={user.status} /></td>
							<td>{surfaceLabel(user.allowed_surface)}</td>
							<td class:muted={!user.bound_account_id}>{boundAccountText(user)}</td>
							<td>{timeAgo(user.last_active_at ?? '')}</td>
							<td class="num {hasTotalCost(user.id) && (totalCosts[user.id] ?? 0) === 0 ? 'muted' : ''}">{totalCostText(user.id)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
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

	.add-user-grid {
		grid-template-columns: minmax(180px, 1fr) minmax(140px, 180px) minmax(240px, 1.5fr) auto;
	}

	.form-actions {
		align-self: end;
		justify-content: flex-start;
	}

	@media (max-width: 840px) {
		.add-user-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
