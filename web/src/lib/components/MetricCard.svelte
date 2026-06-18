<script lang="ts">
	interface Props {
		label: string;
		value: string | number;
		sub?: string;
	}

	let { label, value, sub = '' }: Props = $props();
</script>

<div class="metric-card">
	<div class="metric-label">{label}</div>
	<div class="metric-value">{value}</div>
	{#if sub}<div class="metric-sub">{sub}</div>{/if}
</div>
