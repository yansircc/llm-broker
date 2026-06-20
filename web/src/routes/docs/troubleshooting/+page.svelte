<script lang="ts">
	import { browser } from '$app/environment';
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
				代理、权限、网络报错怎么办 —— 用 AI 一步步排查。安装和使用过程中遇到报错是正常的，本章给出通用的排查方法。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">核心思路：让 AI 陪你一步步查</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					你不需要自己看懂所有英文报错。截图 + 提示词 + 让 AI 一步一步带，就能解决 90% 的问题。推荐使用任何支持图片识别的 AI：豆包、ChatGPT、Claude 网页版，或者——如果 Codex 已经能用，直接问 Codex 最快。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">万能提示词模板</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>打开 AI 的新对话，先发这段，然后贴报错截图，按 AI 指示一步步操作即可：</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>我是命令行新手，不理解终端里各种信息的含义。我的核心目标只有一个：[替换成你的目标，
比如“安装 Node.js 并确认能正常使用”或“让 Claude Code 连上 {BRAND_NAME} 并发出第一条消息”]。
我会把终端报错原样文字或截图发给你，请你根据我发的内容一步一步告诉我该做什么。

要求：
- 始终围绕核心目标，不要跑题。
- 每次只告诉我当前这一步该做什么，不要一次给很多步骤。
- 用新手能听懂的话解释，不要默认我懂命令行。
- 明确告诉我：现在该输入什么、按什么键、看到什么算正常。
- 如果终端出现报错、警告、选择题或权限提示，请根据我发的原文帮我判断，不要让我自己猜。
- 如果某一步成功了，请明确告诉我“这一步完成，接下来做什么”。
- 请把自己当成远程陪我操作的人：我贴终端输出或截图，你判断下一步，直到目标完成。</pre>
				<p class={`mt-4 text-sm ${faintClass}`}>截图方法：macOS 用 Command + Shift + 4 框选区域；Windows 用 Win + Shift + S 框选区域。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">代理问题（最常见的坑）</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					如果你电脑上装了 Clash / V2Ray / Shadowsocks / 机场客户端等，它们可能悄悄影响终端的网络环境。典型症状：
				</p>
				<ul class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
					<li><span class="font-mono">npm install</span> 卡住不动</li>
					<li><span class="font-mono">ECONNREFUSED</span> / <span class="font-mono">ETIMEDOUT</span></li>
					<li><span class="font-mono">UNABLE_TO_VERIFY_LEAF_SIGNATURE</span> / <span class="font-mono">SELF_SIGNED_CERT_IN_CHAIN</span></li>
					<li>浏览器正常但终端命令连不上</li>
				</ul>
				<p class={`mt-5 font-semibold`}>快速尝试（解决 80% 情况）</p>
				<ol class={`mt-2 space-y-2 leading-7 ${mutedClass}`}>
					<li>1. 完全退出代理软件（右键菜单选「退出」，不是最小化）。</li>
					<li>2. 关掉当前终端窗口，重新开一个新终端（旧窗口里的环境变量不会自动刷新）。</li>
					<li>3. 再次执行之前的命令。</li>
				</ol>
				<p class={`mt-5 leading-7 ${mutedClass}`}>彻底检查：让 AI 带你查当前系统有没有代理软件在运行、终端环境变量里有没有设置代理（HTTP_PROXY / HTTPS_PROXY / ALL_PROXY）、npm 全局配置里有没有代理设置。手动清除 npm 代理：</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>npm config delete proxy
npm config delete https-proxy
npm config set registry https://registry.npmmirror.com</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">按症状分类的排查</h2>
				<div class="mt-5 space-y-4">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">① 命令找不到（command not found / 不是内部或外部命令）</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>常见原因：PATH 环境变量没刷新 → 关闭终端重开；或全局包安装路径没加入 PATH。可把以下提示词发给 AID：</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>我在终端输入 [node / npm / claude / codex] 提示 "command not found"
