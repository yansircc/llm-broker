<script lang="ts">
	let { label, cls = '', onclick }: { label: string; cls?: string; onclick: () => void } = $props();

	let confirming = $state(false);
	let timeout: ReturnType<typeof setTimeout> | null = null;

	function startConfirm() {
		confirming = true;
		timeout = setTimeout(() => { confirming = false; }, 3000);
	}

	function doConfirm() {
		confirming = false;
		if (timeout) clearTimeout(timeout);
		onclick();
	}

	function cancel() {
		confirming = false;
		if (timeout) clearTimeout(timeout);
	}
</script>

{#if confirming}
	<span class="confirm-inline">
		确认？
		<button class="link danger" onclick={doConfirm}>确认</button>
		<button class="link" onclick={cancel}>取消</button>
	</span>
{:else}
	<button class="link {cls}" onclick={startConfirm}>{label}</button>
{/if}

<style>
	.confirm-inline {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: var(--muted);
		font-size: 12px;
		white-space: nowrap;
	}

	.confirm-inline button.link {
		padding: 4px 7px;
		font-size: 12px;
	}

	button.danger {
		color: var(--danger);
	}
</style>
