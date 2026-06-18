<script lang="ts">
	import { customerApi } from '$lib/customer-api';
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
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
	<h2>dashboard</h2>
	<div class="bar">
		<span>status {me.user.status}</span>
		<span>plan {billing?.plan ?? me.user.plan ?? '-'}</span>
		<span>credits {billing ? fmtCost(billing.credits_usd) : '-'}</span>
		<span>usage {billing ? fmtCost(billing.usage_usd) : '-'}</span>
		<span>keys {keys.length}</span>
		<span>referrals {referrals?.signups ?? 0}</span>
	</div>

	<h2>account</h2>
	<dl>
		<dt>email</dt><dd>{me.user.email}</dd>
		<dt>status</dt><dd>{me.user.status}</dd>
		<dt>created</dt><dd>{me.user.created_at ? fmtDate(me.user.created_at) : '-'}</dd>
	</dl>
{/if}
