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
				如果你是第一次接触 Claude Code、Codex、终端 AI 工具，从这里开始。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">这些工具到底是什么</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					在开始配置之前，先花 3 分钟理清几个基本概念。如果你已经熟悉，直接跳到
					<a class="text-brand" href="{base}/docs/pricing">注册与充值</a>。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">两家 AI 公司，两类产品</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					目前在 AI 编码工具领域最具代表性的两家公司是 OpenAI 和 Anthropic。它们各自有两类产品：
				</p>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">OpenAI</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>网页聊天版：ChatGPT（给普通人）</p>
						<p class={`mt-1 text-sm leading-6 ${mutedClass}`}>终端工具：Codex CLI（给开发者）</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Anthropic</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>网页聊天版：Claude（给普通人）</p>
						<p class={`mt-1 text-sm leading-6 ${mutedClass}`}>终端工具：Claude Code（给开发者）</p>
					</div>
				</div>
				<p class={`mt-4 leading-7 ${mutedClass}`}>
					网页版只能聊天；Claude Code 和 Codex 能直接在你的电脑里读写文件、执行命令、帮你完成工作。它们不是“更强的聊天”，而是“长了手脚的助手”。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">什么是终端</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					终端（Terminal）是电脑里一个专门用来输入命令的窗口，通常看起来像一个黑色方框，很多人叫它“黑框框”。Claude Code 和 Codex 都运行在终端里。
				</p>
				<p class={`mt-4 font-semibold`}>如何打开终端</p>
				<ul class={`mt-2 space-y-2 leading-7 ${mutedClass}`}>
					<li><strong>Windows：</strong>按 Win 键 → 输入 Terminal 或 PowerShell → 回车</li>
					<li><strong>macOS：</strong>按 Command + 空格 → 输入 Terminal → 回车</li>
					<li><strong>Linux：</strong>桌面环境通常是 Ctrl + Alt + T</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">什么是 API</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>API 就是让软件程序去和 AI 大脑说话的通道。</p>
				<ul class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
					<li>你在网页和 Claude 聊天 = 人直接对 AI 说话</li>
					<li>软件（Claude Code、Cursor 等）通过 API 调用 AI = 软件替你对 AI 说话</li>
				</ul>
				<p class={`mt-4 leading-7 ${mutedClass}`}>
					所以 Claude Code 本质上是一个终端界面的客户端，它通过 API 把你的请求发给模型，再把结果返回到终端。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">这些工具跑在哪儿</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					你的电脑 + 云端 AI 模型。你的电脑负责打开终端、显示结果；真正的理解和生成由云端模型完成。所以使用时必须联网。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">为什么很多人用不起来</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>知道这些工具的人多，能稳定用起来的人少，常见卡点：</p>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li><strong>需要科学上网：</strong>官方 API 在国内直连不稳定</li>
					<li><strong>官方订阅贵：</strong>Claude Pro $20/月起，Max $200/月</li>
					<li><strong>封号风险：</strong>机房 IP 容易被风控，封了退款流程繁琐</li>
					<li><strong>API 按量计费失控：</strong>不熟悉的新手一天烧掉几十美金是常见的</li>
					<li><strong>多工具配置重复：</strong>Cursor、Codex、Claude Code 都要单独接一次</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">{BRAND_NAME} 解决了什么</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>{BRAND_NAME} 把上面 5 件事一次性解决：</p>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li><strong>无需梯子：</strong>国内直连</li>
					<li><strong>更便宜：</strong>1 RMB = 1 USD 额度，按量付费永不过期，月卡 $50/天起</li>
					<li><strong>不会封号：</strong>请求通过 {BRAND_NAME} 的企业账号池转发，风险在我们这边</li>
					<li><strong>一个 Key 通吃：</strong>Claude Code / Codex / Cursor / VS Code 共享同一个密钥</li>
					<li><strong>透明计费：</strong>每一次调用的输入/输出 Token 都在使用记录里看得到</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">谁适合用 {BRAND_NAME}</h2>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>想体验 Claude Code、Codex，但不想折腾海外账号和代理。</li>
					<li>Windows / macOS 用户想在本地终端跑 AI 编码工具。</li>
					<li>对成本敏感，或者想先小额试用。</li>
					<li>用多种 AI 工具（Cursor + Claude Code + Codex），不想每个都单独买。</li>
				</ul>
				<p class={`mt-5 text-sm ${faintClass}`}>
					准备好了就继续下一步 → <a class="text-brand" href="{base}/docs/pricing">注册与充值</a>
				</p>
			</section>
		</article>
	</div>
</section>
