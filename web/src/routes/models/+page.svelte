<script lang="ts">
	import { base } from '$app/paths';

	const models = [
		{ name: 'GPT-5', id: 'gpt-5', desc: '主力推理与编码模型' },
		{ name: 'GPT-5 Mini', id: 'gpt-5-mini', desc: '轻量、快速、低成本任务' },
		{ name: 'Codex Mini', id: 'codex-mini-latest', desc: '编码和 agent 场景' },
		{ name: 'OpenAI compatible', id: '/v1/chat/completions', desc: '兼容层用于现有客户端迁移' }
	];
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<header class="border-b border-line">
		<div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
			<a href="{base}/" class="font-semibold">CDX</a>
			<a class="rounded-md border border-line px-3 py-2 text-sm hover:border-brand/50" href="{base}/docs">接入文档</a>
		</div>
	</header>

	<section class="mx-auto max-w-6xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">models</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">OpenAI/Codex 模型</h1>
		<p class="mt-4 max-w-2xl text-muted">公开站点只展示当前产品线支持的 OpenAI/Codex 表面；具体可用模型以后端 `/v1/models` 和后台模型配置为准。</p>

		<div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each models as model}
				<div class="rounded-lg border border-line bg-card/60 p-5">
					<div class="font-mono text-xs text-faint">{model.id}</div>
					<h2 class="mt-3 text-lg font-semibold">{model.name}</h2>
					<p class="mt-2 text-sm text-muted">{model.desc}</p>
				</div>
			{/each}
		</div>
	</section>
</main>
