<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';
	import type { ReferralSummary } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';

	let referral = $state<ReferralSummary | null>(null);
	let error = $state('');
	let loading = $state(false);
	let copied = $state(false);

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

	async function copyReferralURL() {
		if (!referral?.url) return;
		await navigator.clipboard.writeText(referral.url);
		copied = true;
		setTimeout(() => { copied = false; }, 2000);
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">growth</div>
		<h1>Referrals</h1>
		<p class="lede">Invitee credit is granted on registration. Inviter credit is granted after the invitee pays.</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadReferral}>refresh</button>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading referrals...</p>
{:else if referral}
	<div class="metric-grid">
		<MetricCard label="code" value={referral.code} sub="shareable invite code" />
		<MetricCard label="signups" value={referral.signups} sub="registered invitees" />
		<MetricCard label="credits" value={fmtCost(referral.credits_usd)} sub="paid inviter rewards" />
		<MetricCard label="status" value="active" sub="referral program" />
	</div>

	<div class="split-grid">
		<section class="panel">
			<div class="section-header flush">
				<h2>Referral Link</h2>
				{#if referral.url}<button class="link" onclick={copyReferralURL}>{copied ? 'copied' : 'copy'}</button>{/if}
			</div>
			<div class="copy-value mono">{referral.url || '-'}</div>
		</section>

		<TerminalPanel title="reward rule" lines={['invitee: credit on registration', 'inviter: credit after paid order', `code: ${referral.code}`]} />
	</div>
{:else}
	<p class="muted">no referral data</p>
{/if}
