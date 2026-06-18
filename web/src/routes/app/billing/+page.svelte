<script lang="ts">
	import { customerApi } from '$lib/customer-api';
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

<span class="refresh"><button class="link" onclick={loadSummary}>[refresh]</button></span>
<h2>billing</h2>

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading billing...</p>
{/if}

{#if summary}
	<div class="bar">
		<span>plan {summary.plan}</span>
		<span>status {summary.status}</span>
		<span>balance {fmtCost(summary.balance_usd)}</span>
		<span>credits {fmtCost(summary.credits_usd)}</span>
		<span>usage {fmtCost(summary.usage_usd)}</span>
	</div>
	<dl>
		<dt>period start</dt><dd>{summary.period_start ? fmtDate(summary.period_start) : '-'}</dd>
		<dt>period end</dt><dd>{summary.period_end ? fmtDate(summary.period_end) : '-'}</dd>
	</dl>
{/if}

<h2>add credit</h2>
<div class="bar">
	<input type="text" bind:value={amount} style="width:80px;max-width:80px;margin-right:6px;">
	<button class="link" onclick={createPayment} disabled={creating}>{creating ? '[creating...]' : '[create payment]'}</button>
</div>

{#if order}
	<h2>payment order</h2>
	<dl>
		<dt>id</dt><dd>{order.id}</dd>
		<dt>status</dt><dd>{order.status}</dd>
		<dt>amount</dt><dd>{fmtCost(order.amount_usd)}</dd>
		<dt>created</dt><dd>{fmtDate(order.created_at)}</dd>
		<dt>checkout</dt><dd>{#if order.checkout_url}<a href={order.checkout_url}>open</a>{:else}-{/if}</dd>
	</dl>
{/if}
