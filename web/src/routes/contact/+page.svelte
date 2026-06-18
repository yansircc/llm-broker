<script lang="ts">
	import { base } from '$app/paths';
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<header class="border-b border-line">
		<div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
			<a href="{base}/" class="font-semibold">CDX</a>
			<a class="rounded-md bg-brand px-3 py-2 text-sm font-semibold text-black" href="{base}/app/register">开始使用</a>
		</div>
	</header>
	<section class="mx-auto max-w-4xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">contact</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight">联系我们</h1>
		<p class="mt-4 text-muted">支付、额度、API 接入或企业使用问题，可以通过站点运营方配置的邮箱或客服渠道联系。</p>
		<div class="mt-8 rounded-lg border border-line bg-card/60 p-5">
			<div class="text-sm text-faint">当前版本未在前端硬编码客服邮箱，避免部署到不同域名时留下错误联系信息。</div>
		</div>
	</section>
</main>
