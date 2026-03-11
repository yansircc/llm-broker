<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { EgressCellSummary } from '$lib/admin-types';
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
		priority: number;
		priority_mode: string;
		auto_score: number;
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
	let error = $state('');
	let loading = $state(true);
	let testResult = $state<{ ok: boolean; latency_ms: number; error?: string; time?: string } | null>(null);
	let testing = $state(false);
	let actionError = $state('');

	$effect(() => {
		loadAccount();
	});

	async function loadAccount() {
		try {
			acct = await api<AccountDetail>('/accounts/' + $page.params.id);
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

	function handlePriorityUpdate(mode: string, priority: number) {
		if (!acct) return;
		acct.priority_mode = mode;
		acct.priority = priority;
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
				<input type="text" bind:value={emailInput} onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') saveEmail(); if (e.key === 'Escape') { editingEmail = false; emailError = ''; } }} maxlength={100} style="width:240px;font:13px monospace;border:1px solid #ccc;padding:0 4px;">
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

		<dt>priority</dt>
		<dd>
			<PriorityEditor
				accountId={acct.id}
				priority={acct.priority}
				priorityMode={acct.priority_mode}
				autoScore={acct.auto_score}
				onupdate={handlePriorityUpdate}
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
		<dt>cell</dt>
		<dd>
			{#if acct.cell_id}
				<a href="{base}/cells/{acct.cell_id}">{acct.cell?.name ?? acct.cell_id}</a>
			{:else}
				<span class="muted">legacy direct</span>
			{/if}
		</dd>

		<dt>cell status</dt>
		<dd>
			{#if acct.cell}
				<span class={acct.cell.status === 'active' ? 'g' : acct.cell.status === 'error' ? 'r' : 'muted'}>{acct.cell.status}</span>
			{:else}
				<span class="muted">-</span>
			{/if}
		</dd>

		<dt>cell cooldown</dt>
		<dd>
			{#if acct.cell?.cooldown_until}
				<Countdown until={acct.cell.cooldown_until} variant="cooldown" />
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
