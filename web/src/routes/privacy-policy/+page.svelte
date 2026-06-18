<script lang="ts">
	import { base } from '$app/paths';
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<header class="border-b border-line">
		<div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
			<a href="{base}/" class="font-semibold">CDX</a>
			<a class="rounded-md border border-line px-3 py-2 text-sm hover:border-brand/50" href="{base}/app/login">登录</a>
		</div>
	</header>
	<section class="mx-auto max-w-4xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">privacy</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight">隐私政策</h1>
		<div class="mt-8 space-y-5 text-sm leading-relaxed text-muted">
			<p>CDX 仅收集提供账号、支付、API Key、请求计费和安全审计所需的数据。</p>
			<p>请求日志用于计量、排障和风控。API Key、密码和支付密钥不会在前端展示或日志中明文输出。</p>
			<p>具体数据保留周期由部署方运营策略决定。</p>
		</div>
	</section>
</main>
