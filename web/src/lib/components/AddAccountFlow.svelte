<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { EgressCellView } from '$lib/admin-types';
	import { addAccountPath, type ProviderOption } from '$lib/providers';
	import { egressLabel, statusLabel } from '$lib/admin-i18n';

	interface Props {
		providerID: string;
	}

	let { providerID }: Props = $props();

	let providers = $state<ProviderOption[]>([]);
	let provider = $state<ProviderOption | null>(null);
	let cells = $state<EgressCellView[]>([]);
	let loadingProvider = $state(true);
	let providerError = $state('');
	let generating = $state(false);
	let exchanging = $state(false);
	const legacyDirectValue = '__legacy_direct__';
	let selectedRoute = $state('');
	let sessionId = $state('');
	let authUrl = $state('');
	let callbackInput = $state('');
	let genError = $state('');
	let exchangeError = $state('');
	let result = $state<{ id: string; email: string; status: string } | null>(null);

	$effect(() => {
		void loadProvider(providerID);
	});

	async function loadProvider(id: string) {
		loadingProvider = true;
		providerError = '';
		provider = null;
		cells = [];
		reset();
		try {
			const [providerOptions, cellList] = await Promise.all([
				api<ProviderOption[]>('/providers'),
				api<EgressCellView[]>('/egress/cells')
			]);
			providers = providerOptions;
			provider = providers.find((option) => option.id === id) ?? null;
			cells = cellList;
			const available = availableCells(cellList);
			if (selectedRoute !== legacyDirectValue && !available.some((cell) => cell.id === selectedRoute)) {
				selectedRoute = available[0]?.id ?? legacyDirectValue;
			}
			if (!provider) {
				providerError = `未知上游类型：${id}`;
			}
		} catch (e: any) {
			providerError = e.message;
		} finally {
			loadingProvider = false;
		}
	}

	async function generateAuthUrl() {
		if (!provider || !hasRouteSelection()) return;
		generating = true;
		genError = '';
		try {
			const data = await api<{ session_id: string; auth_url: string }>('/accounts/generate-auth-url', {
				method: 'POST',
				body: JSON.stringify({
					provider: provider.id,
					cell_id: effectiveCellID()
				})
			});
			sessionId = data.session_id;
			authUrl = data.auth_url;
		} catch (e: any) {
			genError = e.message;
		} finally {
			generating = false;
		}
	}

	async function exchangeCode() {
		if (exchanging) return;
		if (!provider || !callbackInput.trim()) return;
		exchanging = true;
		exchangeError = '';
		try {
			const data = await api<{ id: string; email: string; status: string }>('/accounts/exchange-code', {
				method: 'POST',
				timeout: 120000,
				body: JSON.stringify({
					provider: provider.id,
					session_id: sessionId,
					callback_url: callbackInput.trim()
				})
			});
			result = data;
		} catch (e: any) {
			exchangeError = e.message;
		} finally {
			exchanging = false;
		}
	}

	function reset() {
		sessionId = '';
		authUrl = '';
		callbackInput = '';
		genError = '';
		exchangeError = '';
		result = null;
	}

	function startOver() {
		reset();
		void loadProvider(providerID);
	}

	function cooldownActive(cell: EgressCellView | null | undefined): boolean {
		return !!cell?.cooldown_until && new Date(cell.cooldown_until).getTime() > Date.now();
	}

	function cellSelectable(cell: EgressCellView): boolean {
		return cell.status === 'active' && !cooldownActive(cell) && !!cell.proxy?.host && !!cell.proxy?.port;
	}

	function cellAccounts(cell: EgressCellView | null | undefined) {
		return cell?.accounts ?? [];
	}

	function cellAvailable(cell: EgressCellView): boolean {
		if (cell.proxy?.type === 'socks5') return cellSelectable(cell);
		const sameProvider = cellAccounts(cell).filter((a) => a.provider === provider?.id);
		return cellSelectable(cell) && sameProvider.length === 0;
	}

	function availableCells(source: EgressCellView[] = cells): EgressCellView[] {
		return source.filter(cellAvailable);
	}

	function hasRouteSelection(): boolean {
		return selectedRoute !== '';
	}

	function effectiveCellID(): string {
		return selectedRoute === legacyDirectValue ? '' : selectedRoute;
	}

	function selectedCell(): EgressCellView | undefined {
		const cellID = effectiveCellID();
		return cellID ? cells.find((cell) => cell.id === cellID) : undefined;
	}

	function region(cell: EgressCellView | null | undefined): string {
		const labels = cell?.labels ?? {};
		return [labels.country, labels.city].filter(Boolean).join(' / ') || labels.site || '-';
	}

	function optionLabel(cell: EgressCellView): string {
		const parts = [cell.name || cell.id];
		const cellRegion = region(cell);
		if (cellRegion !== '-') parts.push(cellRegion);
		return parts.join(' / ');
	}

	function selectedRouteLabel(): string {
		if (selectedRoute === legacyDirectValue) return egressLabel('');
		return selectedCell()?.name ?? effectiveCellID();
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">上游接入</div>
		<h1>新增账号</h1>
		<p class="lede">生成上游授权链接，选择出口线路，并把 callback 换成 Broker 账号。</p>
	</div>
</div>

{#if loadingProvider}
	<p class="loading">正在加载上游类型...</p>
{:else if providerError}
	<p class="error-msg">{providerError}</p>
	{#if providers.length > 0}
		<div class="bar">
			可用上游：
			{#each providers as option, i (option.id)}
				{#if i > 0}<span class="muted">|</span>{/if}
				<a href={addAccountPath(base, option.id)}>{option.label}</a>
			{/each}
		</div>
	{/if}
{:else if provider}
	<div class="bar" style="margin-bottom:12px">
		上游：<b>{provider.label}</b>
	</div>

	<h2>出口线路 {#if hasRouteSelection()}<span class="g">&#10003;</span>{/if}</h2>
	<div class="bar">
		<select bind:value={selectedRoute} disabled={generating || exchanging || !!sessionId || !!result}>
			<option value="">选择线路</option>
			<option value={legacyDirectValue}>直连</option>
			{#each availableCells() as cell (cell.id)}
				<option value={cell.id}>{optionLabel(cell)}</option>
			{/each}
		</select>
		<button class="link" onclick={() => void loadProvider(providerID)} disabled={loadingProvider || generating || exchanging} style="margin-left:8px">
			刷新节点
		</button>
		{#if sessionId || result}
			<button class="link" onclick={startOver} disabled={generating || exchanging} style="margin-left:8px">
				重新开始
			</button>
		{/if}
		{#if selectedRoute === legacyDirectValue}
			<br><br>
			线路：<b>直连</b><br>
			<span class="muted">使用 Broker 直连出口，不把该账号绑定到节点。</span>
		{:else if selectedCell()}
			<br><br>
			节点：<b>{selectedCell()?.name}</b><br>
			地区：<b>{region(selectedCell())}</b><br>
			代理：<b>{selectedCell()?.proxy?.type}://{selectedCell()?.proxy?.host}:{selectedCell()?.proxy?.port}</b>
		{:else if availableCells().length === 0}
			<br><br>
			<span class="error-msg">暂无可用节点</span>
		{/if}
	</div>

	<h2>授权 {#if sessionId}<span class="g">&#10003;</span>{/if}</h2>
	{#if !sessionId}
		<p class="hint">生成 OAuth URL，在浏览器打开后登录并授权。</p>
		<button class="link" onclick={generateAuthUrl} disabled={generating || !hasRouteSelection()}>
			{generating ? '生成中...' : '生成授权链接'}
		</button>
		{#if genError}
			<p class="error-msg">{genError}</p>
		{/if}
	{:else}
		<div class="bar">
			session：<b>{sessionId}</b> <span class="muted">（10 分钟后过期）</span>
			<br><br>
			授权 URL：<br>
			<a href={authUrl} target="_blank" class="auth-url">{authUrl}</a>
			<br><br>
			<span class="muted">打开这个 URL，登录并授权，然后把 callback 粘贴到下方。</span>
		</div>
	{/if}

	{#if sessionId}
		<h2>交换 code {#if result}<span class="g">&#10003;</span>{/if}</h2>
		{#if !result}
			<label for="callback-input">callback URL 或 code</label>
			<input id="callback-input" type="text" bind:value={callbackInput} placeholder={provider.callback_placeholder}>
			<p class="hint">{provider.callback_hint}</p>
			{#if exchangeError}
				<p class="error-msg">{exchangeError}</p>
			{/if}
			<button class="link" onclick={exchangeCode} disabled={exchanging || !callbackInput.trim()}>
				{exchanging ? '交换中...' : '交换 code'}
			</button>
		{:else}
			<div class="bar">
				<span class="g">&#10003; 账号已创建</span>
				<br><br>
				邮箱：<b>{result.email}</b><br>
				状态：<b class="g">{statusLabel(result.status)}</b><br>
				线路：<b>{selectedRouteLabel()}</b><br>
				<br>
				<a href="{base}/console/accounts/{result.id}">查看账号 &rarr;</a>
			</div>
		{/if}
	{/if}
{/if}

<p class="sub"><a href="{base}/console/dashboard">返回控制台</a></p>
