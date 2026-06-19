<script lang="ts">
	let apiKey = $state('');
	let model = $state('codex');
	let result = $state('');
	const models = [
		{ value: 'codex', label: 'Codex（当前可用）' },
		{ value: 'claude-sonnet-4.5', label: 'Claude Sonnet 4.5（预留）' },
		{ value: 'claude-opus-4.1', label: 'Claude Opus 4.1（预留）' },
		{ value: 'gpt-5', label: 'GPT-5（预留）' }
	];

	function testKey() {
		result = `当前版本尚未接入 Key 真伪测试后端；已保留 ${models.find((item) => item.value === model)?.label ?? model} 的视觉测试入口。`;
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
			<div class="font-mono text-xs uppercase tracking-wider text-brand">key test</div>
			<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">Key 测试</h1>
			<p class="mt-2 text-sm text-muted">一键检测你的 API Key 是否正常工作，验证模型连通性和响应速度。当前测试后端未接入，页面先按参考视觉保留。</p>
		</div>
	</div>

	<section class="max-w-3xl rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">测试你的 API Key</h2>
		<p class="mt-2 text-sm text-faint">后端接入后会发送一条简短测试请求；当前只展示 Codex 当前可用、Claude/GPT 预留的模型选择形态。</p>
	<div class="mt-5 grid gap-3 sm:grid-cols-[1fr_220px_auto]">
		<input class="h-11 rounded-md border border-line bg-black/30 px-3 font-mono text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="sk-xxxxxxxxxxxxxxxx" bind:value={apiKey}>
		<select class="h-11 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" bind:value={model}>
			{#each models as item}
				<option value={item.value}>{item.label}</option>
			{/each}
		</select>
		<button class="h-11 rounded-md bg-brand px-5 text-sm font-semibold text-black" type="button" onclick={testKey}>开始测试</button>
	</div>
	{#if result}
		<p class="mt-4 rounded-md border border-amber-400/30 bg-amber-400/10 px-4 py-3 text-sm text-amber-200">{result}</p>
	{/if}
</section>

<section class="mt-6 grid gap-4 md:grid-cols-3">
	{#each [['测试会发送一条简短消息', '只用于验证 Key、模型和网关链路，不会执行长上下文任务。'], ['只能测试你自己创建的 Key', '用户侧测试入口应绑定当前账户，避免跨账户探测。'], ['后端接入后展示结果', '展示状态、首 token、总耗时和错误原因；当前不伪造测试数据。']] as item}
		<div class="rounded-lg border border-line bg-card/60 p-5">
			<h3 class="text-base font-semibold">{item[0]}</h3>
			<p class="mt-2 text-sm text-faint">{item[1]}</p>
		</div>
	{/each}
</section>
