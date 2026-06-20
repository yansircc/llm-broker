<script lang="ts">
	let apiKey = $state('');
	let model = $state('claude-sonnet-4-6');
	let result = $state('');
	const models = [
		{ value: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6' },
		{ value: 'claude-opus-4-8', label: 'Claude Opus 4.8' },
		{ value: 'gpt-5.4', label: 'GPT-5.4' },
		{ value: 'codex', label: 'Codex' }
	];

	function testKey() {
		if (!apiKey.trim()) {
			result = '请输入要测试的 API Key。';
			return;
		}
		result = '请在工具中用该 Key 与所选模型发起一次调用，并在「使用记录」中查看连通性、首 token 和耗时。';
	}
</script>

<div class="mb-8">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">key test</div>
	<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">Key 测试</h1>
	<p class="mt-2 text-sm text-muted">一键检测你的 API Key 是否正常工作，验证模型连通性和响应速度。</p>
</div>

<section class="max-w-3xl rounded-lg border border-line bg-card/60 p-5">
	<h2 class="m-0 text-base font-semibold">测试你的 API Key</h2>
	<p class="mt-2 text-sm text-faint">测试会发送一条简短消息，验证 Key、模型和网关链路。</p>
	<div class="mt-5 space-y-4">
		<div>
			<label class="mb-2 block text-sm font-medium" for="key-test-input">API Key</label>
			<input id="key-test-input" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 font-mono text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="sk-xxxxxxxxxxxxxxxx" bind:value={apiKey}>
		</div>
		<div>
			<label class="mb-2 block text-sm font-medium" for="key-test-model">模型</label>
			<select id="key-test-model" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" bind:value={model}>
				{#each models as item}
					<option value={item.value}>{item.label}</option>
				{/each}
			</select>
		</div>
		<button class="h-11 w-full rounded-md bg-brand px-5 text-sm font-semibold text-black" type="button" onclick={testKey}>开始测试</button>
	</div>
	{#if result}
		<p class="mt-4 rounded-md border border-amber-400/30 bg-amber-400/10 px-4 py-3 text-sm text-amber-200">{result}</p>
	{/if}
</section>

<section class="mt-6 max-w-3xl rounded-lg border border-line bg-card/60 p-5">
	<h3 class="text-base font-semibold">说明</h3>
	<ul class="mt-4 space-y-3 text-sm text-muted">
		<li>测试只发送一条简短消息，用于验证 Key、模型和网关链路，不会执行长上下文任务。</li>
		<li>只能测试你自己创建的 Key，测试入口绑定当前账户，避免跨账户探测。</li>
		<li>发起测试后，可在「使用记录」查看本次调用的连通性、首 token 和耗时，以及失败时的错误原因。</li>
	</ul>
</section>
