<script lang="ts">
	import { onMount } from 'svelte';

	interface ModelPrice {
		model: string;
		input_usd_per_million: number;
		output_usd_per_million: number;
	}

	let models = $state<ModelPrice[]>([]);

	onMount(async () => {
		const res = await fetch('/api/public/model-prices', { headers: { Accept: 'application/json' } }).catch(() => null);
		if (res?.ok) {
			models = await res.json();
		}
	});
</script>

<section class="mx-auto max-w-6xl px-5 py-16">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">models</div>
	<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">OpenAI/Codex 模型</h1>
	<p class="mt-4 max-w-2xl text-muted">公开站点只展示当前产品线支持的 OpenAI/Codex Responses 表面；具体可用模型以后端 `/v1/models` 和后台模型配置为准。</p>

	<div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		{#each models as model}
			<div class="rounded-lg border border-line bg-card/60 p-5">
				<div class="font-mono text-xs text-faint">Responses</div>
				<h2 class="mt-3 text-lg font-semibold">{model.model}</h2>
				<p class="mt-2 text-sm text-muted">输入 ${model.input_usd_per_million}/M token，输出 ${model.output_usd_per_million}/M token。</p>
			</div>
		{:else}
			<div class="rounded-lg border border-line bg-card/60 p-5 text-sm text-muted">正在加载模型列表...</div>
		{/each}
	</div>
</section>
