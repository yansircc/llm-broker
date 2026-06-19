<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const activeHref = '/docs/getting-started';
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
</script>

<svelte:head>
	<title>新手入门 | {BRAND_NAME}</title>
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
			<div class="font-mono text-xs uppercase tracking-wider text-brand">Getting started</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">新手入门</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				先把几个概念对齐，再开始配置工具。{BRAND_NAME} 的核心目标是让用户不用自己处理海外账号、网络、余额和模型路由。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">OpenAI 和 Anthropic 是什么</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					OpenAI 和 Anthropic 是模型提供方。Codex 属于 OpenAI 生态，Claude Code 属于 Anthropic 生态。
					它们的 API、鉴权、模型名和错误格式不同，但最终都可以被开发工具当成“给 AI 发请求、拿结果”的接口来使用。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">网页聊天版 vs 终端工具</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">网页聊天版</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>适合问答、写作和临时分析。它通常跑在浏览器里，不能天然读取你的项目目录，也不适合反复执行命令。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">终端工具</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>适合写代码、看文件、跑测试和修改项目。Claude Code、Codex CLI、Cursor 这类工具会在你的电脑或开发环境里工作。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">终端、API、工具跑在哪儿</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					终端是你输入命令的地方。API 是工具和模型服务之间的通信入口。工具本身运行在你的电脑、服务器或云开发环境里；
					它读取本地文件、发起 API 请求，然后把模型返回的结果变成命令、补丁或回答。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">为什么很多人用不起来</h2>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>海外账号、银行卡、手机号或组织限制导致账号注册失败。</li>
					<li>网络链路不稳定，工具能安装但请求经常超时。</li>
					<li>API Key、base URL、模型名和工具配置混在一起，错一个就不能用。</li>
					<li>不同工具使用不同环境变量，新手很难判断到底是哪一层出错。</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">产品解决什么</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					{BRAND_NAME} 把可用账号、请求转发、余额扣费、密钥管理和用量记录放在同一个控制台里。
					用户只需要充值、创建 API Key，并把工具的 base URL 指向网关。当前 Codex 可用，Claude 家族即将接入同一套账号和账本体验。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">谁适合用</h2>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>想尽快把 Codex CLI 接到稳定 API 的开发者。</li>
					<li>需要给团队统一管理 Key、余额和使用记录的小团队。</li>
					<li>正在从网页聊天过渡到终端 AI 编程工具的新手。</li>
					<li>未来准备同时使用 Codex、Claude Code、Cursor 等工具的人。</li>
				</ul>
			</section>
		</article>
	</div>
</section>
