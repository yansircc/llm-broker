<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import type { BillingSummary, PaymentOrder } from '$lib/customer-types';
	import { fmtCost, fmtDate } from '$lib/format';

	let summary = $state<BillingSummary | null>(null);
	let order = $state<PaymentOrder | null>(null);
	let amount = $state('20');
	let error = $state('');
	let loading = $state(false);
	let creating = $state(false);

	$effect(() => {
		loadSummary();
	});

	async function loadSummary() {
		loading = true;
		error = '';
		try {
			summary = await customerApi<BillingSummary>('/billing/summary');
		} catch (e: any) {
			error = e.message || 'failed to load billing';
		} finally {
			loading = false;
		}
	}

	async function createPayment() {
		const amountUsd = Number.parseFloat(amount);
		if (!Number.isFinite(amountUsd) || amountUsd <= 0) {
			error = 'amount must be greater than zero';
			return;
		}
		creating = true;
		error = '';
		try {
			order = await customerApi<PaymentOrder>('/payments/create', {
				method: 'POST',
				body: JSON.stringify({ amount_usd: amountUsd })
			});
		} catch (e: any) {
			error = e.message || 'failed to create payment';
		} finally {
			creating = false;
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">ledger</div>
		<h1>Billing</h1>
		<p class="lede">Prepaid credit balance, token debits, and 7pay recharge order creation.</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadSummary}>refresh</button>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading billing...</p>
{/if}

{#if summary}
	<div class="metric-grid">
		<MetricCard label="balance" value={fmtCost(summary.balance_usd)} sub="current spendable credits" />
		<MetricCard label="credits" value={fmtCost(summary.credits_usd)} sub="all successful credits" />
		<MetricCard label="usage" value={fmtCost(summary.usage_usd)} sub="recorded token debits" />
		<MetricCard label="plan" value={summary.plan} sub={summary.status} />
	</div>
{/if}

<div class="split-grid">
	<section class="panel">
		<div class="section-header flush">
			<div>
				<h2>Add Credit</h2>
				<p class="hint">Amount is entered as USD credit. Payment is collected in RMB by the configured ratio.</p>
			</div>
		</div>
		<div class="form-row">
			<label for="amount">amount</label>
			<input id="amount" type="text" bind:value={amount}>
			<button class="primary-btn" onclick={createPayment} disabled={creating}>{creating ? 'Creating...' : 'Create payment'}</button>
		</div>
	</section>

	{#if summary}
		<section class="panel">
			<div class="section-header flush">
				<h2>Billing State</h2>
				<StatusBadge status={summary.status} />
			</div>
			<dl>
				<dt>period start</dt><dd>{summary.period_start ? fmtDate(summary.period_start) : '-'}</dd>
				<dt>period end</dt><dd>{summary.period_end ? fmtDate(summary.period_end) : '-'}</dd>
				<dt>plan</dt><dd>{summary.plan}</dd>
				<dt>status</dt><dd>{summary.status}</dd>
			</dl>
		</section>
	{/if}
</div>

{#if order}
	<section class="panel">
		<div class="section-header flush">
			<h2>Payment Order</h2>
			<StatusBadge status={order.status} />
		</div>
		<dl>
			<dt>id</dt><dd>{order.id}</dd>
			<dt>amount</dt><dd>{fmtCost(order.amount_usd)}</dd>
			<dt>created</dt><dd>{fmtDate(order.created_at)}</dd>
			<dt>checkout</dt><dd>{#if order.checkout_url}<a class="secondary-btn fit" href={order.checkout_url}>Open checkout</a>{:else}-{/if}</dd>
		</dl>
	</section>
{/if}
