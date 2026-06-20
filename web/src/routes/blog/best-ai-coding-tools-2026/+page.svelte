<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const author = `${BRAND_NAME} 团队`;
	const date = '2026-04-06';
	const readTime = '4 min read';

	let origin = $state('https://your-domain.example');
	$effect(() => {
		if (browser) origin = window.location.origin;
	});

	const formattedDate = new Intl.DateTimeFormat('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	}).format(new Date(date));
</script>

<article class="mx-auto max-w-3xl px-5 py-16">
	<a class="font-mono text-sm text-brand hover:text-brand" href="{base}/blog">← 返回博客</a>
	<div class="mt-8 flex flex-wrap gap-2">
		<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">analysis</span>
		<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">tools</span>
	</div>
	<h1 class="mt-4 text-4xl font-bold tracking-tight sm:text-5xl">2026 年最强 AI 编程工具横评：Claude Code / Codex / OpenClaw / Cursor</h1>
	<div class="mt-4 flex flex-wrap items-center gap-2 text-xs text-faint">
		<span>{formattedDate}</span><span>·</span><span>{readTime}</span><span>·</span><span>{author}</span>
	</div>
	<p class="mt-4 text-muted">
		AI 编程工具已经成为开发者的标配。本文从多个维度对比主流工具，帮你做出最佳选择。
	</p>

	<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
		<section>
			<h2 class="text-2xl font-bold text-slate-100">工具一览</h2>
			<div class="mt-4 overflow-x-auto rounded-xl border border-line">
				<table class="w-full min-w-[640px] text-left text-sm">
					<thead class="bg-white/[0.03] font-mono text-xs text-faint">
						<tr><th class="px-4 py-3 font-medium">工具</th><th class="px-4 py-3 font-medium">类型</th><th class="px-4 py-3 font-medium">核心模型</th><th class="px-4 py-3 font-medium">特点</th></tr>
					</thead>
					<tbody class="divide-y divide-line">
						<tr><td class="px-4 py-3 font-medium text-slate-200">Claude Code</td><td class="px-4 py-3">CLI</td><td class="px-4 py-3">Claude Opus/Sonnet</td><td class="px-4 py-3">Anthropic 官方，Agent 能力最强</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">Codex</td><td class="px-4 py-3">CLI</td><td class="px-4 py-3">GPT-5.4 / Codex Mini</td><td class="px-4 py-3">OpenAI 官方，代码生成专精</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">OpenClaw</td><td class="px-4 py-3">CLI + 多端</td><td class="px-4 py-3">多模型</td><td class="px-4 py-3">开源，支持 WhatsApp/Telegram</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">OpenCode</td><td class="px-4 py-3">CLI</td><td class="px-4 py-3">多模型</td><td class="px-4 py-3">开源，75+ Provider 支持</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">Cursor</td><td class="px-4 py-3">IDE</td><td class="px-4 py-3">多模型</td><td class="px-4 py-3">AI-first 编辑器，GUI 友好</td></tr>
					</tbody>
				</table>
			</div>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">Claude Code：Agent 之王</h2>
			<p>Claude Code 是目前最强的 AI 编码 Agent。Opus 在复杂推理和多步骤任务上遥遥领先。</p>
			<p class="mt-3 font-semibold text-slate-200">优势：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>Agent 模式：自动读写文件、执行命令、迭代修复</li>
				<li>百万 Token 上下文，理解整个代码库</li>
				<li>支持 thinking 模式，推理过程可视化</li>
			</ul>
			<p class="mt-3 font-semibold text-slate-200">劣势：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>需要命令行操作，新手有门槛</li>
				<li>官方订阅价格偏高</li>
			</ul>
			<p class="mt-3"><strong class="text-slate-200">国内使用：</strong>通过 {BRAND_NAME} 中转，设置两个环境变量即可使用，1 元 = 1 刀额度。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">OpenAI Codex：代码专精</h2>
			<p>Codex 是 OpenAI 专为代码生成打造的工具，基于 GPT-5.4 和 Codex Mini 模型。</p>
			<p class="mt-3 font-semibold text-slate-200">优势：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>代码生成质量极高</li>
				<li>支持 OpenAI 生态</li>
				<li>轻量快速</li>
			</ul>
			<p class="mt-3"><strong class="text-slate-200">国内使用：</strong>设置 OPENAI_BASE_URL 和 OPENAI_API_KEY 指向 {BRAND_NAME} 即可。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">OpenClaw：全能龙虾</h2>
			<p>OpenClaw 是开源 AI 助手，不仅支持编码，还能通过 WhatsApp、Telegram 等聊天工具使用。</p>
			<p class="mt-3 font-semibold text-slate-200">优势：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>开源免费</li>
				<li>多端支持（CLI + 聊天）</li>
				<li>插件生态丰富</li>
			</ul>
			<p class="mt-3"><strong class="text-slate-200">国内使用：</strong>在 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">~/.openclaw/settings.json</code> 配置 {BRAND_NAME} 作为 provider。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">Cursor：GUI 党的最爱</h2>
			<p>Cursor 是一款 AI-first 代码编辑器，对不习惯命令行的开发者非常友好。</p>
			<p class="mt-3 font-semibold text-slate-200">优势：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>图形界面，上手零门槛</li>
				<li>内置 AI 对话和补全</li>
				<li>支持自定义 API endpoint</li>
			</ul>
			<p class="mt-3"><strong class="text-slate-200">国内使用：</strong>Settings → Models → 填入 {BRAND_NAME} 的 API Base URL 和 Key。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">选择建议</h2>
			<div class="mt-4 overflow-x-auto rounded-xl border border-line">
				<table class="w-full min-w-[480px] text-left text-sm">
					<thead class="bg-white/[0.03] font-mono text-xs text-faint">
						<tr><th class="px-4 py-3 font-medium">你是谁</th><th class="px-4 py-3 font-medium">推荐工具</th><th class="px-4 py-3 font-medium">推荐模型</th></tr>
					</thead>
					<tbody class="divide-y divide-line">
						<tr><td class="px-4 py-3 text-slate-200">全栈工程师</td><td class="px-4 py-3">Claude Code</td><td class="px-4 py-3 font-mono text-brand">Sonnet 4.6</td></tr>
						<tr><td class="px-4 py-3 text-slate-200">前端开发</td><td class="px-4 py-3">Cursor</td><td class="px-4 py-3 font-mono text-brand">Sonnet 4.6</td></tr>
						<tr><td class="px-4 py-3 text-slate-200">Python / 数据</td><td class="px-4 py-3">Codex</td><td class="px-4 py-3 font-mono text-brand">GPT-5.4</td></tr>
						<tr><td class="px-4 py-3 text-slate-200">开源爱好者</td><td class="px-4 py-3">OpenClaw</td><td class="px-4 py-3 font-mono text-brand">Sonnet 4.6</td></tr>
						<tr><td class="px-4 py-3 text-slate-200">Vibe Coder</td><td class="px-4 py-3">Claude Code</td><td class="px-4 py-3 font-mono text-brand">Opus 4.8</td></tr>
					</tbody>
				</table>
			</div>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">统一接入：一把 Key 搞定</h2>
			<p>不管你用哪个工具，{BRAND_NAME} 都能接入。一把 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">sk-</code> 开头的 Key，通过不同的 Base URL 配置，就能在所有工具之间自由切换：</p>
			<div class="mt-4 rounded-lg border border-line bg-card/70 p-5">
				<pre class="text-xs"><code>Claude Code / OpenCode / OpenClaw → {origin}
Codex / Cursor                    → {origin}/v1</code></pre>
			</div>
			<p class="mt-4">而且 {BRAND_NAME} 支持 CC-Switch 一键导入，在 API 密钥页面点一下就能自动配好所有工具。</p>
		</section>
	</div>

	<div class="mt-12 rounded-2xl border border-brand/40 bg-brand/10 p-8 text-center">
		<h2 class="text-2xl font-bold text-slate-100">一个 Key 用遍所有工具</h2>
		<p class="mt-3 text-sm text-muted">免费试用 {BRAND_NAME}，新用户获得免费 API 积分。</p>
		<a class="mt-6 inline-block rounded-md bg-brand px-6 py-3 text-sm font-semibold text-black hover:opacity-90" href="{base}/app/register">立即注册</a>
	</div>
</article>
