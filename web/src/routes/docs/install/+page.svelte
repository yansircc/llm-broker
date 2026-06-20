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
				Windows / macOS 上从零安装 Node.js、Claude Code、Codex，并接入 {BRAND_NAME}。本章完成后你的电脑上会有：
			</p>
			<ul class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
				<li><strong>Node.js</strong> —— Claude Code / Codex 的运行环境</li>
				<li><strong>Claude Code</strong> —— Anthropic 终端助手</li>
				<li><strong>Codex CLI</strong> —— OpenAI 终端助手</li>
				<li><strong>CC Switch（可选）</strong> —— 图形化切换不同 API 供应商</li>
			</ul>
			<p class={`mt-4 rounded-md border p-4 text-sm leading-6 ${faintClass} ${borderClass}`}>
				⚠️ 开始前先退出所有翻墙 / 代理软件（Clash、V2Ray、机场客户端等）。浏览器能上网 ≠ 终端能上网。代理是安装过程中最常见的“隐形坑”，详见
				<a class="text-brand" href="{base}/docs/troubleshooting">故障排查 §代理问题</a>。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第一步：安装 Node.js</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Node.js 是 Claude Code 和 Codex 的运行环境，版本需 ≥ v18 LTS（推荐最新 LTS）。安装后两个版本号都能打印出来，表示安装成功。</p>
				<div class="mt-5 grid gap-4 md:grid-cols-3">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Windows</h3>
						<p class={`mt-2 text-xs leading-5 ${faintClass}`}>打开 PowerShell（Win → 输入 PowerShell → 回车），提示确认时输入 Y。装完关闭重开。</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>winget install OpenJS.NodeJS.LTS
node -v
npm -v</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">macOS</h3>
						<p class={`mt-2 text-xs leading-5 ${faintClass}`}>方式 A：到 nodejs.org 下载 .pkg 安装包双击安装。方式 B：Homebrew。</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>brew install node@20
node -v
npm -v</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Linux</h3>
						<p class={`mt-2 text-xs leading-5 ${faintClass}`}>其他发行版参考 NodeSource 官方说明。</p>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs</pre>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第二步：安装 Claude Code</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					Claude Code 是 Anthropic 的终端编码工具。推荐使用 npm 全局安装，配合 {BRAND_NAME} 的同一套网关地址和 API Key 即可使用。
				</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}># macOS / Linux（需要 sudo）
sudo npm install -g @anthropic-ai/claude-code@latest

# Windows（无需 sudo）
npm install -g @anthropic-ai/claude-code@latest

claude --version</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第三步：配置 Claude Code 连到 {BRAND_NAME}</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Claude Code 通过两个环境变量知道要走 {BRAND_NAME}：</p>
				<div class="mt-4 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">变量</th>
								<th class="py-2 font-semibold">值</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">ANTHROPIC_BASE_URL</td><td class="py-2 font-mono text-brand">{origin}</td>
							</tr>
							<tr>
								<td class="py-2 pr-4 font-mono">ANTHROPIC_AUTH_TOKEN</td><td class="py-2">你在 API 密钥页创建的 API Key</td>
							</tr>
						</tbody>
					</table>
				</div>
				<p class={`mt-5 leading-7 ${mutedClass}`}>下面是临时生效的写法，要永久生效请写入 shell 配置文件（zsh 用户改 <span class="font-mono">~/.zshrc</span>，bash 用户改 <span class="font-mono">~/.bashrc</span>）：</p>
				<div class="mt-4 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">macOS / Linux（zsh）</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>echo 'export ANTHROPIC_BASE_URL="{origin}"' >> ~/.zshrc
echo 'export ANTHROPIC_AUTH_TOKEN="sk-你的密钥"' >> ~/.zshrc
source ~/.zshrc</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Windows（PowerShell，永久）</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>[System.Environment]::SetEnvironmentVariable('ANTHROPIC_BASE_URL','{origin}','User')
[System.Environment]::SetEnvironmentVariable('ANTHROPIC_AUTH_TOKEN','sk-你的密钥','User')
# 关闭所有终端窗口重新打开才会生效</pre>
					</div>
				</div>
				<p class={`mt-4 text-sm ${faintClass}`}>
					部分 Claude Code 版本要求 <span class="font-mono">ANTHROPIC_AUTH_TOKEN</span>，因此优先使用它；如果你的版本只认 <span class="font-mono">ANTHROPIC_API_KEY</span>，把变量名替换即可。在任意项目目录下运行 <span class="font-mono">claude</span> 启动，首次进入一路回车即可。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第四步：安装 Codex CLI</h2>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}># macOS / Linux（需要 sudo）
sudo npm install -g @openai/codex@latest

# Windows（无需 sudo）
npm install -g @openai/codex@latest

