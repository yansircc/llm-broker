<script lang="ts">
	import { base } from '$app/paths';
	import { browser } from '$app/environment';

	let origin = $state('https://your-domain.example');
	$effect(() => {
		if (browser) origin = window.location.origin;
	});
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">images</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">AI 生图 <span class="align-middle rounded bg-brand/10 px-2 py-1 font-mono text-xs text-brand">NEW</span></h1>
		<p class="mt-2 text-sm text-muted">OpenAI Images 兼容，生图分组 image 独立管理；当前为视觉入口，provider 接入后按张从余额扣费。</p>
	</div>
	<a class="rounded-md bg-brand px-4 py-2 text-sm font-semibold text-black" href="{base}/app/keys">去创建生图 Key</a>
</div>

<section class="grid gap-6 lg:grid-cols-[0.9fr_1.1fr]">
	<div class="space-y-4">
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<h2 class="m-0 text-base font-semibold">如何开通</h2>
			<ol class="mt-4 grid gap-3 text-sm text-muted">
				<li class="rounded-md border border-line bg-black/20 p-3">1. 创建一个独立 API Key。</li>
				<li class="rounded-md border border-line bg-black/20 p-3">2. 将该 Key 用于 POST /v1/images/generations。</li>
				<li class="rounded-md border border-line bg-black/20 p-3">3. 支持模型名：gpt-image-1、openai-image、dall-e-3。</li>
			</ol>
		</section>
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<h2 class="m-0 text-base font-semibold">定价</h2>
			<p class="mt-2 text-sm text-faint">按图片模型、尺寸和质量计费。本分支尚未接入生图 provider，价格表先作为视觉占位。</p>
			<div class="mt-4 grid gap-2 text-sm">
				<div class="flex justify-between rounded-md border border-line bg-black/20 p-3"><span>gpt-image-1</span><span class="font-mono text-brand">$0.04 / 张</span></div>
				<div class="flex justify-between rounded-md border border-line bg-black/20 p-3"><span>openai-image</span><span class="font-mono text-brand">$0.04 / 张</span></div>
				<div class="flex justify-between rounded-md border border-line bg-black/20 p-3"><span>dall-e-3</span><span class="font-mono text-brand">$0.04 / 张</span></div>
			</div>
		</section>
	</div>
	<section class="rounded-lg border border-line bg-[#050505]">
		<div class="border-b border-line px-4 py-3 font-mono text-xs text-faint">API 调用示例</div>
		<pre class="m-0 border-0 bg-transparent p-5 text-sm">curl {origin}/v1/images/generations \
  -H "Authorization: Bearer sk-你的生图Key" \
  -H "Content-Type: application/json" \
  -d '&#123;
    "model": "gpt-image-1",
    "prompt": "一只戴宇航头盔的橘猫，赛博朋克风格",
    "size": "1024x1024"
  &#125;'</pre>
	</section>
</section>

<div class="mt-6">
	<a class="inline-flex rounded-md border border-line bg-card px-4 py-2 text-sm hover:border-brand/50" href="{base}/app/billing">充值余额</a>
</div>
