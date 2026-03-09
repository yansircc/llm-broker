<script lang="ts">
	import { page } from '$app/stores';
	import AddAccountFlow from '$lib/components/AddAccountFlow.svelte';
</script>

<AddAccountFlow providerID={$page.params.provider} />
