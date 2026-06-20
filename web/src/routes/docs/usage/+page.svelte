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
				Claude Code / Codex 日常操作 —— 切模型、整理对话、看 Token 明细。记住一句话：终端里的 <span class="font-mono">claude</span> 和 <span class="font-mono">codex</span> 才是真正干活的入口；{BRAND_NAME} 负责把请求稳定地送到模型那里。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">启动前的 3 个确认</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>每次打开终端前：</p>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>代理软件是不是关掉了（或者你确认它不影响终端）。</li>
					<li>环境变量是不是生效（新开终端最保险）。</li>
					<li>个人中心余额够不够用。</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">先用哪个：Claude 还是 Codex</h2>
				<div class="mt-5 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">场景</th>
								<th class="py-2 font-semibold">推荐</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">想聊天、梳理思路、问“我现在该怎么做”</td><td class="py-2">Claude Code</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">想让它直接读代码、写代码、改文件、跑测试</td><td class="py-2">Claude Code（工程任务强）</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">想让它执行脚本、排查报错、快速回答</td><td class="py-2">Codex</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">不确定</td><td class="py-2">优先 Claude Code，日常编码更稳</td>
							</tr>
						</tbody>
					</table>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">启动工具</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">启动 Claude Code</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>进入项目目录后启动，首次进入一路回车用默认即可。</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>cd ~/你的项目目录
claude</pre>
						<p class={`mt-2 text-xs leading-5 ${faintClass}`}>进入后可先问：“你现在用的是什么模型？”</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">启动 Codex</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>同样有引导，默认即可。</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>codex</pre>
						<p class={`mt-2 text-xs leading-5 ${faintClass}`}>可以试：“帮我看看当前文件夹里有什么”。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">最常用的几个操作</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">1. 切换模型</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>进入 <span class="font-mono">claude</span> 或 <span class="font-mono">codex</span> 后输入 <span class="font-mono">/model</span>，上下方向键选择，回车确认。</p>
						<ul class={`mt-2 space-y-1 text-xs leading-5 ${faintClass}`}>
							<li>日常编码主力 → Claude Sonnet 4.6（性价比最高）</li>
							<li>复杂架构 / 深度推理 → Claude Opus 4.6</li>
							<li>大量简单查询 / 代码补全 → Claude Haiku 4.5</li>
							<li>兼容 OpenAI 生态 → GPT-5.4</li>
						</ul>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">2. 对话乱了怎么办</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>按顺序尝试：</p>
						<ul class={`mt-2 space-y-1 text-xs leading-5 ${faintClass}`}>
							<li><span class="font-mono">/compact</span> —— 压缩当前对话（保留核心上下文）</li>
							<li><span class="font-mono">/clear</span> —— 清空当前对话（话题跑偏，重新开始）</li>
							<li>直接开新终端窗口（彻底重置）</li>
						</ul>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">3. 让 AI 看文件 / 图片</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>不用打完整路径。在文件管理器找到文件，复制，回到终端对话框粘贴即可，Claude Code 和 Codex 都支持。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">4. 看起来卡死了</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>先别关：<span class="font-mono">Ctrl + O</span> 切换查看后台输出；等 10 秒观察是否仍在刷新；确实不动了才按 <span class="font-mono">Ctrl + C</span> 中断。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">5. 误操作了</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>先试 <span class="font-mono">Esc</span> 取消当前输入；不行再 <span class="font-mono">Ctrl + C</span> 中断；实在不行关窗口重开。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">怎么提问效果最好</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>新手最容易犯的错是问得太笼统。推荐万能句式：</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>我想完成 [你的目标]。你希望我提供哪些信息，能帮你更好地一步步带我完成？</pre>
				<p class={`mt-3 leading-7 ${mutedClass}`}>让 AI 反问你要上下文，比你自己琢磨“该说什么”高效得多。再补一句更简单的：</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>我是新手，请一步一步带我做，不要一次说太多。</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">看懂 Token 明细</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					打开 <a class="text-brand" href="{base}/app/usage">使用记录</a>，会看到一堆 Token 数字。含义：
				</p>
				<div class="mt-5 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">术语</th>
								<th class="py-2 pr-4 font-semibold">含义</th>
								<th class="py-2 font-semibold">比喻（打车）</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">输入 Token</td><td class="py-2 pr-4">你发给 AI 的内容</td><td class="py-2">上车前说了多少要求</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">输出 Token</td><td class="py-2 pr-4">AI 回给你的内容</td><td class="py-2">司机给的反馈和路线</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">缓存命中 Token</td><td class="py-2 pr-4">系统复用了之前算过的上下文，比普通输入便宜 10 倍</td><td class="py-2">这段路之前走过</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">总 Token</td><td class="py-2 pr-4">本次请求总消耗</td><td class="py-2">整趟里程</td>
							</tr>
						</tbody>
					</table>
				</div>
				<p class={`mt-5 font-semibold`}>省钱小技巧：</p>
				<ul class={`mt-2 space-y-2 leading-7 ${mutedClass}`}>
					<li>不需要上下文时用 <span class="font-mono">/clear</span> 清一下，避免反复带旧内容。</li>
					<li>用 <span class="font-mono">/compact</span> 压缩比新开对话更省钱（保留精华）。</li>
					<li>简单问题用 Haiku，复杂才用 Opus。</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">出现一大片红字时</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>不要慌，不要立刻关窗口。按优先级处理：</p>
				<ol class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
					<li>1. 截图或复制完整报错。</li>
					<li>2. 丢给还能用的 AI（Codex / Claude 网页版 / 豆包）。</li>
					<li>3. 仍搞不定 → <a class="text-brand" href="{base}/docs/troubleshooting">故障排查</a>。</li>
				</ol>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">新手只需记住这 8 条</h2>
				<ol class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
					<li>1. <span class="font-mono">claude</span> 启动 Claude Code，<span class="font-mono">codex</span> 启动 Codex。</li>
					<li>2. 第一次启动一路回车用默认。</li>
					<li>3. 日常编码优先 Claude Sonnet 4.6。</li>
					<li>4. 切换模型用 <span class="font-mono">/model</span>。</li>
					<li>5. 对话乱了先 <span class="font-mono">/compact</span>，再 <span class="font-mono">/clear</span>。</li>
					<li>6. 误操作先 <span class="font-mono">Esc</span>，再 <span class="font-mono">Ctrl + C</span>。</li>
					<li>7. 卡住了先 <span class="font-mono">Ctrl + O</span> 看后台输出。</li>
					<li>8. 看不懂的报错 → 截图丢给 AI。</li>
				</ol>
				<p class={`mt-5 text-sm ${faintClass}`}>
					想了解计费、退款、稳定性细节？ → <a class="text-brand" href="{base}/docs/faq">FAQ</a>
				</p>
			</section>
		</article>
	</div>
</section>
