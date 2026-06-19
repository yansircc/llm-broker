<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const activeHref = '/docs/install';
	const docsNav = [
		{ href: '/docs', label: `${BRAND_NAME} 文档`, summary: '产品说明、文档导航和快速开始' },
		{ href: '/docs/getting-started', label: '新手入门', summary: '理解 OpenAI、Anthropic、API 和终端工具' },
		{ href: '/docs/pricing', label: '注册与充值', summary: '注册、套餐、充值、API 密钥和模型分组' },
		{ href: '/docs/install', label: '安装配置', summary: 'Node.js、Claude Code、Codex CLI 和环境变量' },
		{ href: '/docs/usage', label: '入门使用手册', summary: '启动工具、切换模型、管理对话和查看用量' },
		{ href: '/docs/troubleshooting', label: '故障排查', summary: '代理、权限、401、余额、模型和网络问题' },
		{ href: '/docs/faq', label: '常见问题', summary: '基础、计费、能力、安全和稳定性问题' }
	];

	let origin = $state('https://your-domain.example');
	let query = $state('');
	let searchOpen = $state(false);
	let lightMode = $state(false);

	const searchResults = $derived(
		query.trim()
			? docsNav.filter((item) => `${item.label} ${item.summary}`.toLowerCase().includes(query.trim().toLowerCase()))
			: []
	);
	const shellClass = $derived(lightMode ? 'bg-slate-50 text-slate-950' : 'bg-bg text-slate-100');
	const panelClass = $derived(lightMode ? 'border-slate-200 bg-white' : 'border-line bg-card/70');
	const mutedClass = $derived(lightMode ? 'text-slate-600' : 'text-muted');
	const faintClass = $derived(lightMode ? 'text-slate-500' : 'text-faint');
	const borderClass = $derived(lightMode ? 'border-slate-200' : 'border-line');
	const codeClass = $derived(lightMode ? 'border-slate-200 bg-slate-950 text-brand' : 'border-line bg-black/40 text-brand');

	$effect(() => {
		if (browser) origin = window.location.origin;
	});
</script>

<svelte:head>
	<title>安装配置 | {BRAND_NAME}</title>
</svelte:head>

<section class={`min-h-screen border-t ${borderClass} ${shellClass}`}>
	<div class="mx-auto grid max-w-6xl gap-10 px-5 py-10 lg:grid-cols-[240px_minmax(0,1fr)]">
		<aside class="lg:sticky lg:top-24 lg:self-start">
			<div class={`rounded-lg border p-4 ${panelClass}`}>
				<div class="font-mono text-xs uppercase tracking-wider text-brand">Documentation</div>
				<div class="mt-2 text-lg font-semibold">{BRAND_NAME} 文档</div>
				<nav class="mt-5 space-y-1" aria-label="文档导航">
					{#each docsNav as item (item.href)}
						<a
							class={`block rounded-md px-3 py-2 text-sm ${
								item.href === activeHref
									? lightMode
										? 'bg-slate-900 text-white'
										: 'bg-brand text-black'
									: lightMode
										? 'text-slate-600 hover:bg-slate-100 hover:text-slate-950'
										: 'text-slate-300 hover:bg-white/5 hover:text-brand'
							}`}
							href="{base}{item.href}"
						>
							{item.label}
						</a>
					{/each}
				</nav>
				<div class="mt-5 flex gap-2">
					<button class="h-9 flex-1 rounded-md border px-3 text-sm" type="button" onclick={() => (searchOpen = !searchOpen)}>
						搜索
					</button>
					<button class="h-9 flex-1 rounded-md border px-3 text-sm" type="button" onclick={() => (lightMode = !lightMode)}>
						{lightMode ? '暗色' : '浅色'}
					</button>
				</div>
				{#if searchOpen}
					<div class={`mt-3 rounded-md border p-3 ${panelClass}`}>
						<input
							class={`h-10 max-w-none rounded-md border px-3 text-sm outline-none ${
								lightMode ? 'border-slate-200 bg-slate-50 text-slate-950' : 'border-line bg-black/40 text-slate-100'
							}`}
							placeholder="搜索文档"
							bind:value={query}
							aria-label="搜索文档"
						/>
						<div class="mt-3 space-y-1">
							{#if query.trim() && searchResults.length === 0}
								<div class={`text-xs ${faintClass}`}>没有匹配的章节</div>
							{/if}
							{#each searchResults as item (item.href)}
								<a class="block rounded px-2 py-1 text-sm hover:text-brand" href="{base}{item.href}">{item.label}</a>
							{/each}
						</div>
					</div>
				{/if}
			</div>
		</aside>

		<article class="min-w-0">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">Install</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">安装配置</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				先装运行环境，再装工具，最后把工具的 API 地址和密钥指向 {BRAND_NAME}。当前 Codex 可用，Claude 家族即将接入。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">安装 Node.js</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Node.js 提供 npm 包管理器，Claude Code 的 npm 安装方式和很多开发工具都会用到它。安装 LTS 版本后先确认命令可用。</p>
				<div class="mt-5 grid gap-4 md:grid-cols-3">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Windows</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>winget install OpenJS.NodeJS.LTS
node -v
npm -v</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">macOS</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>brew install node
node -v
npm -v</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Linux</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}># 使用系统包管理器或 nvm 安装 LTS
node -v
npm -v</pre>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">安装 Claude Code</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					Claude Code 是 Anthropic 的终端编码工具。{BRAND_NAME} 的 Claude 家族接入完成后，可以使用同一套网关地址和 API Key；当前请先以 Codex 路径完成验证。
				</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}># macOS / Linux / WSL
curl -fsSL https://claude.ai/install.sh | bash

# Windows PowerShell
irm https://claude.ai/install.ps1 | iex

# npm 方式
npm install -g @anthropic-ai/claude-code
claude --version</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">配置 Claude Code 到网关</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Claude 家族接入后，终端里设置 Anthropic 兼容环境变量，再启动 Claude Code。</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>export ANTHROPIC_BASE_URL="{origin}"
export ANTHROPIC_API_KEY="sk-xxx"
claude</pre>
				<p class={`mt-3 text-sm ${faintClass}`}>如果工具要求 token 变量名，也可以按工具版本使用 <span class="font-mono">ANTHROPIC_AUTH_TOKEN</span>，密钥值仍然来自 {BRAND_NAME} 控制台。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">安装 Codex CLI</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Codex CLI 当前可用。安装完成后用 API Key 登录或通过环境变量指定网关。</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}># macOS / Linux
curl -fsSL https://chatgpt.com/codex/install.sh | sh

# npm 方式
npm install -g @openai/codex
codex --version</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">配置 Codex CLI</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Codex CLI 使用 OpenAI 兼容环境变量。base URL 指向 <span class="font-mono text-brand">{origin}/openai</span>。</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>export OPENAI_BASE_URL="{origin}/openai"
export OPENAI_API_KEY="sk-xxx"
codex</pre>
				<p class={`mt-3 text-sm ${faintClass}`}>Windows PowerShell 使用 <span class="font-mono">$env:OPENAI_BASE_URL="{origin}/openai"</span> 和 <span class="font-mono">$env:OPENAI_API_KEY="sk-xxx"</span>。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">CC Switch 可选</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					如果你已经用 CC Switch 管理多个 Claude Code 配置，可以把 {BRAND_NAME} 做成一个独立 profile。它只是配置管理工具，不是使用 {BRAND_NAME} 的前置条件。
				</p>
			</section>
		</article>
	</div>
</section>
