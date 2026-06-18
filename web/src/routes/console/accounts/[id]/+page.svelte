<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { EgressCellSummary, EgressCellView } from '$lib/admin-types';
	import { timeAgo, fmtDate, dotClass, remainClass, remainTime } from '$lib/format';
	import Countdown from '$lib/components/Countdown.svelte';
	import PriorityEditor from '$lib/components/PriorityEditor.svelte';
	import ConfirmAction from '$lib/components/ConfirmAction.svelte';
	import { providerLabel, statusLabel } from '$lib/admin-i18n';

	interface AccountDetail {
		id: string;
		email: string;
		provider: string;
		subject: string;
		status: string;
		probe_label: string;
		weight: number;
		weight_mode: string;
		auto_weight: number;
		error_message: string;
		provider_fields: { label: string; value: string }[];
		created_at: string;
		last_used_at: string | null;
		last_refresh_at: string | null;
		expires_at: number;
		cooldown_until: string | null;
		cell_id?: string;
		cell?: EgressCellSummary | null;
		windows: { label: string; pct: number; reset?: number }[];
		stainless: Record<string, string> | null;
		sessions: { session_uuid: string; account_id: string; created_at: string; last_used_at: string; expires_at: string }[];
	}

	let acct = $state<AccountDetail | null>(null);
	let cells = $state<EgressCellView[]>([]);
	let error = $state('');
	let loading = $state(true);
	let testResult = $state<{ ok: boolean; latency_ms: number; error?: string; time?: string } | null>(null);
	let testing = $state(false);
	let actionError = $state('');
	let selectedCellID = $state('');
	let editingCellBinding = $state(false);
	let savingCell = $state(false);
	let cellResult = $state('');

	$effect(() => {
		loadAccount();
	});

	async function loadAccount() {
		loading = true;
		error = '';
		try {
			const [accountData, cellList] = await Promise.all([
				api<AccountDetail>('/accounts/' + $page.params.id),
				api<EgressCellView[]>('/egress/cells')
			]);
			acct = accountData;
			cells = cellList;
			selectedCellID = accountData.cell_id ?? '';
			editingCellBinding = false;
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function testAccount() {
		if (!acct) return;
		testing = true;
		testResult = null;
		try {
			const data = await api<{ ok: boolean; latency_ms: number; error?: string }>(`/accounts/${acct.id}/test`, { method: 'POST' });
			testResult = { ...data, time: new Date().toLocaleTimeString('en-GB', { hour12: false }) };
		} catch (e: any) {
			testResult = { ok: false, latency_ms: 0, error: e.message, time: new Date().toLocaleTimeString('en-GB', { hour12: false }) };
		} finally {
			testing = false;
		}
	}

	async function forceRefresh() {
		if (!acct) return;
		actionError = '';
		try {
			await api(`/accounts/${acct.id}/refresh`, { method: 'POST' });
			await loadAccount();
		} catch (e: any) {
			actionError = e.message;
		}
	}

	async function toggleStatus() {
		if (!acct) return;
		const newStatus = acct.status === 'disabled' ? 'active' : 'disabled';
		actionError = '';
		try {
			await api(`/accounts/${acct.id}/status`, {
				method: 'POST',
				body: JSON.stringify({ status: newStatus })
			});
			acct.status = newStatus;
		} catch (e: any) {
			actionError = e.message;
		}
	}

	async function deleteAccount() {
		if (!acct) return;
		actionError = '';
		try {
			await api(`/accounts/${acct.id}`, { method: 'DELETE' });
			goto(`${base}/console/accounts`);
		} catch (e: any) {
			actionError = e.message;
		}
	}

	async function unbindSession(uuid: string) {
		if (!acct) return;
		actionError = '';
		try {
			await api(`/sessions/binding/${uuid}`, { method: 'DELETE' });
			acct.sessions = acct.sessions.filter((s) => s.session_uuid !== uuid);
		} catch (e: any) {
			actionError = e.message;
		}
	}

	function handleWeightUpdate(mode: string, weight: number) {
		if (!acct) return;
		acct.weight_mode = mode;
		acct.weight = weight;
	}

	function cooldownActive(cell: EgressCellSummary | EgressCellView | null | undefined): boolean {
		return !!cell?.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now();
	}

	function activeCooldownUntil(cell: EgressCellSummary | EgressCellView | null | undefined): string | null {
		return cooldownActive(cell) ? cell?.cooldown_until ?? null : null;
	}

	function cellSelectable(cell: EgressCellView): boolean {
		return cell.status === 'active' && !cooldownActive(cell) && !!cell.proxy?.host && !!cell.proxy?.port;
	}

	function cellAccounts(cell: EgressCellView | null | undefined) {
		return cell?.accounts ?? [];
	}

	function cellAvailableForAccount(cell: EgressCellView): boolean {
		if (cell.id === acct?.cell_id) return true;
		if (cell.proxy?.type === 'socks5') return cellSelectable(cell);
		const sameProvider = cellAccounts(cell).filter((a) => a.provider === acct?.provider);
		return cellSelectable(cell) && sameProvider.length === 0;
	}

	function bindableCells(): EgressCellView[] {
		return cells.filter(cellAvailableForAccount);
	}

	function selectedCell(): EgressCellView | undefined {
		return cells.find((cell) => cell.id === selectedCellID);
	}

	function region(cell: EgressCellSummary | EgressCellView | null | undefined): string {
		const labels = cell?.labels ?? {};
		return [labels.country, labels.city].filter(Boolean).join(' / ') || labels.site || '-';
	}

	function bindingChanged(): boolean {
		return (acct?.cell_id ?? '') !== selectedCellID;
	}

	function startCellBindingEdit() {
		selectedCellID = acct?.cell_id ?? '';
		actionError = '';
		cellResult = '';
		editingCellBinding = true;
	}

	function cancelCellBindingEdit() {
		selectedCellID = acct?.cell_id ?? '';
		actionError = '';
		cellResult = '';
		editingCellBinding = false;
	}

	function optionLabel(cell: EgressCellView): string {
		const parts = [cell.name || cell.id];
		const cellRegion = region(cell);
		if (cellRegion !== '-') parts.push(cellRegion);
		if (cooldownActive(cell)) {
			parts.push(statusLabel('cooling'));
		} else if (cell.status !== 'active') {
			parts.push(statusLabel(cell.status));
		}
		return parts.join(' / ');
	}

	async function saveCellBinding() {
		if (!acct) return;
		savingCell = true;
		actionError = '';
		cellResult = '';
		try {
			await api(`/accounts/${acct.id}/cell`, {
				method: 'POST',
				body: JSON.stringify({ cell_id: selectedCellID })
			});
			cellResult = selectedCellID ? `已绑定 ${acct.email} -> ${selectedCellID}` : `已解除 ${acct.email} 的节点绑定`;
			await loadAccount();
		} catch (e: any) {
			actionError = e.message;
		} finally {
			savingCell = false;
		}
	}

	let editingEmail = $state(false);
	let emailInput = $state('');
	let savingEmail = $state(false);
	let emailError = $state('');

	async function saveEmail() {
		if (!acct) return;
		const trimmed = emailInput.trim();
		if (!trimmed) return;
		savingEmail = true;
		emailError = '';
		try {
			const res = await api<{ email: string }>(`/accounts/${acct.id}/email`, {
				method: 'POST',
				body: JSON.stringify({ email: trimmed })
			});
			acct.email = res.email;
			editingEmail = false;
		} catch (e: any) {
			emailError = e.message;
		} finally {
			savingEmail = false;
		}
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
	<p><a href="{base}/console/accounts">&larr; 返回账号池</a></p>
{:else if loading}
	<p class="loading">正在加载账号...</p>
{:else if acct}
	<div class="page-header">
		<div>
			<div class="eyebrow">账号详情</div>
			<h1>{acct.email}</h1>
			<p class="lede">{providerLabel(acct.provider)} / {acct.id}</p>
		</div>
		<div class="page-actions">
			<button class="link" onclick={testAccount} disabled={testing}>{testing ? '测试中...' : '测试'}</button>
			<button class="link" onclick={forceRefresh}>刷新 token</button>
			<button class="link {acct.status === 'disabled' ? 'g' : 'r'}" onclick={toggleStatus}>
				{acct.status === 'disabled' ? '启用' : '停用'}
			</button>
			<ConfirmAction label="删除" cls="r" onclick={deleteAccount} />
		</div>
	</div>

	{#if actionError}<p class="error-msg">{actionError}</p>{/if}

	{#if testResult}
		<div class="bar" style="margin-top:0">
			{#if testResult.ok}
				<span class="g">正常</span> / {acct.probe_label} / {(testResult.latency_ms / 1000).toFixed(1)}s / <span class="muted">{testResult.time}</span>
			{:else}
				<span class="r">失败</span> / {testResult.error} / <span class="muted">{testResult.time}</span>
			{/if}
		</div>
	{/if}

	<h2>基本信息</h2>
	<dl>
		<dt>id</dt>
		<dd class="muted">{acct.id}</dd>

		<dt>状态</dt>
		<dd><span class={dotClass(acct.status)}>{statusLabel(acct.status)}</span></dd>

		<dt>subject</dt>
		<dd class="muted">{acct.subject || '-'}</dd>

		<dt>邮箱</dt>
		<dd>
			{#if editingEmail}
				<input
					id="account-email"
					name="account-email"
					type="text"
					bind:value={emailInput}
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
					data-1p-ignore="true"
					data-lpignore="true"
					data-bwignore="true"
					onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') saveEmail(); if (e.key === 'Escape') { editingEmail = false; emailError = ''; } }}
					maxlength={100}
					class="email-edit"
				>
				<button class="link compact-action" onclick={saveEmail} disabled={savingEmail}>{savingEmail ? '保存中...' : '保存'}</button>
				<button class="link compact-action" onclick={() => { editingEmail = false; emailError = ''; }}>取消</button>
			{:else}
				{acct.email}
				<button class="link compact-action" onclick={() => { emailInput = acct!.email; editingEmail = true; emailError = ''; }}>编辑</button>
			{/if}
			{#if emailError}<span class="error-msg">{emailError}</span>{/if}
		</dd>

		{#if acct.provider_fields.length > 0}
			{#each acct.provider_fields as field (field.label)}
				<dt>{field.label}</dt>
				<dd>{field.value}</dd>
			{/each}
		{/if}

		<dt>权重</dt>
		<dd>
			<PriorityEditor
				accountId={acct.id}
				weight={acct.weight}
				weightMode={acct.weight_mode}
				autoWeight={acct.auto_weight}
				onupdate={handleWeightUpdate}
			/>
		</dd>

		<dt>创建时间</dt>
		<dd>{fmtDate(acct.created_at)}</dd>
	</dl>

	<h2>token</h2>
	<dl>
		<dt>过期倒计时</dt>
		<dd><Countdown until={acct.expires_at} /></dd>

		<dt>最近刷新</dt>
		<dd>{#if acct.last_refresh_at}{timeAgo(acct.last_refresh_at)} <span class="muted">({new Date(acct.last_refresh_at).toLocaleTimeString('en-GB', { hour12: false })})</span>{:else}<span class="muted">-</span>{/if}</dd>
	</dl>

	<h2>调度</h2>
	<dl>
		<dt>冷却</dt>
		<dd>
			<Countdown until={acct.cooldown_until} variant="cooldown" />
		</dd>

		{#if acct.windows.length > 0}
			{#each acct.windows as window (window.label)}
				<dt>{window.label}</dt>
				<dd>
					{#if acct.status === 'blocked' || acct.status === 'disabled'}
						<span class="muted">&ndash;</span>
					{:else}
						{@const remain = 100 - window.pct}
						<span class={remainClass(remain)}>{remain}%</span>
						<span class="muted">{remainTime(window.reset ?? null)}</span>
					{/if}
				</dd>
			{/each}
		{:else}
			<dt>额度窗口</dt>
			<dd><span class="muted">-</span></dd>
		{/if}

		<dt>最近使用</dt>
		<dd>{#if acct.last_used_at}{timeAgo(acct.last_used_at)} <span class="muted">({new Date(acct.last_used_at).toLocaleTimeString('en-GB', { hour12: false })})</span>{:else}<span class="muted">-</span>{/if}</dd>

		<dt>错误</dt>
		<dd>
			{#if acct.error_message}
				<span class="r">{acct.error_message}</span>
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>
	</dl>

	<h2>出口</h2>
	<dl>
		<dt>绑定</dt>
		<dd>
			{#if editingCellBinding}
				<select bind:value={selectedCellID} style="margin-right:8px;max-width:320px;">
					<option value="">直连</option>
					{#each bindableCells() as cell (cell.id)}
						<option value={cell.id}>{optionLabel(cell)}</option>
					{/each}
				</select>
				<button class="link" onclick={saveCellBinding} disabled={savingCell || !bindingChanged()}>
					{savingCell ? '保存中...' : '保存'}
				</button>
				<button class="link" onclick={cancelCellBindingEdit} disabled={savingCell} style="margin-left:6px">
					取消
				</button>
			{:else}
				{#if acct.cell_id}
					<a href="{base}/console/cells/{acct.cell_id}">{acct.cell?.name ?? acct.cell_id}</a>
				{:else}
					<span class="muted">直连</span>
				{/if}
				<button class="link compact-action" onclick={startCellBindingEdit}>编辑</button>
			{/if}
		</dd>

		<dt>节点</dt>
		<dd>
			{#if acct.cell_id}
				<a href="{base}/console/cells/{acct.cell_id}">{acct.cell?.name ?? acct.cell_id}</a>
			{:else}
				<span class="muted">直连</span>
			{/if}
		</dd>

		{#if editingCellBinding}
			<dt>目标</dt>
			<dd>
				{#if selectedCell()}
					{selectedCell()?.name || selectedCell()?.id}
				{:else}
					<span class="muted">直连</span>
				{/if}
			</dd>

			<dt>目标地区</dt>
			<dd>
				{#if selectedCell()}
					{region(selectedCell())}
				{:else}
					<span class="muted">-</span>
				{/if}
			</dd>

			<dt>目标状态</dt>
			<dd>
				{#if selectedCell()}
					{#if cooldownActive(selectedCell())}
						<span class="o">冷却中</span>
					{:else}
						<span class={selectedCell()?.status === 'active' ? 'g' : selectedCell()?.status === 'error' ? 'r' : 'muted'}>{statusLabel(selectedCell()?.status)}</span>
					{/if}
				{:else}
					<span class="muted">-</span>
				{/if}
			</dd>
		{/if}

		<dt>节点状态</dt>
		<dd>
			{#if acct.cell}
				{#if cooldownActive(acct.cell)}
					<span class="o">冷却中</span>
				{:else}
					<span class={acct.cell.status === 'active' ? 'g' : acct.cell.status === 'error' ? 'r' : 'muted'}>{statusLabel(acct.cell.status)}</span>
				{/if}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>节点冷却</dt>
		<dd>
			{#if activeCooldownUntil(acct.cell)}
				<Countdown until={activeCooldownUntil(acct.cell)!} variant="cooldown" />
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>节点标签</dt>
		<dd>
			{#if acct.cell?.labels && Object.keys(acct.cell.labels).length > 0}
				{Object.entries(acct.cell.labels).map(([key, value]) => `${key}=${value}`).join(' / ')}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>
	</dl>

	{#if cellResult}
		<div class="bar" style="margin-top:0"><span class="g">{cellResult}</span></div>
	{/if}

	{#if acct.stainless && Object.keys(acct.stainless).length > 0}
		<h2>Stainless 指纹</h2>
		<dl>
			{#each Object.entries(acct.stainless) as [key, value] (key)}
				<dt>{key}</dt>
				<dd>{value}</dd>
			{/each}
		</dl>
	{/if}

	<h2>绑定 session <span class="muted">({acct.sessions?.length ?? 0})</span></h2>
	{#if !acct.sessions?.length}
		<p class="muted">暂无活跃 session 绑定</p>
	{:else}
		<div class="table-wrap">
		<table><thead>
			<tr>
				<th>session uuid</th>
				<th>最近活跃</th>
				<th>TTL</th>
				<th></th>
			</tr></thead><tbody>
			{#each acct.sessions as s (s.session_uuid)}
				<tr>
					<td class="muted">{s.session_uuid}</td>
					<td>{timeAgo(s.last_used_at)}</td>
					<td><Countdown until={s.expires_at} /></td>
					<td><ConfirmAction label="解绑" onclick={() => unbindSession(s.session_uuid)} /></td>
				</tr>
			{/each}
		</tbody></table>
		</div>
	{/if}

	<p class="sub"><a href="{base}/console/accounts">返回账号池</a></p>
{/if}

<style>
	.compact-action {
		margin-left: 6px;
		padding: 4px 7px;
		font-size: 12px;
	}

	.email-edit {
		width: 260px;
		max-width: min(100%, 260px);
		padding: 6px 8px;
		font: 13px ui-monospace, SFMono-Regular, Menlo, monospace;
	}
</style>
