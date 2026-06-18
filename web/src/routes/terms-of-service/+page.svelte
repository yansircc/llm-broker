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
		<div class="font-mono text-xs uppercase tracking-wider text-brand">terms</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight">服务条款</h1>
		<div class="mt-8 space-y-5 text-sm leading-relaxed text-muted">
			<p>用户需自行确保 API 使用场景合法合规，不得用于攻击、欺诈、垃圾信息、绕过安全限制或其他滥用行为。</p>
			<p>CDX 按预付费额度提供中转服务。请求准入依赖账号状态、API Key 状态和余额。</p>
			<p>因上游模型、网络、账号池容量或支付通道造成的临时不可用，部署方会按实际运营策略处理。</p>
		</div>
	</section>
</main>
