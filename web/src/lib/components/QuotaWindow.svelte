<script lang="ts">
	import type { UtilWindow } from '$lib/admin-types';
	import { remainClass, remainTime, timeAgo } from '$lib/format';

	let { window, compact = false }: { window: UtilWindow; compact?: boolean } = $props();
	let remaining = $derived(
		window.pct == null || window.status === 'stale' ? null : Math.max(0, Math.min(100, 100 - window.pct))
	);
	let observed = $derived(window.observed_at ? timeAgo(new Date(window.observed_at * 1000).toISOString()) : '');
	let hint = $derived([
		window.reason,
		window.raw_pct == null ? '' : `Used ${window.raw_pct.toFixed(0)}%`,
		observed ? `Updated ${observed}` : ''
	].filter(Boolean).join(' · '));
</script>

<span class="quota-window" title={hint}>
	<span class="reading">
		{#if remaining == null}
			<span class="muted">{window.status === 'stale' ? 'pending update' : 'unknown'}</span>
		{:else}
			<span class={remainClass(remaining)}>{remaining}%</span>
		{/if}
		{#if window.sub_pct != null}
			/<span class={remainClass(100 - window.sub_pct)}>{Math.max(0, 100 - window.sub_pct)}%</span>
		{/if}
		{#if window.reset && window.status !== 'stale'}
			<span class="muted">{remainTime(window.reset)}</span>
		{/if}
	</span>
	{#if window.status === 'rejected'}
		<span class="r">{window.scope === 'billing' ? 'credits unavailable' : window.scope === 'model' ? 'model limited' : 'limited'}</span>
	{/if}
	{#if !compact}
		{#if window.reason}<span class="muted">{window.reason}</span>{/if}
		{#if window.raw_pct != null && window.raw_pct > 100}
			<span class="muted">(used {window.raw_pct.toFixed(0)}%)</span>
		{/if}
		{#if observed}<span class="muted">updated {observed}</span>{/if}
	{/if}
</span>

<style>
	.quota-window { display: inline-flex; flex-direction: column; gap: 2px; text-align: left; }
	.reading { white-space: nowrap; }
</style>
