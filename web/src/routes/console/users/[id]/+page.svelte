<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem, RecentRequestLog, UserSurface } from '$lib/admin-types';
	import { timeAgo, fmtNum, fmtCost, fmtDate, fmtJSON, statusColor, shortModel } from '$lib/format';
	import ConfirmAction from '$lib/components/ConfirmAction.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { egressLabel, providerLabel, roleLabel, surfaceLabel } from '$lib/admin-i18n';

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

	interface UserDetail {
		id: string;
		name: string;
		email?: string;
		role?: 'admin' | 'user';
		token_prefix: string;
		status: string;
		allowed_surface: UserSurface;
		bound_account_id?: string;
		bound_account_email?: string;
		created_at: string;
		last_active_at: string | null;
		usage: UsagePeriod[];
		model_usage: ModelUsageRow[];
		recent_requests: RecentRequestLog[];
	}

	let user = $state<UserDetail | null>(null);
	let accounts = $state<AccountListItem[]>([]);
	let error = $state('');
	let loading = $state(true);
	let newToken = $state('');
	let actionError = $state('');
	let selectedSurface = $state<UserSurface>('native');
	let selectedBoundAccountID = $state('');
	let editingPolicy = $state(false);
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
			editingPolicy = false;
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
			goto(`${base}/console/users`);
		} catch (e: any) {
			actionError = e.message;
		}
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

	function policyChanged(): boolean {
		if (!user) return false;
		return user.allowed_surface !== selectedSurface || (user.bound_account_id ?? '') !== selectedBoundAccountID;
	}

	function boundAccountText(): string {
		if (!user) return '-';
		if (user.bound_account_email) return user.bound_account_email;
		if (user.bound_account_id) return user.bound_account_id;
		return '未绑定账号';
	}

	function startPolicyEdit() {
		if (!user) return;
		selectedSurface = user.allowed_surface ?? 'native';
		selectedBoundAccountID = user.bound_account_id ?? '';
		policyError = '';
		policyResult = '';
		editingPolicy = true;
	}

	function cancelPolicyEdit() {
		if (!user) return;
		selectedSurface = user.allowed_surface;
		selectedBoundAccountID = user.bound_account_id ?? '';
		policyError = '';
		policyResult = '';
		editingPolicy = false;
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
			policyResult = '策略已保存';
			editingPolicy = false;
		} catch (e: any) {
			policyError = e.message;
		} finally {
			savingPolicy = false;
		}
	}

	function requestOutcome(request: RecentRequestLog): string {
		const parts: string[] = [];
		if (request.effect_kind) parts.push(request.effect_kind);
		if (request.upstream_status) parts.push(String(request.upstream_status));
		return parts.join(' / ') || request.status;
	}

	function requestAccountLabel(request: RecentRequestLog): string {
		return accounts.find((account) => account.id === request.account_id)?.email || '-';
	}

	function requestError(request: RecentRequestLog): string {
		const parts: string[] = [];
		if (request.upstream_error_type) parts.push(request.upstream_error_type);
		if (request.upstream_error_message) parts.push(request.upstream_error_message);
		return parts.join(': ') || '-';
	}

	function hasRequestDetails(request: RecentRequestLog): boolean {
		return !!(
			request.session_uuid ||
			request.binding_source ||
			request.upstream_error_type ||
			request.upstream_error_message ||
			request.client_body_excerpt ||
			request.request_meta ||
			request.client_headers ||
			request.upstream_url ||
			request.upstream_request_headers ||
			request.upstream_request_meta ||
			request.upstream_request_body_excerpt ||
			request.upstream_headers ||
			request.upstream_response_meta ||
			request.upstream_response_body_excerpt
		);
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
	<p><a href="{base}/console/users">&larr; 返回用户列表</a></p>
{:else if loading}
	<p class="loading">正在加载用户...</p>
{:else if user}
	{@const usage = user.usage ?? []}
	{@const models = user.model_usage ?? []}
	{@const requests = user.recent_requests ?? []}

	<div class="page-header">
		<div>
			<div class="eyebrow">用户详情</div>
			<h1>{user.name}</h1>
			<p class="lede mono">{user.id}</p>
		</div>
		<div class="page-actions">
			<StatusBadge status={user.status} />
			<ConfirmAction label="重新生成 token" onclick={regenerateToken} />
			<button class="link {user.status === 'active' ? 'r' : 'g'}" onclick={toggleStatus}>
				{user.status === 'active' ? '停用' : '启用'}
			</button>
			<ConfirmAction label="删除" cls="r" onclick={deleteUser} />
		</div>
	</div>

	{#if actionError}<p class="error-msg">{actionError}</p>{/if}

	{#if newToken}
		<section class="panel">
			<div class="section-header flush">
				<h2>新 Token</h2>
				<p class="hint">这个 token 只显示一次。</p>
			</div>
			<div class="copy-value mono">{newToken}</div>
		</section>
	{/if}

	<dl>
		<dt>邮箱</dt>
		<dd>{user.email ?? '-'}</dd>

		<dt>角色</dt>
		<dd>{roleLabel(user.role)}</dd>

		<dt>token 前缀</dt>
		<dd>{user.token_prefix} <span class="muted">（完整 token 不落库）</span></dd>

		<dt>创建时间</dt>
		<dd>{fmtDate(user.created_at)}</dd>

		<dt>最近活跃</dt>
		<dd>{#if user.last_active_at}{timeAgo(user.last_active_at)} ({new Date(user.last_active_at).toLocaleTimeString('en-GB', { hour12: false })}){:else}<span class="muted">-</span>{/if}</dd>
	</dl>

	<h2>访问策略</h2>
	<dl>
		<dt>接口面</dt>
		<dd>
			{#if editingPolicy}
				<select bind:value={selectedSurface} disabled={savingPolicy}>
					<option value="native">Responses</option>
					<option value="compat">兼容层</option>
					<option value="all">全部</option>
				</select>
			{:else}
				{surfaceLabel(user.allowed_surface)}
			{/if}
		</dd>

		<dt>绑定账号</dt>
		<dd>
			{#if editingPolicy}
				<select bind:value={selectedBoundAccountID} disabled={savingPolicy} style="max-width:420px;">
					<option value="">不绑定账号</option>
					{#if user.bound_account_id && !accounts.some((account) => account.id === user.bound_account_id)}
						<option value={user.bound_account_id}>{user.bound_account_email || user.bound_account_id}</option>
					{/if}
					{#each accounts as account (account.id)}
						<option value={account.id}>{accountLabel(account)}</option>
					{/each}
				</select>
			{:else}
				<span class:muted={!user.bound_account_id}>{boundAccountText()}</span>
			{/if}
		</dd>
	</dl>

	<div class="actions" style="margin-top:0">
		{#if editingPolicy}
			<button class="link" onclick={savePolicy} disabled={savingPolicy || !policyChanged()}>
				{savingPolicy ? '保存中...' : '保存策略'}
			</button>
			<button class="link" onclick={cancelPolicyEdit} disabled={savingPolicy}>
				取消
			</button>
		{:else}
			<button class="link" onclick={startPolicyEdit}>编辑</button>
		{/if}
	</div>

	{#if policyError}<p class="error-msg">{policyError}</p>{/if}
	{#if policyResult}<p class="muted">{policyResult}</p>{/if}

	<h2>用量</h2>
	{#if usage.length === 0}
		<p class="muted">暂无用量数据</p>
	{:else}
		<div class="table-wrap">
		<table><thead>
			<tr>
				<th></th>
				<th class="num">请求</th>
				<th class="num">输入</th>
				<th class="num">输出</th>
				<th class="num">缓存读取</th>
				<th class="num">成本</th>
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
		</div>
	{/if}

	<h2>按模型统计（7 天）</h2>
	{#if models.length === 0}
		<p class="muted">暂无模型数据</p>
	{:else}
		<div class="table-wrap">
		<table><thead>
			<tr>
				<th>模型</th>
				<th class="num">请求</th>
				<th class="num">输入</th>
				<th class="num">输出</th>
				<th class="num">缓存读取</th>
				<th class="num">成本</th>
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
		</div>
	{/if}

	<h2>最近请求</h2>
	{#if requests.length === 0}
		<p class="muted">暂无最近请求</p>
	{:else}
		<div class="table-wrap">
		<table><thead>
			<tr>
				<th>时间</th>
				<th>接口面</th>
				<th>模型</th>
				<th>路径</th>
				<th class="num">输入</th>
				<th class="num">输出</th>
				<th class="num">缓存读/写</th>
				<th>账号</th>
				<th>节点</th>
				<th>结果</th>
				<th>错误</th>
				<th class="num">耗时</th>
				<th>详情</th>
			</tr></thead><tbody>
			{#each requests as r (r.id)}
				<tr>
					<td class="muted">{new Date(r.created_at).toLocaleTimeString('en-GB', { hour12: false })}</td>
					<td>{surfaceLabel(r.surface)}</td>
					<td>{shortModel(r.model)}</td>
					<td>{r.path || '-'}</td>
					<td class="num">{fmtNum(r.input_tokens)}</td>
					<td class="num">{fmtNum(r.output_tokens)}</td>
					<td class="num">{fmtNum(r.cache_read_tokens)} / {fmtNum(r.cache_create_tokens)}</td>
					<td>{requestAccountLabel(r)}</td>
					<td>{egressLabel(r.cell_id)}</td>
					<td class={statusColor(r.status)}>{requestOutcome(r)}</td>
					<td>{requestError(r)}</td>
					<td class="num">{r.duration_ms > 0 ? (r.duration_ms / 1000).toFixed(1) + 's' : '-'}</td>
					<td>
						{#if hasRequestDetails(r)}
							<details>
								<summary>查看</summary>
								<div class="detail-block">
									<div><span class="muted">完整账号</span> <span class="mono">{r.account_id}</span></div>
									<div><span class="muted">session</span> <span class="mono">{r.session_uuid || '-'}</span></div>
									<div><span class="muted">绑定</span> {r.binding_source || '-'}</div>
									<div><span class="muted">错误</span> {requestError(r)}</div>
									<div><span class="muted">客户端 body</span><pre>{r.client_body_excerpt || '-'}</pre></div>
									<div><span class="muted">请求 meta</span><pre>{fmtJSON(r.request_meta)}</pre></div>
									<div><span class="muted">客户端 headers</span><pre>{fmtJSON(r.client_headers)}</pre></div>
									<div><span class="muted">上游 URL</span> <span class="mono">{r.upstream_url || '-'}</span></div>
									<div><span class="muted">上游请求 headers</span><pre>{fmtJSON(r.upstream_request_headers)}</pre></div>
									<div><span class="muted">上游请求 meta</span><pre>{fmtJSON(r.upstream_request_meta)}</pre></div>
									<div><span class="muted">上游请求 body</span><pre>{r.upstream_request_body_excerpt || '-'}</pre></div>
									<div><span class="muted">上游响应 headers</span><pre>{fmtJSON(r.upstream_headers)}</pre></div>
									<div><span class="muted">上游响应 meta</span><pre>{fmtJSON(r.upstream_response_meta)}</pre></div>
									<div><span class="muted">上游响应 body</span><pre>{r.upstream_response_body_excerpt || '-'}</pre></div>
								</div>
							</details>
						{:else}
							<span class="muted">-</span>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody></table>
		</div>
	{/if}

	<p class="sub"><a href="{base}/console/users">返回用户列表</a></p>
{/if}

<style>
	.mono {
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
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
