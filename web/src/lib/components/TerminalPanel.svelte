<script lang="ts">
	interface Props {
		title?: string;
		lines: string[];
	}

	let { title = 'terminal', lines }: Props = $props();
</script>

<div class="terminal">
	<div class="terminal-head">
		<span class="terminal-dot"></span>
		<span class="terminal-dot"></span>
		<span class="terminal-dot"></span>
		<span style="margin-left:8px;">{title}</span>
	</div>
	<div class="terminal-body">
		{#each lines as line}
			<div>{line}</div>
		{/each}
	</div>
</div>
