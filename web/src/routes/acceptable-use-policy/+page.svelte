<script lang="ts">
	import { base } from '$app/paths';
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<header class="border-b border-line">
		<div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
			<a href="{base}/" class="font-semibold">CDX</a>
			<a class="rounded-md border border-line px-3 py-2 text-sm hover:border-brand/50" href="{base}/docs">文档</a>
		</div>
	</header>
	<section class="mx-auto max-w-4xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">acceptable use</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight">可接受使用政策</h1>
		<div class="mt-8 space-y-5 text-sm leading-relaxed text-muted">
			<p>禁止使用 CDX 进行违法、攻击性、安全绕过、批量滥用、垃圾注册、支付欺诈或侵犯第三方权益的行为。</p>
			<p>系统可能基于账号状态、余额、请求行为和上游反馈限制或暂停服务。</p>
			<p>如发现异常使用，部署方可冻结账号、停用 API Key 或保留相关审计记录。</p>
		</div>
	</section>
</main>
