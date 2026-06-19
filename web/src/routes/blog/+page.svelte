<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	type Post = {
		title: string;
		slug: string;
		category: string;
		date: string;
		summary: string;
		readTime: string;
	};

	const categories = ['All', 'analysis', 'tutorials', 'claude-code', 'pricing', 'tools'];

	const posts: Post[] = [
		{
			title: 'Claude Code 国内使用完全指南',
			slug: 'claude-code-setup-guide',
			category: 'claude-code',
			date: '2026-06-19',
			summary: `从 API Key、Base URL 到模型选择，说明如何把 Claude Code 风格工作流迁移到 ${BRAND_NAME} 当前可用的 Codex 中转，并为 Claude 家族接入预留配置习惯。`,
			readTime: '8 min read'
		},
		{
			title: '2026 AI API 定价全对比',
			slug: 'api-pricing-comparison',
			category: 'pricing',
			date: '2026-06-19',
			summary: '按输入、输出、cache 和套餐四个口径比较主流 API 计费方式，解释为什么公开报价和实际账单经常不是同一个数。',
			readTime: '10 min read'
		},
		{
			title: '2026 AI 编程工具横评',
			slug: 'best-ai-coding-tools-2026',
			category: 'tools',
			date: '2026-06-19',
			summary: '从终端代理、IDE 插件、网页工作台和自动化脚本四类工具看 AI 编程体验，重点比较接入成本、上下文管理和可验证性。',
			readTime: '9 min read'
		}
	];
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">blog</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">博客</h1>
		<p class="mt-4 max-w-3xl text-muted">产品接入、模型价格、Claude Code/Codex 工作流和 AI 编程工具分析。</p>
		<div class="mt-8 flex flex-wrap gap-2">
			{#each categories as category (category)}
				<button class={`rounded-md px-3 py-2 font-mono text-xs ${category === 'All' ? 'border-brand/60 bg-brand text-black' : 'border-line bg-card/60 text-faint'}`} type="button">{category}</button>
			{/each}
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-14">
	<div class="grid gap-5 md:grid-cols-3">
		{#each posts as post (post.slug)}
			<a class="group flex min-h-[300px] flex-col rounded-lg border border-line bg-card/60 p-6 hover:border-brand/50" href="{base}/blog/{post.slug}">
				<div class="flex items-center justify-between gap-3">
					<span class="rounded-md border border-line bg-black/20 px-2 py-1 font-mono text-xs text-brand">{post.category}</span>
					<span class="font-mono text-xs text-faint">{post.date}</span>
				</div>
				<h2 class="mt-5 text-xl font-semibold leading-tight group-hover:text-brand">{post.title}</h2>
				<p class="mt-4 flex-1 text-sm leading-relaxed text-faint">{post.summary}</p>
				<div class="mt-6 flex items-center justify-between border-t border-line pt-4 text-sm">
					<span class="font-mono text-xs text-muted">{post.readTime}</span>
					<span class="text-brand">阅读全文</span>
				</div>
			</a>
		{/each}
	</div>
</section>
