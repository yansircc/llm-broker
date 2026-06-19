<script lang="ts">
	import { BRAND_NAME } from '$lib/brand';

	interface Props {
		href?: string;
		label?: string;
	}

	let { href, label = BRAND_NAME }: Props = $props();
</script>

{#if href}
	<a class="brand" {href} aria-label={label}>
		<span class="brand-mark" aria-hidden="true"></span>
		<span class="brand-word">{label}</span>
	</a>
{:else}
	<span class="brand" aria-label={label}>
		<span class="brand-mark" aria-hidden="true"></span>
		<span class="brand-word">{label}</span>
	</span>
{/if}