（或“不是内部或外部命令”）。我刚用 npm 安装过了，请帮我排查为什么找不到这个命令。
一次一步，我做完发结果给你。我的系统是 [Mac / Windows]。</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">② 权限错误（Permission denied / EACCES）</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>macOS/Linux 通常是 <span class="font-mono">sudo npm install -g ...</span>；Windows 上不需要 sudo。</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>我在执行 [你的命令] 时报了权限错误（Permission denied 或 EACCES）。
请一步步带我解决，不要让我做可能破坏系统的危险操作。
我的系统是 [Mac / Windows]。</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">③ 网络错误（timeout / fetch failed）</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>先按「代理问题」一节排查，确认无代理问题后再找 AI。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">④ Node.js 版本不兼容</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>我安装了 [工具名]，但运行时提示版本不兼容或需要更高版本的 Node.js。
我的 Node.js 版本是 [贴 node -v 的输出]。请帮我判断要不要升级、怎么升级。
一次一步。我的系统是 [Mac / Windows]。</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">⑤ 看不懂的其他报错</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>我在 [安装 / 使用] [工具名] 时遇到下面这个报错，完全看不懂。
请帮我判断是什么问题，然后一步步带我解决。一次一步。
我的系统是 [Mac / Windows]。</pre>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">症状 → 排查起点速查表</h2>
				<div class="mt-5 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">症状关键词</th>
								<th class="py-2 font-semibold">先排查</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">timeout / ECONNREFUSED / SSL / 证书</td><td class="py-2">代理问题（本章第三节）</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">command not found / 不是内部命令</td><td class="py-2">PATH / 环境变量（①）</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">Permission denied / EACCES</td><td class="py-2">权限（②）</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">engine "node" is incompatible</td><td class="py-2">Node 版本（④）</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">401 Unauthorized / invalid_api_key</td><td class="py-2">检查 ANTHROPIC_AUTH_TOKEN / OPENAI_API_KEY 是否拼错，或是否误把 Base URL 改成了官方地址</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">402 / insufficient_quota</td><td class="py-2">去控制台查余额，可能需要充值</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">5xx / 请求偶发失败</td><td class="py-2">过几分钟重试；持续出现请发邮件到支持邮箱</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">其他看不懂的英文</td><td class="py-2">截图发 AI（⑤）</td>
							</tr>
						</tbody>
					</table>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">{BRAND_NAME} 特有问题</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">配完环境变量还是连的官方</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>旧终端窗口里已经加载了原来的变量。关闭所有终端，重新打开再启动工具。macOS 上如果用 launchd 启动的 GUI 工具（如 IDE 插件），可能需要重启该 GUI 程序。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Claude Code 一直显示 "Connecting..."</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>检查 <span class="font-mono">ANTHROPIC_BASE_URL</span> 是否为 <span class="font-mono text-brand">{origin}</span>（不要有多余斜杠、不要写 /v1）。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Codex 报 404</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}><span class="font-mono">OPENAI_BASE_URL</span> 必须是 <span class="font-mono text-brand">{origin}/openai</span>，和 Claude Code 的写法不一样。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">余额明明有为什么扣不到</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>查使用记录：月卡用户当日额度优先；额度用完会自动切按量付费余额。若按量余额也为 0 就会失败。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">查完还是不行？</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					发邮件到 <a class="font-mono text-brand" href="mailto:{BRAND_SUPPORT_EMAIL}">{BRAND_SUPPORT_EMAIL}</a>，附上：
				</p>
				<ul class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
					<li>使用的工具和版本（<span class="font-mono">claude --version</span> / <span class="font-mono">codex --version</span>）</li>
					<li>系统信息（Windows 11 / macOS 14 等）</li>
					<li>完整的报错截图或文本</li>
					<li>你的注册邮箱（不要发 API Key）</li>
				</ul>
				<p class={`mt-3 text-sm ${faintClass}`}>不要发送完整 API Key、密码或浏览器 Cookie。</p>
			</section>
		</article>
	</div>
</section>
