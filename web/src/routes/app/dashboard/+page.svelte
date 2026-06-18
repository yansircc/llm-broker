<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';
	import type { BillingSummary, CustomerApiKey, CustomerMe, ReferralSummary } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';

	let me = $state<CustomerMe | null>(null);
	let keys = $state<CustomerApiKey[]>([]);
	let billing = $state<BillingSummary | null>(null);
	let referrals = $state<ReferralSummary | null>(null);
	let error = $state('');
	let lastRefresh = $state('');

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		try {
			const [meResult, keyResult, billingResult, referralResult] = await Promise.all([
				customerApi<CustomerMe>('/me'),
				customerApi<CustomerApiKey[]>('/keys').catch(() => []),
				customerApi<BillingSummary>('/billing/summary').catch(() => null),
				customerApi<ReferralSummary>('/referrals').catch(() => null)
			]);
			me = meResult;
			keys = keyResult;
			billing = billingResult;
			referrals = referralResult;
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message || 'failed to load dashboard';
		}
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else if !me}
	<p class="loading">loading customer dashboard...</p>
{:else}
	<div class="page-header">
		<div>
			<div class="eyebrow">customer console</div>
			<h1>Dashboard</h1>
			<p class="lede">Balance, key inventory, usage, and referral status for the current CDX account.</p>
		</div>
		<div class="page-actions">
			<button class="link" onclick={loadAll}>refresh</button>
			<span class="muted mono">{lastRefresh}</span>
		</div>
	</div>

	<div class="metric-grid">
		<MetricCard label="balance" value={billing ? fmtCost(billing.balance_usd) : '-'} sub="available prepaid credit" />
		<MetricCard label="credits" value={billing ? fmtCost(billing.credits_usd) : '-'} sub="total credited amount" />
		<MetricCard label="usage" value={billing ? fmtCost(billing.usage_usd) : '-'} sub="token debits recorded" />
		<MetricCard label="api keys" value={keys.length} sub={`${referrals?.signups ?? 0} referral signups`} />
	</div>

	<div class="split-grid">
		<section class="panel">
			<div class="section-header flush">
				<h2>Account</h2>
				<StatusBadge status={me.user.status} />
			</div>
			<dl>
				<dt>email</dt><dd>{me.user.email}</dd>
				<dt>plan</dt><dd>{billing?.plan ?? me.user.plan ?? '-'}</dd>
				<dt>created</dt><dd>{me.user.created_at ? fmtDate(me.user.created_at) : '-'}</dd>
				<dt>billing status</dt><dd>{billing?.status ?? '-'}</dd>
			</dl>
		</section>

		<TerminalPanel
			title="relay contract"
			lines={[
				'surface: openai/responses',
				'admission: balance > 0',
				'pricing: token metering',
				'currency: USD display / RMB recharge'
			]}
		/>
	</div>
{/if}
