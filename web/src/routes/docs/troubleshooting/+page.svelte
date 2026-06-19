<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';

	const activeHref = '/docs/troubleshooting';
	const docsNav = [
		{ href: '/docs', label: `${BRAND_NAME} 文档`, summary: '产品说明、文档导航和快速开始' },
		{ href: '/docs/getting-started', label: '新手入门', summary: '理解 OpenAI、Anthropic、API 和终端工具' },
		{ href: '/docs/pricing', label: '注册与充值', summary: '注册、套餐、充值、API 密钥和模型分组' },
		{ href: '/docs/install', label: '安装配置', summary: 'Node.js、Claude Code、Codex CLI 和环境变量' },
		{ href: '/docs/usage', label: '入门使用手册', summary: '启动工具、切换模型、管理对话和查看用量' },
		{ href: '/docs/troubleshooting', label: '故障排查', summary: '代理、权限、401、余额、模型和网络问题' },
		{ href: '/docs/faq', label: '常见问题', summary: '基础、计费、能力、安全和稳定性问题' }
	];

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
</script>

<svelte:head>
	<title>故障排查 | {BRAND_NAME}</title>
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
			<div class="font-mono text-xs uppercase tracking-wider text-brand">Troubleshooting</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">故障排查</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				排查不要从猜测开始。先确认请求有没有到达 {BRAND_NAME}，再看工具配置、网络、鉴权、余额和模型。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">核心思路</h2>
				<ol class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>1. 先看终端完整错误，不只看最后一行。</li>
					<li>2. 确认当前终端里的环境变量值，而不是配置文件里“应该”的值。</li>
					<li>3. 到控制台看 API Key 状态、余额、用量记录和订单状态。</li>
					<li>4. 把工具问题和网关问题分开：命令找不到通常是本地安装问题，401 通常是鉴权问题。</li>
				</ol>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">万能提示词模板</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>向 AI 或支持人员求助时，直接提供可复现信息。</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>我在使用 {BRAND_NAME} 连接 AI 编程工具时出错。

工具：
操作系统：
启动命令：
base URL：
模型名：
错误原文：
我已经检查过：
- API Key 状态：
- 账户余额：
- 当前终端环境变量：
- 是否能打开控制台：</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">代理问题</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					如果你同时设置了系统代理、终端代理和工具内代理，先保留一种。代理配置错误会表现为连接超时、TLS 错误或请求发不到网关。
				</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>env | grep -i proxy
unset HTTP_PROXY HTTPS_PROXY ALL_PROXY</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">命令找不到</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					如果出现 <span class="font-mono">command not found</span>，说明工具没有安装成功，或安装目录不在 PATH 里。先确认版本命令能运行。
				</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>node -v
npm -v
codex --version
claude --version</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">权限错误</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					npm 全局安装权限错误通常是本机 Node 安装方式导致的。优先使用官方安装器、nvm 或用户级安装，不要随意对系统目录执行递归改权限。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">网络超时</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					超时先看 base URL 是否写对，再看本机网络和代理。能打开控制台不代表终端代理一定正常，终端是独立环境。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">401、余额、模型不存在</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-3">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">401</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>检查 API Key 是否完整、是否禁用、是否误用了其他平台的 Key。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">余额不足</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>先充值或等待订单入账。余额不足的请求不会进入上游。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">模型不存在</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>确认模型名来自当前模型列表。Claude 家族未接入完成前，不要把 Claude 模型当成已可用。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">联系支持</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					如果仍不能定位，把错误原文、API Key 后四位、订单号、请求时间和模型名发给支持邮箱：
					<a class="font-mono text-brand" href="mailto:{BRAND_SUPPORT_EMAIL}">{BRAND_SUPPORT_EMAIL}</a>。
				</p>
				<p class={`mt-3 text-sm ${faintClass}`}>不要发送完整 API Key、密码或浏览器 Cookie。</p>
			</section>
		</article>
	</div>
</section>
