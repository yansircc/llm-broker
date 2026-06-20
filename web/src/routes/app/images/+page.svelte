<script lang="ts">
	import { base } from '$app/paths';
	import { browser } from '$app/environment';
	import Icon from '$lib/components/Icon.svelte';

	let origin = $state('https://your-domain.example');
	$effect(() => {
		if (browser) origin = window.location.origin;
	});
</script>

<div class="mb-8 flex items-start gap-3">
	<div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-md bg-brand/10 text-brand">
		<Icon name="image" size={20} />
	</div>
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">images</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">AI 生图 <span class="align-middle rounded bg-brand/10 px-2 py-1 font-mono text-xs text-brand">NEW</span></h1>
		<p class="mt-2 text-sm text-muted">OpenAI Images 兼容，生图分组 image 独立管理，按张从余额扣费。</p>
	</div>
</div>

<div class="max-w-3xl space-y-6">
	<section class="rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">如何开通</h2>
		<ol class="mt-4 grid gap-3 text-sm text-muted">
			<li class="rounded-md border border-line bg-black/20 p-3">1. 创建一个独立 API Key。</li>
			<li class="rounded-md border border-line bg-black/20 p-3">2. 将该 Key 用于 POST /v1/images/generations。</li>
			<li class="rounded-md border border-line bg-black/20 p-3">3. 支持模型名：gpt-image-1、openai-image、dall-e-3。</li>
		</ol>
		<div class="mt-5 flex flex-wrap gap-3">
			<a class="rounded-md bg-brand px-4 py-2 text-sm font-semibold text-black" href="{base}/app/keys">去创建生图 Key</a>
			<a class="inline-flex rounded-md border border-line bg-card px-4 py-2 text-sm hover:border-brand/50" href="{base}/app/billing">充值余额</a>
		</div>
	</section>

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

	<section class="rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">定价</h2>
		<p class="mt-2 text-sm text-faint">按图片模型、尺寸和质量计费。</p>
		<div class="mt-4 grid gap-2 text-sm">
			<div class="flex justify-between rounded-md border border-line bg-black/20 p-3"><span>gpt-image-1</span><span class="font-mono text-brand">$0.04 / 张</span></div>
			<div class="flex justify-between rounded-md border border-line bg-black/20 p-3"><span>openai-image</span><span class="font-mono text-brand">$0.04 / 张</span></div>
			<div class="flex justify-between rounded-md border border-line bg-black/20 p-3"><span>dall-e-3</span><span class="font-mono text-brand">$0.04 / 张</span></div>
		</div>
	</section>
</div>
