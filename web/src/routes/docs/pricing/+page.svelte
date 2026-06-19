<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const activeHref = '/docs/pricing';
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
	<title>注册与充值 | {BRAND_NAME}</title>
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
			<div class="font-mono text-xs uppercase tracking-wider text-brand">Billing</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">注册与充值</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				先注册，再充值，最后创建 API Key。余额是请求准入的真实边界；没有余额时请求不会进入上游。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第一步：注册账号</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					进入 <a class="text-brand" href="{base}/app/register">注册页面</a>，使用邮箱创建账号。注册完成后登录控制台，确认账户状态正常。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第二步：选择套餐</h2>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">PAYGO</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>按量充值、按实际 token 消耗扣费。新手建议先用 PAYGO，小额验证工具、模型和工作流都跑通后再提高预算。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">月卡订阅</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>适合稳定高频使用。月卡额度、刷新周期和可用模型以控制台展示为准；如果当前账号还没有订阅入口，先使用 PAYGO。</p>
					</div>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第三步：充值</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					进入 <a class="text-brand" href="{base}/app/billing">充值页面</a> 选择金额并完成支付。支付成功后，余额会写入账户账本。
					如果页面显示订单处理中，先刷新订单状态，不要重复创建多个同金额订单。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第四步：创建 API 密钥</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					进入 <a class="text-brand" href="{base}/app/keys">API Key 页面</a> 创建密钥。建议按项目、工具或团队成员拆分 Key，泄露时可以单独禁用，不影响其他场景。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">模型定价与分组</h2>
					<p class={`mt-3 leading-7 ${mutedClass}`}>
						模型价格按输入、输出和缓存 token 计费，不同模型属于不同能力和容量分组。当前 Codex 可用，Claude 家族即将接入；
						<a class="text-brand" href="{base}/models">模型页面</a> 用于展示目标能力和参考价格，实际可调用模型、单价和上下文长度以控制台/API 返回为准。
					</p>
				<ul class={`mt-4 space-y-3 leading-7 ${mutedClass}`}>
					<li>轻量任务优先选低成本模型。</li>
					<li>复杂代码修改、长上下文分析再使用更强模型。</li>
					<li>同一项目不要混用太多 Key，否则后续排查用量会变复杂。</li>
				</ul>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">下一步</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					完成充值和 API Key 后，进入 <a class="text-brand" href="{base}/docs/install">安装配置</a>，把 Codex CLI 或其他 OpenAI 兼容工具指向 {BRAND_NAME} 网关。
				</p>
				<p class={`mt-3 text-sm ${faintClass}`}>余额、订单、用量和密钥状态都以登录后的控制台为准。</p>
			</section>
		</article>
	</div>
</section>
