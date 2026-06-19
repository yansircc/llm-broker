<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<article class="mx-auto max-w-3xl px-5 py-14">
		<a class="font-mono text-sm text-brand hover:text-brand" href="{base}/blog">← 返回博客</a>
		<div class="mt-8 font-mono text-xs uppercase tracking-wider text-brand">tools</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">2026 AI 编程工具横评</h1>
		<p class="mt-4 text-muted">
			AI 编程工具的差异不只在模型，还在上下文入口、可验证性、权限控制和账单透明度。本文按开发者真实工作流比较四类工具。
		</p>

		<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
			<section>
				<h2 class="text-2xl font-bold text-slate-100">终端代理</h2>
				<p>
					终端代理适合修改真实仓库、运行测试和保留命令证据。成本是权限边界必须清楚，模型目录和 API Key 配置也必须稳定。{BRAND_NAME} 当前主要服务这一类 Codex 工作流。
				</p>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">IDE 插件</h2>
				<p>
					IDE 插件适合局部补全和小范围编辑，反馈快，但复杂任务容易丢失执行证据。适合和终端代理配合，而不是完全替代。
				</p>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">网页工作台</h2>
				<p>
					网页工作台适合讨论方案、解释代码和生成草稿。它的优势是交互简单，弱点是对本地文件系统、测试和运行时证据的控制较弱。
				</p>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">自动化脚本</h2>
				<p>
					自动化脚本适合批量任务，例如生成报告、迁移内容或重复执行验证。关键指标是失败可恢复、日志可追踪、账单可解释。
				</p>
			</section>
		</div>
	</article>
</main>
