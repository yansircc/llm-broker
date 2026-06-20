<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	type Post = {
		title: string;
		slug: string;
		tags: string[];
		date: string;
		summary: string;
		readTime: string;
	};

	const posts: Post[] = [
		{
			title: `Claude Code 国内使用完全指南：5 分钟配置 ${BRAND_NAME} 中转`,
			slug: 'claude-code-setup-guide',
			tags: ['tutorials', 'claude-code'],
			date: '2026-04-08',
			summary: `手把手教你在国内无障碍使用 Claude Code。从注册 ${BRAND_NAME}、创建 API Key 到配置环境变量，一文搞定。支持 macOS / Windows / Linux，兼容 VS Code 插件。`,
			readTime: '3 min read'
		},
		{
			title: '2026 年 AI API 定价全对比：Claude / GPT / Gemini 谁更划算（含实测成本）',
			slug: 'api-pricing-comparison',
			tags: ['analysis', 'pricing'],
			date: '2026-04-07',
			summary: `基于 2026 年 5 月 Anthropic、OpenAI、Google、AWS Bedrock 官网公开定价的横向对比。包含 SWE-bench / GPQA 性能基准、实测每千行代码成本，以及通过 ${BRAND_NAME} 中转后的国内最终成本测算。`,
			readTime: '9 min read'
		},
		{
			title: '2026 年最强 AI 编程工具横评：Claude Code / Codex / OpenClaw / Cursor',
			slug: 'best-ai-coding-tools-2026',
			tags: ['analysis', 'tools'],
			date: '2026-04-06',
			summary: '全面对比 2026 年主流 AI 编程工具：Claude Code、OpenAI Codex、OpenClaw、OpenCode、Cursor。从功能、模型、价格、国内可用性等维度帮你选出最适合的工具。',
			readTime: '4 min read'
		}
	];

	const dateFormatter = new Intl.DateTimeFormat('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});

	function formatDate(date: string): string {
		return dateFormatter.format(new Date(date));
	}

	function categoryCount(category: string): number {
		return posts.filter((post) => post.tags.includes(category)).length;
	}

	const categories = ['All', 'analysis', 'tutorials', 'claude-code', 'pricing', 'tools'];

	const featured = posts[0];
	const rest = posts.slice(1);
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">blog</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">博客</h1>
		<p class="mt-4 max-w-3xl text-muted">AI API 使用教程、模型更新、开发者最佳实践。</p>
		<div class="mt-16 flex flex-wrap gap-2">
			{#each categories as category (category)}
				<button class={`rounded-full px-4 py-2 font-mono text-xs ${category === 'All' ? 'border border-brand/60 bg-brand text-black' : 'border border-line bg-card/60 text-faint hover:border-brand/50'}`} type="button">
					{category === 'All' ? category : `${category} (${categoryCount(category)})`}
				</button>
			{/each}
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-28">
	{#if featured}
		<a class="group mb-8 grid gap-6 rounded-2xl border border-line bg-card/60 p-8 hover:border-brand/50 md:grid-cols-[1.4fr_1fr] md:items-center" href="{base}/blog/{featured.slug}">
			<div>
				<div class="flex flex-wrap items-center gap-2">
					{#each featured.tags as tag (tag)}
						<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">{tag}</span>
					{/each}
					<span class="font-mono text-xs uppercase tracking-wider text-brand">featured</span>
				</div>
				<h2 class="mt-5 text-2xl font-bold leading-tight group-hover:text-brand sm:text-3xl">{featured.title}</h2>
				<p class="mt-4 text-sm leading-relaxed text-faint">{featured.summary}</p>
			</div>
			<div class="flex flex-col gap-6 border-t border-line pt-5 text-sm md:border-l md:border-t-0 md:pl-6 md:pt-0">
				<span class="font-mono text-xs text-faint">{formatDate(featured.date)}</span>
				<span class="font-mono text-xs text-muted">{featured.readTime}</span>
				<span class="text-brand">阅读全文 →</span>
			</div>
		</a>
	{/if}

	{#if rest.length}
		<div class="grid gap-5 md:grid-cols-2">
			{#each rest as post (post.slug)}
				<a class="group flex min-h-[260px] flex-col rounded-2xl border border-line bg-card/60 p-6 hover:border-brand/50" href="{base}/blog/{post.slug}">
					<div class="flex flex-wrap items-center justify-between gap-3">
						<div class="flex flex-wrap gap-2">
							{#each post.tags as tag (tag)}
								<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">{tag}</span>
							{/each}
						</div>
						<span class="font-mono text-xs text-faint">{formatDate(post.date)}</span>
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
	{/if}
</section>
