<script lang="ts">
	import { onDestroy } from 'svelte';

	let {
		until,
		tag = 'span',
		variant = 'default'
	}: {
		until: string | number | null;
		tag?: 'span' | 'td';
		variant?: 'default' | 'cooldown';
	} = $props();

	let now = $state(Date.now());
	let timer: ReturnType<typeof setInterval> | null = null;

	let target = $derived(
		until == null ? 0
		: typeof until === 'number' ? until
		: new Date(until).getTime()
	);
	let diff = $derived(target - now);
	let active = $derived(diff > 0);
	let hasValue = $derived(until != null);
	let text = $derived.by(() => {
		if (!active) {
			if (hasValue && variant === 'default') return 'expired';
			return '\u2013';
		}
		const totalSecs = Math.floor(diff / 1000);
		const secs = totalSecs % 60;
		const totalMins = Math.floor(totalSecs / 60);
		const mins = totalMins % 60;
		const hours = Math.floor(totalMins / 60);
		if (hours > 0) return `${hours}h ${String(mins).padStart(2, '0')}m ${String(secs).padStart(2, '0')}s`;
		return `${mins}m ${String(secs).padStart(2, '0')}s`;
	});
	let cls = $derived.by(() => {
		if (!active) {
			if (hasValue && variant === 'default') return 'r';
			return 'muted';
		}
		if (variant === 'cooldown') return 'o';
		const mins = Math.floor(diff / 60000);
		if (mins < 10) return 'r';
		if (mins < 60) return 'o';
		return 'g';
	});

	$effect(() => {
		if (active && !timer) {
			timer = setInterval(() => { now = Date.now(); }, 1000);
		} else if (!active && timer) {
			clearInterval(timer);
			timer = null;
		}
	});

	onDestroy(() => { if (timer) clearInterval(timer); });
</script>

{#if tag === 'td'}
	<td class={cls}>{text}</td>
{:else}
	<span class={cls}>{text}</span>
{/if}