codex --version</pre>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第五步：配置 Codex 连到 {BRAND_NAME}</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>Codex 使用 OpenAI 协议，环境变量名略有不同。base URL 指向 <span class="font-mono text-brand">{origin}/openai</span>：</p>
				<div class="mt-4 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">变量</th>
								<th class="py-2 font-semibold">值</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4 font-mono">OPENAI_BASE_URL</td><td class="py-2 font-mono text-brand">{origin}/openai</td>
							</tr>
							<tr>
								<td class="py-2 pr-4 font-mono">OPENAI_API_KEY</td><td class="py-2">你的 {BRAND_NAME} API Key</td>
							</tr>
						</tbody>
					</table>
				</div>
				<div class="mt-4 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">macOS / Linux（zsh）</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>echo 'export OPENAI_BASE_URL="{origin}/openai"' >> ~/.zshrc
echo 'export OPENAI_API_KEY="sk-你的密钥"' >> ~/.zshrc
source ~/.zshrc</pre>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">Windows（PowerShell，永久）</h3>
						<pre class={`mt-3 whitespace-pre-wrap text-sm ${codeClass}`}>[System.Environment]::SetEnvironmentVariable('OPENAI_BASE_URL','{origin}/openai','User')
[System.Environment]::SetEnvironmentVariable('OPENAI_API_KEY','sk-你的密钥','User')
# 重开终端后生效</pre>
					</div>
				</div>
				<p class={`mt-4 text-sm ${faintClass}`}>配置完成后运行 <span class="font-mono">codex</span> 启动，同样有引导，默认即可。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第六步（可选）：CC Switch 图形化管理</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					如果你同时用多个 API 供应商（{BRAND_NAME}、官方、其他中转），推荐 CC Switch，一个开源 GUI 工具，一键切换。下载地址：
					<span class="font-mono">https://github.com/farion1231/cc-switch/releases</span>（Windows 用 .msi，macOS 用 .dmg）。系统要求：Windows 10+ / macOS 12+ / Ubuntu 22.04+。
				</p>
				<p class="mt-4 font-semibold">添加 {BRAND_NAME} 供应商：</p>
				<ol class={`mt-2 space-y-2 leading-7 ${mutedClass}`}>
					<li>1. 打开 CC Switch → 点右上角 +。</li>
					<li>2. 选择「自定义配置」。</li>
					<li>3. 填写：供应商名称 {BRAND_NAME}；请求地址 <span class="font-mono text-brand">{origin}</span>（不勾选“完整 URL”）；API Key 贴入 {BRAND_NAME} 密钥。</li>
					<li>4. 点「添加」→ 列表中点「启用」→ 点「检测」。</li>
					<li>5. 顶部出现绿色提示 = 配置成功。</li>
				</ol>
				<p class={`mt-3 text-sm ${faintClass}`}>CC Switch 会自动帮你写好 Claude Code 的配置，不需要再手动改环境变量。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">其他工具接入</h2>
				<div class="mt-5 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">工具</th>
								<th class="py-2 pr-4 font-semibold">Base URL</th>
								<th class="py-2 font-semibold">密钥字段</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Cursor</td>
								<td class="py-2 pr-4">Settings → Models → Override OpenAI Base URL：<span class="font-mono text-brand">{origin}/v1</span></td>
								<td class="py-2">OpenAI API Key</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Continue (VS Code)</td>
								<td class="py-2 pr-4 font-mono text-brand">apiBase: "{origin}/v1"</td>
								<td class="py-2 font-mono">apiKey</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Cline (VS Code)</td>
								<td class="py-2 pr-4">API Provider 选 OpenAI Compatible，Base URL：<span class="font-mono text-brand">{origin}/v1</span></td>
								<td class="py-2">API Key</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Gemini CLI</td>
								<td class="py-2 pr-4">参考工具文档，指向 <span class="font-mono text-brand">{origin}</span></td>
								<td class="py-2">{BRAND_NAME} Key</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">JetBrains AI Assistant</td>
								<td class="py-2 pr-4">通过 Continue 插件或 Proxy 配置</td>
								<td class="py-2">{BRAND_NAME} Key</td>
							</tr>
						</tbody>
					</table>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">验证一切正常</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>在 Claude Code 或 Codex 里问一句：</p>
				<pre class={`mt-5 whitespace-pre-wrap text-sm ${codeClass}`}>你现在用的是什么模型？</pre>
				<p class={`mt-3 leading-7 ${mutedClass}`}>如果它回答了一个模型名（比如 claude-sonnet-4-6），说明整条链路打通。</p>
				<p class={`mt-4 text-sm ${faintClass}`}>
					遇到报错？ → 翻 <a class="text-brand" href="{base}/docs/troubleshooting">故障排查</a>。能用了？ → 进入
					<a class="text-brand" href="{base}/docs/usage">入门使用手册</a>。
				</p>
			</section>
		</article>
	</div>
</section>
