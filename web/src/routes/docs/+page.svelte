<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';

	let origin = $state('https://your-domain.example');
	$effect(() => {
		if (browser) origin = window.location.origin;
	});
</script>

<section class="mx-auto max-w-4xl px-5 py-16">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">docs</div>
	<h1 class="mt-3 text-4xl font-bold tracking-tight">快速接入</h1>
	<p class="mt-4 text-muted">注册、充值、创建 API Key，然后把客户端 base URL 指向 CDX。</p>

	<div class="mt-8 space-y-6">
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<h2 class="text-xl font-semibold">1. 创建 API Key</h2>
			<p class="mt-2 text-sm text-muted">登录客户控制台，在 API 密钥页创建 key。token 只展示一次。</p>
		</section>
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<h2 class="text-xl font-semibold">2. 配置 Codex CLI</h2>
			<pre class="mt-4 overflow-x-auto rounded-md border border-line bg-black p-4 font-mono text-sm text-brand">model_provider = "cdx"

[model_providers.cdx]
name = "CDX"
base_url = "{origin}/openai"
env_key = "CDX_API_KEY"
wire_api = "responses"</pre>
			<p class="mt-3 text-sm text-muted">本地环境变量设置为客户控制台创建的 API Key。</p>
		</section>
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<h2 class="text-xl font-semibold">3. 直接调用 Responses</h2>
			<pre class="mt-4 overflow-x-auto rounded-md border border-line bg-black p-4 font-mono text-sm text-brand">curl {origin}/v1/responses \
  -H "Authorization: Bearer cdx_live_xxx" \
  -H "Content-Type: application/json" \
  -d '&#123;"model":"gpt-5","input":"hello"&#125;'</pre>
		</section>
		<section class="rounded-lg border border-line bg-card/60 p-5">
			<h2 class="text-xl font-semibold">4. 扣费规则</h2>
			<p class="mt-2 text-sm text-muted">请求准入时余额必须大于 0。请求完成后按实际 usage 写入账本；最后一次请求允许把余额扣到短暂负数。</p>
		</section>
	</div>

	<a class="mt-8 inline-flex rounded-md border border-line px-4 py-2 text-sm hover:border-brand/50" href="{base}/docs/faq">查看 FAQ</a>
</section>
