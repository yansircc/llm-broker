<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem, EgressCellView } from '$lib/admin-types';
	import { dotClass, timeAgo } from '$lib/format';

	let accounts = $state<AccountListItem[]>([]);
	let cells = $state<EgressCellView[]>([]);
	let error = $state('');
	let lastRefresh = $state('');
	let selectedAccountID = $state('');
	let selectedCellID = $state('');
	let binding = $state(false);
	let testing = $state(false);
	let actionError = $state('');
	let bindResult = $state('');
	let testResult = $state<{ ok: boolean; latency_ms: number; error?: string } | null>(null);

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		actionError = '';
		try {
			const [accountList, cellList] = await Promise.all([
				api<AccountListItem[]>('/accounts'),
				api<EgressCellView[]>('/egress/cells')
			]);
			accounts = accountList;
			cells = cellList;
			const legacyAccounts = accountList.filter((acct) => !acct.cell_id);
			if (!legacyAccounts.some((acct) => acct.id === selectedAccountID)) {
				selectedAccountID = legacyAccounts[0]?.id ?? '';
			}
			if (!cellList.some((cell) => cell.id === selectedCellID && cellAvailable(cell))) {
				selectedCellID = cellList.find(cellAvailable)?.id ?? '';
			}
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	function region(cell: EgressCellView): string {
		const labels = cell.labels ?? {};
		return [labels.country, labels.city].filter(Boolean).join(' / ') || labels.site || '-';
	}

	function cooldownActive(cell: EgressCellView): boolean {
		return !!cell.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now();
	}

	function cellSelectable(cell: EgressCellView): boolean {
		return cell.status === 'active' && !cooldownActive(cell) && !!cell.proxy?.host && !!cell.proxy?.port;
	}

	function cellAccounts(cell: EgressCellView | null | undefined) {
		return cell?.accounts ?? [];
	}

	function cellAvailable(cell: EgressCellView): boolean {
		if (cell.proxy?.type === 'socks5') return cellSelectable(cell);
		const selectedAcct = accounts.find((a) => a.id === selectedAccountID);
		const providerID = selectedAcct?.provider ?? '';
		const sameProvider = cellAccounts(cell).filter((a) => a.provider === providerID);
		return cellSelectable(cell) && sameProvider.length === 0;
	}

	function availableCells(): EgressCellView[] {
		return cells.filter(cellAvailable);
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

	function selectedAccount(): AccountListItem | undefined {
		return accounts.find((acct) => acct.id === selectedAccountID);
	}

	function selectedCell(): EgressCellView | undefined {
		return cells.find((cell) => cell.id === selectedCellID);
	}

	function legacyAccounts(): AccountListItem[] {
		return accounts.filter((acct) => !acct.cell_id);
	}

	function currentEgress(account: AccountListItem | undefined): string {
		if (!account) return '-';
		return account.cell?.name ?? account.cell_id ?? 'legacy direct';
	}

	async function bindAccount() {
		const account = selectedAccount();
		if (!account || !selectedCellID) return;
		binding = true;
		actionError = '';
		bindResult = '';
		testResult = null;
		try {
			await api(`/accounts/${account.id}/cell`, {
				method: 'POST',
				body: JSON.stringify({ cell_id: selectedCellID })
			});
			bindResult = `bound ${account.email} -> ${selectedCellID}`;
			await loadAll();
		} catch (e: any) {
			actionError = e.message;
		} finally {
			binding = false;
		}
	}

	async function testAccount() {
		const account = selectedAccount();
		if (!account) return;
		testing = true;
		actionError = '';
		testResult = null;
		try {
			testResult = await api<{ ok: boolean; latency_ms: number; error?: string }>(`/accounts/${account.id}/test`, {
				method: 'POST'
			});
		} catch (e: any) {
			actionError = e.message;
		} finally {
			testing = false;
		}
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else}
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
	<div class="bar">
		<span>legacy queue {legacyAccounts().length}</span>
		<span>available cells {availableCells().length}</span>
		<span>cooling {cells.filter(cooldownActive).length}</span>
	</div>

	<h2>migration workbench <span class="muted">temporary</span></h2>
	<p class="hint">move one legacy direct account onto one empty active cell, then optionally probe it.</p>

	<div class="bar workbench">
		<div class="step">
			<label for="migration-account">step 1: account</label>
			<select id="migration-account" bind:value={selectedAccountID} style="max-width:320px;">
				<option value="">select account</option>
				{#each legacyAccounts() as account (account.id)}
					<option value={account.id}>{account.email}</option>
				{/each}
			</select>
		</div>

		<div class="step">
			<label for="migration-cell">step 2: target cell</label>
			<select id="migration-cell" bind:value={selectedCellID} style="max-width:320px;">
				<option value="">select cell</option>
				{#each availableCells() as cell (cell.id)}
					<option value={cell.id}>{optionLabel(cell)}</option>
				{/each}
			</select>
		</div>

		<div class="step actions-row">
			<button class="link" onclick={bindAccount} disabled={binding || !selectedAccountID || !selectedCellID}>
				{binding ? '[migrating...]' : '[migrate now]'}
			</button>
			<button class="link" onclick={testAccount} disabled={testing || !selectedAccountID}>
				{testing ? '[testing...]' : '[test selected account]'}
			</button>
		</div>
	</div>

	{#if selectedAccountID || selectedCellID}
		<div class="bar">
		<dl>
			<dt>account</dt>
			<dd>
				{#if selectedAccount()}
					<a href="{base}/accounts/{selectedAccount()!.id}">{selectedAccount()!.email}</a>
				{:else}
					-
				{/if}
			</dd>

			<dt>status</dt>
			<dd>
				{#if selectedAccount()}
					<span class={dotClass(selectedAccount()!.status)}>{selectedAccount()!.status}</span>
				{:else}
					<span class="muted">-</span>
				{/if}
			</dd>

			<dt>current egress</dt>
			<dd>{currentEgress(selectedAccount())}</dd>

			<dt>last used</dt>
			<dd>{selectedAccount() ? timeAgo(selectedAccount()!.last_used_at ?? '') : '-'}</dd>

			<dt>target cell</dt>
			<dd>
				{#if selectedCell()}
					<a href="{base}/cells/{selectedCell()!.id}">{selectedCell()!.name ?? selectedCell()!.id}</a>
				{:else}
					{selectedCellID || '-'}
				{/if}
			</dd>

			<dt>target region</dt>
			<dd>{selectedCell() ? region(selectedCell()) : '-'}</dd>

			<dt>target load</dt>
			<dd>{selectedCell() ? cellAccounts(selectedCell()).length : '-'}</dd>

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
		</dl>
		</div>
	{/if}

	{#if bindResult}
		<div class="bar"><span class="g">{bindResult}</span></div>
	{/if}
	{#if testResult}
		<div class="bar">
			{#if testResult.ok}
				<span class="g">&#10003; ok</span> {#if testResult.latency_ms}( {(testResult.latency_ms / 1000).toFixed(1)}s ){/if}
			{:else}
				<span class="r">&#10007; failed</span> {testResult.error}
			{/if}
		</div>
	{/if}
	{#if actionError}
		<p class="error-msg">{actionError}</p>
	{/if}

	<details class="ops-details">
		<summary>legacy queue ({legacyAccounts().length})</summary>
		{#if legacyAccounts().length === 0}
			<p class="muted">no legacy direct accounts</p>
		{:else}
			<table>
				<thead>
					<tr>
						<th>email</th>
						<th>provider</th>
						<th>status</th>
						<th>weight</th>
						<th>last used</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each legacyAccounts() as account (account.id)}
						<tr>
							<td><a href="{base}/accounts/{account.id}">{account.email}</a></td>
							<td>{account.provider}</td>
							<td><span class={dotClass(account.status)}>{account.status}</span></td>
							<td>{account.weight}{#if account.weight_mode === 'auto'} <span class="muted">(auto)</span>{/if}</td>
							<td>{timeAgo(account.last_used_at ?? '')}</td>
							<td><a href="{base}/accounts/{account.id}">open</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</details>

	<details class="ops-details">
		<summary>cell inventory ({cells.length})</summary>
		{#if cells.length === 0}
			<p class="muted">no cells</p>
		{:else}
			<table>
				<thead>
					<tr>
						<th>cell</th>
						<th>region</th>
						<th class="num">load</th>
						<th>status</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each cells as cell (cell.id)}
						<tr>
							<td><a href="{base}/cells/{cell.id}">{cell.name || cell.id}</a></td>
							<td>{region(cell)}</td>
							<td class="num">{cellAccounts(cell).length}</td>
							<td>
								{#if cooldownActive(cell)}
									<span class="o">cooling</span>
								{:else}
									<span class={cell.status === 'active' ? 'g' : cell.status === 'error' ? 'r' : 'muted'}>{cell.status}</span>
								{/if}
							</td>
							<td><a href="{base}/cells/{cell.id}">open</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</details>
{/if}

<style>
	.workbench {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.step label {
		display: block;
		margin-bottom: 2px;
	}
	.actions-row {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		align-items: center;
	}
	.ops-details {
		margin-top: 14px;
	}
	.ops-details summary {
		cursor: pointer;
		font-weight: bold;
		margin-bottom: 8px;
	}
</style>
