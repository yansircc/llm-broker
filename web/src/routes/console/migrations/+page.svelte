<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem, EgressCellView } from '$lib/admin-types';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import { dotClass, timeAgo } from '$lib/format';
	import { egressLabel, providerLabel, statusLabel, weightModeLabel } from '$lib/admin-i18n';

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
			parts.push(statusLabel('cooling'));
		} else if (cell.status !== 'active') {
			parts.push(statusLabel(cell.status));
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
		return account.cell?.name ?? egressLabel(account.cell_id);
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
			bindResult = `已绑定 ${account.email} -> ${selectedCellID}`;
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
	<div class="page-header">
		<div>
			<div class="eyebrow">迁移</div>
			<h1>迁移工具</h1>
			<p class="lede">把直连账号迁移到可用出口节点，并在迁移后测试账号可用性。</p>
		</div>
		<div class="page-actions">
			<button class="link" onclick={loadAll}>刷新</button>
			<span class="muted mono">{lastRefresh}</span>
		</div>
	</div>

	<div class="metric-grid">
		<MetricCard label="直连队列" value={legacyAccounts().length} sub="直连账号" />
		<MetricCard label="可用节点" value={availableCells().length} sub="活跃目标" />
		<MetricCard label="冷却中" value={cells.filter(cooldownActive).length} sub="冷却节点" />
		<MetricCard label="账号" value={accounts.length} sub="账号池总数" />
	</div>

	<section class="panel workbench">
		<div class="step">
			<label for="migration-account">步骤 1：账号</label>
			<select id="migration-account" bind:value={selectedAccountID} style="max-width:320px;">
				<option value="">选择账号</option>
				{#each legacyAccounts() as account (account.id)}
					<option value={account.id}>{account.email}</option>
				{/each}
			</select>
		</div>

		<div class="step">
			<label for="migration-cell">步骤 2：目标节点</label>
			<select id="migration-cell" bind:value={selectedCellID} style="max-width:320px;">
				<option value="">选择节点</option>
				{#each availableCells() as cell (cell.id)}
					<option value={cell.id}>{optionLabel(cell)}</option>
				{/each}
			</select>
		</div>

		<div class="step actions-row">
			<button class="link" onclick={bindAccount} disabled={binding || !selectedAccountID || !selectedCellID}>
				{binding ? '迁移中...' : '立即迁移'}
			</button>
			<button class="link" onclick={testAccount} disabled={testing || !selectedAccountID}>
				{testing ? '测试中...' : '测试所选账号'}
			</button>
		</div>
	</section>

	{#if selectedAccountID || selectedCellID}
		<div class="bar">
		<dl>
			<dt>账号</dt>
			<dd>
				{#if selectedAccount()}
					<a href="{base}/console/accounts/{selectedAccount()!.id}">{selectedAccount()!.email}</a>
				{:else}
					-
				{/if}
			</dd>

			<dt>状态</dt>
			<dd>
				{#if selectedAccount()}
					<span class={dotClass(selectedAccount()!.status)}>{statusLabel(selectedAccount()!.status)}</span>
				{:else}
					<span class="muted">-</span>
				{/if}
			</dd>

			<dt>当前出口</dt>
			<dd>{currentEgress(selectedAccount())}</dd>

			<dt>最近使用</dt>
			<dd>{selectedAccount() ? timeAgo(selectedAccount()!.last_used_at ?? '') : '-'}</dd>

			<dt>目标节点</dt>
			<dd>
				{#if selectedCell()}
					<a href="{base}/console/cells/{selectedCell()!.id}">{selectedCell()!.name ?? selectedCell()!.id}</a>
				{:else}
					{selectedCellID || '-'}
				{/if}
			</dd>

			<dt>目标地区</dt>
			<dd>{selectedCell() ? region(selectedCell()) : '-'}</dd>

			<dt>目标负载</dt>
			<dd>{selectedCell() ? cellAccounts(selectedCell()).length : '-'}</dd>

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
		</dl>
		</div>
	{/if}

	{#if bindResult}
		<div class="bar"><span class="g">{bindResult}</span></div>
	{/if}
	{#if testResult}
		<div class="bar">
			{#if testResult.ok}
				<span class="g">&#10003; 正常</span> {#if testResult.latency_ms}( {(testResult.latency_ms / 1000).toFixed(1)}s ){/if}
			{:else}
				<span class="r">&#10007; 失败</span> {testResult.error}
			{/if}
		</div>
	{/if}
	{#if actionError}
		<p class="error-msg">{actionError}</p>
	{/if}

	<details class="ops-details">
		<summary>直连队列（{legacyAccounts().length}）</summary>
		{#if legacyAccounts().length === 0}
			<p class="muted">暂无直连账号</p>
		{:else}
			<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>邮箱</th>
						<th>上游</th>
						<th>状态</th>
						<th>权重</th>
						<th>最近使用</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each legacyAccounts() as account (account.id)}
						<tr>
							<td><a href="{base}/console/accounts/{account.id}">{account.email}</a></td>
							<td>{providerLabel(account.provider)}</td>
							<td><span class={dotClass(account.status)}>{statusLabel(account.status)}</span></td>
							<td>{account.weight}{#if account.weight_mode === 'auto'} <span class="muted">({weightModeLabel(account.weight_mode)})</span>{/if}</td>
							<td>{timeAgo(account.last_used_at ?? '')}</td>
							<td><a href="{base}/console/accounts/{account.id}">打开</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
			</div>
		{/if}
	</details>

	<details class="ops-details">
		<summary>节点库存（{cells.length}）</summary>
		{#if cells.length === 0}
			<p class="muted">暂无节点</p>
		{:else}
			<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>节点</th>
						<th>地区</th>
						<th class="num">负载</th>
						<th>状态</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each cells as cell (cell.id)}
						<tr>
							<td><a href="{base}/console/cells/{cell.id}">{cell.name || cell.id}</a></td>
							<td>{region(cell)}</td>
							<td class="num">{cellAccounts(cell).length}</td>
							<td>
								{#if cooldownActive(cell)}
									<span class="o">冷却中</span>
								{:else}
									<span class={cell.status === 'active' ? 'g' : cell.status === 'error' ? 'r' : 'muted'}>{statusLabel(cell.status)}</span>
								{/if}
							</td>
							<td><a href="{base}/console/cells/{cell.id}">打开</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
			</div>
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
