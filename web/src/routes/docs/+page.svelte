<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_DESCRIPTION, BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';

	const activeHref = '/docs';
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
	<title>文档 | {BRAND_NAME}</title>
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
			<div class="font-mono text-xs uppercase tracking-wider text-brand">docs</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">{BRAND_NAME} 文档</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				{BRAND_NAME} AI API 中转服务使用指南 —— Claude Code / Codex / Cursor 零门槛接入
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">{BRAND_NAME} 使用指南</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					欢迎使用 {BRAND_NAME} —— Claude Code 超强平替，{BRAND_DESCRIPTION}。
				</p>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					无论你是刚接触终端 AI 工具的新手，还是已经在用 Claude Code / Codex 的开发者，本文档都会带你一步一步从注册走到能正常使用。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">什么是 {BRAND_NAME}</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					{BRAND_NAME} 把 Claude Code、OpenAI Codex、Cursor、VS Code (Continue / Cline)、JetBrains、Gemini CLI 等主流 AI 编码工具接入同一条稳定线路，提供统一的配置方式、透明计费与使用体验。
				</p>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li><strong>统一中转：</strong>一个 API 地址 <span class="font-mono text-brand">{origin}</span>，同时支持全部主流工具</li>
					<li><strong>无需梯子：</strong>国内直连，企业级服务器集群转发</li>
					<li><strong>透明计费：</strong>1 RMB = 1 USD 额度，约官方 1.4 折</li>
					<li><strong>账号安全：</strong>封号风险由 {BRAND_NAME} 承担，多条独立上游线路毫秒级故障切换</li>
					<li><strong>多平台：</strong>Windows / macOS / Linux 全覆盖</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">文档导航</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>按以下顺序阅读，适合零基础用户完整跑通：</p>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					{#each docsNav.slice(1) as item (item.href)}
						<a class={`rounded-lg border p-5 hover:border-brand/50 ${panelClass}`} href="{base}{item.href}">
							<h3 class="text-lg font-semibold">{item.label}</h3>
							<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>{item.summary}</p>
						</a>
					{/each}
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">快速开始（已有基础）</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>如果你熟悉终端工具，直接：</p>
				<ol class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>1. 前往 <a class="text-brand" href="{base}/app/register">注册页面</a> 注册并充值。</li>
					<li>2. 在 <a class="text-brand" href="{base}/app/keys">API 密钥</a> 页面创建 Key。</li>
					<li>3. 将 Base URL 配置为 <span class="font-mono text-brand">{origin}</span>（Codex 等 OpenAI 兼容工具使用 <span class="font-mono text-brand">{origin}/v1</span>），密钥贴入工具配置。</li>
					<li>4. 开始使用。</li>
				</ol>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>curl {origin}/v1/responses \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '&#123;"model":"gpt-5.3-codex","input":"hello"&#125;'</pre>
				<p class={`mt-4 text-sm ${faintClass}`}>
					遇到问题？先翻 <a class="text-brand" href="{base}/docs/troubleshooting">故障排查</a>，再翻 <a class="text-brand" href="{base}/docs/faq">FAQ</a>，仍未解决请发邮件到 <a class="font-mono text-brand" href="mailto:{BRAND_SUPPORT_EMAIL}">{BRAND_SUPPORT_EMAIL}</a>。
				</p>
			</section>
		</article>
	</div>
</section>
