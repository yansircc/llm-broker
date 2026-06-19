<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const activeHref = '/docs/usage';
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
	<title>入门使用手册 | {BRAND_NAME}</title>
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
			<div class="font-mono text-xs uppercase tracking-wider text-brand">Usage</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">入门使用手册</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				配置完成后，先用最小任务验证链路，再把它接入真实项目。不要一开始就让 AI 改大范围代码。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">启动前确认</h2>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>控制台里已经有可用余额。</li>
					<li>API Key 没有被禁用，且复制时没有多出空格。</li>
					<li>终端能看到正确的 base URL 和 API Key 环境变量。</li>
					<li>当前要用的模型在模型页面或控制台列表里存在。</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">先用 Claude 还是 Codex</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					当前先用 Codex。Claude 家族即将接入后，再把 Claude Code 配置到同一套网关。两类工具的核心差异是环境变量名不同，不是充值和密钥体系不同。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">启动 Claude Code</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Claude 家族接入后，进入项目目录，设置 Anthropic 兼容变量并启动。</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>cd /path/to/project
export ANTHROPIC_BASE_URL="{origin}"
export ANTHROPIC_API_KEY="sk-xxx"
claude</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">启动 Codex</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Codex 当前可用。进入项目目录，设置 OpenAI 兼容变量并启动。</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>cd /path/to/project
export OPENAI_BASE_URL="{origin}/openai"
export OPENAI_API_KEY="sk-xxx"
codex</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">常用操作</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">切换模型</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>优先使用工具内置的 model 参数或配置文件。模型名必须来自 {BRAND_NAME} 的模型列表。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">对话乱了</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>新开会话，重新给目标、约束和当前文件。上下文污染时继续追问通常会放大错误。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">让 AI 看文件/图片</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>在项目目录启动工具，给出相对路径或把图片放到工具可访问的位置。不要把密钥文件发给 AI。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">看用量</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>进入 <a class="text-brand" href="{base}/app/usage">用量记录</a> 查看请求、模型、token 和扣费。按 Key 拆分能更快定位异常消耗。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">建议的第一条任务</h2>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>请阅读当前项目结构，只总结你看到的主要目录和启动方式。不要修改文件。</pre>
				<p class={`mt-3 text-sm ${faintClass}`}>这条任务能验证工具是否能读取项目，又不会产生代码改动。</p>
			</section>
		</article>
	</div>
</section>
