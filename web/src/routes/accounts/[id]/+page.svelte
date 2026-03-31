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
			goto(`${base}/accounts`);
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
			parts.push('cooling');
		} else if (cell.status !== 'active') {
			parts.push(cell.status);
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
			cellResult = selectedCellID ? `bound ${acct.email} -> ${selectedCellID}` : `unbound ${acct.email}`;
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
	<p><a href="{base}/accounts">&larr; back to accounts</a></p>
{:else if loading}
	<p class="loading">loading account...</p>
{:else if acct}
	<h2>{acct.email} <span class="muted">{acct.provider}</span></h2>

	<div class="actions">
		<button class="link" onclick={testAccount} disabled={testing}>{testing ? '[testing...]' : '[test]'}</button>
		<button class="link" onclick={forceRefresh}>[refresh token]</button>
		<button class="link {acct.status === 'disabled' ? 'g' : 'r'}" onclick={toggleStatus}>
			[{acct.status === 'disabled' ? 'enable' : 'disable'}]
		</button>
		<ConfirmAction label="[delete]" cls="r" onclick={deleteAccount} />
	</div>

	{#if actionError}<p class="error-msg">{actionError}</p>{/if}

	{#if testResult}
		<div class="bar" style="margin-top:0">
			{#if testResult.ok}
				<span class="g">&#10003; ok</span> &mdash; {acct.probe_label} &mdash; {(testResult.latency_ms / 1000).toFixed(1)}s &mdash; <span class="muted">{testResult.time}</span>
			{:else}
				<span class="r">&#10007; failed</span> &mdash; {testResult.error} &mdash; <span class="muted">{testResult.time}</span>
			{/if}
		</div>
	{/if}

	<h2>info</h2>
	<dl>
		<dt>id</dt>
		<dd class="muted">{acct.id}</dd>

		<dt>status</dt>
		<dd><span class={dotClass(acct.status)}>{acct.status}</span></dd>

		<dt>subject</dt>
		<dd class="muted">{acct.subject || '-'}</dd>

		<dt>email</dt>
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
					style="width:240px;font:13px monospace;border:1px solid #ccc;padding:0 4px;"
				>
				<button class="link" style="font-size:12px" onclick={saveEmail} disabled={savingEmail}>{savingEmail ? 'saving...' : '[save]'}</button>
				<button class="link" style="font-size:12px;margin-left:4px" onclick={() => { editingEmail = false; emailError = ''; }}>[cancel]</button>
			{:else}
				{acct.email}
				<button class="link" style="font-size:12px;margin-left:6px" onclick={() => { emailInput = acct!.email; editingEmail = true; emailError = ''; }}>[edit]</button>
			{/if}
			{#if emailError}<span class="error-msg">{emailError}</span>{/if}
		</dd>

		{#if acct.provider_fields.length > 0}
			{#each acct.provider_fields as field (field.label)}
				<dt>{field.label}</dt>
				<dd>{field.value}</dd>
			{/each}
		{/if}

		<dt>weight</dt>
		<dd>
			<PriorityEditor
				accountId={acct.id}
				weight={acct.weight}
				weightMode={acct.weight_mode}
				autoWeight={acct.auto_weight}
				onupdate={handleWeightUpdate}
			/>
		</dd>

		<dt>created</dt>
		<dd>{fmtDate(acct.created_at)}</dd>
	</dl>

	<h2>token</h2>
	<dl>
		<dt>expires in</dt>
		<dd><Countdown until={acct.expires_at} /></dd>

		<dt>last refreshed</dt>
		<dd>{#if acct.last_refresh_at}{timeAgo(acct.last_refresh_at)} <span class="muted">({new Date(acct.last_refresh_at).toLocaleTimeString('en-GB', { hour12: false })})</span>{:else}<span class="muted">-</span>{/if}</dd>
	</dl>

	<h2>scheduling</h2>
	<dl>
		<dt>cooldown</dt>
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
			<dt>windows</dt>
			<dd><span class="muted">-</span></dd>
		{/if}

		<dt>last used</dt>
		<dd>{#if acct.last_used_at}{timeAgo(acct.last_used_at)} <span class="muted">({new Date(acct.last_used_at).toLocaleTimeString('en-GB', { hour12: false })})</span>{:else}<span class="muted">-</span>{/if}</dd>

		<dt>error</dt>
		<dd>
			{#if acct.error_message}
				<span class="r">{acct.error_message}</span>
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>
	</dl>

	<h2>egress</h2>
	<dl>
		<dt>binding</dt>
		<dd>
			{#if editingCellBinding}
				<select bind:value={selectedCellID} style="margin-right:8px;max-width:320px;">
					<option value="">legacy direct</option>
					{#each bindableCells() as cell (cell.id)}
						<option value={cell.id}>{optionLabel(cell)}</option>
					{/each}
				</select>
				<button class="link" onclick={saveCellBinding} disabled={savingCell || !bindingChanged()}>
					{savingCell ? '[saving...]' : '[save]'}
				</button>
				<button class="link" onclick={cancelCellBindingEdit} disabled={savingCell} style="margin-left:6px">
					[cancel]
				</button>
			{:else}
				{#if acct.cell_id}
					<a href="{base}/cells/{acct.cell_id}">{acct.cell?.name ?? acct.cell_id}</a>
				{:else}
					<span class="muted">legacy direct</span>
				{/if}
				<button class="link" onclick={startCellBindingEdit} style="margin-left:6px">[edit]</button>
			{/if}
		</dd>

		<dt>cell</dt>
		<dd>
			{#if acct.cell_id}
				<a href="{base}/cells/{acct.cell_id}">{acct.cell?.name ?? acct.cell_id}</a>
			{:else}
				<span class="muted">legacy direct</span>
			{/if}
		</dd>

		{#if editingCellBinding}
			<dt>target</dt>
			<dd>
				{#if selectedCell()}
					{selectedCell()?.name || selectedCell()?.id}
				{:else}
					<span class="muted">legacy direct</span>
				{/if}
			</dd>

			<dt>target region</dt>
			<dd>
				{#if selectedCell()}
					{region(selectedCell())}
				{:else}
					<span class="muted">-</span>
				{/if}
			</dd>

			<dt>target status</dt>
			<dd>
				{#if selectedCell()}
					{#if cooldownActive(selectedCell())}
						<span class="o">cooling</span>
					{:else}
						<span class={selectedCell()?.status === 'active' ? 'g' : selectedCell()?.status === 'error' ? 'r' : 'muted'}>{selectedCell()?.status}</span>
					{/if}
				{:else}
					<span class="muted">-</span>
				{/if}
			</dd>
		{/if}

		<dt>cell status</dt>
		<dd>
			{#if acct.cell}
				{#if cooldownActive(acct.cell)}
					<span class="o">cooling</span>
				{:else}
					<span class={acct.cell.status === 'active' ? 'g' : acct.cell.status === 'error' ? 'r' : 'muted'}>{acct.cell.status}</span>
				{/if}
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>cell cooldown</dt>
		<dd>
			{#if activeCooldownUntil(acct.cell)}
				<Countdown until={activeCooldownUntil(acct.cell)!} variant="cooldown" />
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>cell labels</dt>
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
		<h2>stainless fingerprint</h2>
		<dl>
			{#each Object.entries(acct.stainless) as [key, value] (key)}
				<dt>{key}</dt>
				<dd>{value}</dd>
			{/each}
		</dl>
	{/if}

	<h2>bound sessions <span class="muted">({acct.sessions?.length ?? 0})</span></h2>
	{#if !acct.sessions?.length}
		<p class="muted">no active session bindings</p>
	{:else}
		<table><thead>
			<tr>
				<th>session uuid</th>
				<th>last active</th>
				<th>ttl</th>
				<th></th>
			</tr></thead><tbody>
			{#each acct.sessions as s (s.session_uuid)}
				<tr>
					<td class="muted">{s.session_uuid}</td>
					<td>{timeAgo(s.last_used_at)}</td>
					<td><Countdown until={s.expires_at} /></td>
					<td><ConfirmAction label="unbind" onclick={() => unbindSession(s.session_uuid)} /></td>
				</tr>
			{/each}
		</tbody></table>
	{/if}

	<p style="margin-top:16px;font-size:12px"><a href="{base}/accounts">&larr; back</a></p>
{/if}
