<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { ReferralSummary } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';

	let referral = $state<ReferralSummary | null>(null);
	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		loadReferral();
	});

	async function loadReferral() {
		loading = true;
		error = '';
		try {
			referral = await customerApi<ReferralSummary>('/referrals');
		} catch (e: any) {
			error = e.message || 'failed to load referrals';
		} finally {
			loading = false;
		}
	}
</script>

<span class="refresh"><button class="link" onclick={loadReferral}>[refresh]</button></span>
<h2>referrals</h2>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading referrals...</p>
{:else if referral}
	<div class="bar">
		<span>code {referral.code}</span>
		<span>signups {referral.signups}</span>
		<span>credits {fmtCost(referral.credits_usd)}</span>
	</div>
	<dl>
		<dt>url</dt>
		<dd>{#if referral.url}<span style="user-select:all;">{referral.url}</span>{:else}-{/if}</dd>
	</dl>
{:else}
	<p class="muted">no referral data</p>
{/if}
