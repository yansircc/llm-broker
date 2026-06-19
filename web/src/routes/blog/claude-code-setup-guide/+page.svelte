<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<article class="mx-auto max-w-3xl px-5 py-14">
		<a class="font-mono text-sm text-brand hover:text-brand" href="{base}/blog">← 返回博客</a>
		<div class="mt-8 font-mono text-xs uppercase tracking-wider text-brand">claude-code</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">Claude Code 国内使用完全指南</h1>
		<p class="mt-4 text-muted">
			这篇指南面向已经习惯 Claude Code/Codex 终端工作流的开发者，重点是把调用入口切到 {BRAND_NAME}，并保持模型可用性边界清晰。
		</p>

		<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
			<section>
				<h2 class="text-2xl font-bold text-slate-100">当前可用边界</h2>
				<p>
					{BRAND_NAME} 当前提供 Codex 中转。Claude 家族会接入同一套餐表达，但不能把未接入模型当作可调用模型。实际模型列表始终以运行时接口和控制台展示为准。
				</p>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">基础配置</h2>
				<div class="mt-4 rounded-lg border border-line bg-card/70 p-5">
					<div class="font-mono text-xs text-faint">environment</div>
					<pre class="mt-3 text-xs"><code>OPENAI_API_KEY=your_api_key
OPENAI_BASE_URL=https://your-domain.example/v1
OPENAI_MODEL=codex-mini-latest</code></pre>
				</div>
				<p class="mt-4">
					核心是三件事：使用 {BRAND_NAME} 生成的 API Key，把 Base URL 指向兼容入口，并选择当前运行时确实返回的模型 ID。
				</p>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">验证顺序</h2>
				<ol class="mt-4 list-decimal space-y-3 pl-5">
					<li>先访问模型目录，确认目标模型存在。</li>
					<li>再发起一条最小请求，确认认证、余额和上游账号池都能通过。</li>
					<li>最后切到真实项目，让工具读取仓库并执行一个可回滚的小改动。</li>
				</ol>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">常见失败</h2>
				<p>
					401 通常是 Key 或 Base URL 配置错误；402/余额类错误说明账本准入失败；模型不存在则说明本地配置超前于服务端模型目录。不要用别名猜测，使用接口返回的 ID。
				</p>
			</section>
		</div>
	</article>
</main>
