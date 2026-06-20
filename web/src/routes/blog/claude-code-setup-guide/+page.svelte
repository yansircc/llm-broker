<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const author = 'CyberFisher';
	const date = '2026-04-08';
	const readTime = '3 min read';

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
		<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">tutorials</span>
		<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">claude-code</span>
	</div>
	<h1 class="mt-4 text-4xl font-bold tracking-tight sm:text-5xl">Claude Code 国内使用完全指南：5 分钟配置 {BRAND_NAME} 中转</h1>
	<div class="mt-4 flex flex-wrap items-center gap-2 text-xs text-faint">
		<span>{formattedDate}</span><span>·</span><span>{readTime}</span><span>·</span><span>{author}</span>
	</div>
	<p class="mt-4 text-muted">
		Claude Code 是 Anthropic 官方推出的 AI 编码 CLI 工具，能力强大但国内直连不稳定。通过 {BRAND_NAME} 中转，你可以在国内网络下流畅使用 Claude Code 的全部功能，无需科学上网。
	</p>

	<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
		<section>
			<h2 class="text-2xl font-bold text-slate-100">为什么需要中转？</h2>
			<p>Claude Code 默认连接 api.anthropic.com，在国内网络环境下：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>连接不稳定，经常超时</li>
				<li>官方订阅需要海外信用卡</li>
				<li>汇率换算后价格偏高</li>
			</ul>
			<p class="mt-3">通过 {BRAND_NAME} 中转，你可以享受：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>国内直连，零延迟</li>
				<li>1 元 = 1 美元额度，约官方 1.4 折</li>
				<li>满血模型，Opus 4.8 / Sonnet 4.6 / Haiku 4.5 无阉割</li>
			</ul>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">第一步：注册 {BRAND_NAME}</h2>
			<p>访问 <a class="text-brand hover:underline" href="{base}/app/register">{BRAND_NAME} 注册页面</a>，用邮箱注册账号。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">第二步：充值</h2>
			<p>进入「充值 / 订阅」页面，支持微信、支付宝和 USDT 支付。最低 20 元起充，额度永不过期。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">第三步：创建 API Key</h2>
			<p>进入「API 密钥」页面，点击「创建密钥」，选择合适的分组：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>入门组（0.7x）：性价比最高，适合日常开发</li>
				<li>Pro 组（0.8x）：百万上下文，长任务更稳</li>
				<li>Business 组（2.5x）：企业级稳定线路</li>
			</ul>
			<p class="mt-3">创建后复制 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">sk-</code> 开头的密钥。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">第四步：配置 Claude Code</h2>
			<p class="font-semibold text-slate-200">macOS / Linux</p>
			<p class="mt-1">打开终端，执行：</p>
			<div class="mt-3 rounded-lg border border-line bg-card/70 p-5">
				<pre class="text-xs"><code>export ANTHROPIC_BASE_URL="{origin}"
export ANTHROPIC_AUTH_TOKEN="sk-你的密钥"</code></pre>
			</div>
			<p class="mt-3">如需永久生效，添加到 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">~/.zshrc</code> 或 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">~/.bashrc</code>。</p>

			<p class="mt-5 font-semibold text-slate-200">Windows</p>
			<p class="mt-1">打开 CMD，执行：</p>
			<div class="mt-3 rounded-lg border border-line bg-card/70 p-5">
				<pre class="text-xs"><code>set ANTHROPIC_BASE_URL={origin}
set ANTHROPIC_AUTH_TOKEN=sk-你的密钥</code></pre>
			</div>

			<p class="mt-5 font-semibold text-slate-200">VS Code 插件</p>
			<p class="mt-1">编辑 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">~/.claude/settings.json</code>：</p>
			<div class="mt-3 rounded-lg border border-line bg-card/70 p-5">
				<pre class="text-xs"><code>&#123;
  "env": &#123;
    "ANTHROPIC_BASE_URL": "{origin}",
    "ANTHROPIC_AUTH_TOKEN": "sk-你的密钥"
  &#125;
&#125;</code></pre>
			</div>

			<p class="mt-5 font-semibold text-slate-200">CC-Switch 一键配置</p>
			<p class="mt-1">安装 CC-Switch 后，在 {BRAND_NAME} 的 API 密钥页面点击「导入」，一键完成配置。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">第五步：开始使用</h2>
			<p>配置完成后，直接在终端运行 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">claude</code>，即可开始 AI 编码。</p>
			<div class="mt-3 rounded-lg border border-line bg-card/70 p-5">
				<pre class="text-xs"><code>claude "帮我写一个 React 登录组件"</code></pre>
			</div>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">常见问题</h2>
			<p class="font-semibold text-slate-200">Q：提示 Not logged in？</p>
			<p class="mt-1">在 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">~/.claude.json</code> 中添加 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">"hasCompletedOnboarding": true</code>，或使用 CC-Switch 配置。</p>
			<p class="mt-4 font-semibold text-slate-200">Q：一次对话消耗很多额度？</p>
			<p class="mt-1">跟上下文长度有关。建议定期用 <code class="rounded bg-black/30 px-1.5 py-0.5 font-mono text-brand">/clear</code> 清理对话上下文。</p>
			<p class="mt-4 font-semibold text-slate-200">Q：支持哪些模型？</p>
			<p class="mt-1">支持 Claude 全系列：Opus 4.8、Sonnet 4.6、Haiku 4.5。默认使用 Sonnet 4.6。</p>
		</section>
	</div>

	<div class="mt-12 rounded-2xl border border-brand/40 bg-brand/10 p-8 text-center">
		<h2 class="text-2xl font-bold text-slate-100">准备好使用 AI API 了吗？</h2>
		<p class="mt-3 text-sm text-muted">免费试用 {BRAND_NAME}，新用户获得免费 API 积分。</p>
		<a class="mt-6 inline-block rounded-md bg-brand px-6 py-3 text-sm font-semibold text-black hover:opacity-90" href="{base}/app/register">立即注册</a>
	</div>
</article>
