<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';

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
				{BRAND_NAME} 套餐说明、支付方式、创建 API Key 的完整流程。注册、支付、创建 Key 全流程 3 分钟以内，不需要梯子。
			</p>

			<section class={`mt-10 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第一步：注册账号</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					前往 <a class="text-brand" href="{base}/app/register">注册页面</a>，支持：
				</p>
				<ul class={`mt-4 space-y-2 leading-7 ${mutedClass}`}>
					<li>Google 账号一键登录</li>
					<li>邮箱 Magic Link —— 填邮箱，点登录链接进入</li>
				</ul>
				<p class={`mt-4 leading-7 ${mutedClass}`}>注册登录后，默认进入个人中心。</p>
				<p class={`mt-3 text-sm ${faintClass}`}>📌 请务必记住绑定的邮箱，售后和申诉都会通过邮箱核验身份。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第二步：选择套餐</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>{BRAND_NAME} 提供两种计费方式，按需选择。</p>
				<div class="mt-5 grid gap-4 md:grid-cols-2">
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">按量付费（PAYGO）—— 推荐新手</h3>
						<ul class={`mt-2 space-y-1 text-sm leading-6 ${mutedClass}`}>
							<li>1 元人民币 = $1 USD 额度</li>
							<li>最低充值 10 元起步</li>
							<li>余额永不过期</li>
							<li>按模型官方价 × VIP 倍率实时扣费</li>
						</ul>
						<p class={`mt-3 text-sm leading-6 ${faintClass}`}>适合：用量不稳定、想先试用、或者月均用量不到 $50 的用户。</p>
					</div>
					<div class={`rounded-lg border p-5 ${panelClass}`}>
						<h3 class="text-lg font-semibold">月卡订阅</h3>
						<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>每日固定 USD 额度，当日零点（UTC+8）刷新，不结转。</p>
						<p class={`mt-3 text-sm leading-6 ${faintClass}`}>月卡额度优先消耗，用完后自动切到按量付费余额，不会中断。</p>
					</div>
				</div>

				<div class="mt-5 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">套餐</th>
								<th class="py-2 pr-4 font-semibold">每日额度</th>
								<th class="py-2 pr-4 font-semibold">月价</th>
								<th class="py-2 font-semibold">适合人群</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">入门版</td><td class="py-2 pr-4">$20/天</td><td class="py-2 pr-4">¥199</td><td class="py-2">轻度日常</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">标准版</td><td class="py-2 pr-4">$50/天</td><td class="py-2 pr-4">¥399</td><td class="py-2">主力编码</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">专业版</td><td class="py-2 pr-4">$150/天</td><td class="py-2 pr-4">¥999</td><td class="py-2">重度使用、多项目并行</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">旗舰版</td><td class="py-2 pr-4">$300/天</td><td class="py-2 pr-4">¥1899</td><td class="py-2">团队 / 企业级重度</td>
							</tr>
						</tbody>
					</table>
				</div>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第三步：充值</h2>
				<ol class={`mt-3 space-y-2 leading-7 ${mutedClass}`}>
					<li>1. 进入 <a class="text-brand" href="{base}/app/billing">充值页面</a>。</li>
					<li>2. 选择套餐 → 点击「立即购买」。</li>
					<li>3. 支持支付方式：支付宝、微信、信用卡（通过 Creem / Stripe）。</li>
					<li>4. 支付完成后额度秒到账，在个人中心查看余额。</li>
				</ol>
				<p class={`mt-4 text-sm ${faintClass}`}>💰 新用户首充福利：首次充值享额外 10% 额度赠送。</p>
				<p class={`mt-2 text-sm ${faintClass}`}>
					🧾 企业开票：满 500 元起开，6%-10% 税点，联系
					<a class="font-mono text-brand" href="mailto:{BRAND_SUPPORT_EMAIL}">{BRAND_SUPPORT_EMAIL}</a>。
				</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">第四步：创建 API 密钥</h2>
				<ol class={`mt-3 space-y-2 leading-7 ${mutedClass}`}>
					<li>1. 左侧导航进入 <a class="text-brand" href="{base}/app/keys">API 密钥</a>。</li>
					<li>2. 点击「创建密钥」。</li>
					<li>3. 填写名称（用于区分用途，如 claude-code-mac、cursor-work）与分组（默认即可）。</li>
					<li>4. 点击创建 → 立即复制密钥保存（密钥仅显示一次）。</li>
				</ol>
				<p class={`mt-4 text-sm ${faintClass}`}>🔐 安全提示：API Key 等同于账号。泄露后任何人都能消耗你的额度，请勿提交到 GitHub 或贴到截图里。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">模型定价与分组</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					{BRAND_NAME} 的扣费公式：模型官方美元价 × VIP 倍率 = 实扣美元额度。
				</p>
				<div class="mt-5 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">模型</th>
								<th class="py-2 pr-4 font-semibold">输入 ($/M)</th>
								<th class="py-2 pr-4 font-semibold">输出 ($/M)</th>
								<th class="py-2 font-semibold">备注</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Claude Opus 4.6</td><td class="py-2 pr-4">$5</td><td class="py-2 pr-4">$25</td><td class="py-2">最强推理</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Claude Sonnet 4.6</td><td class="py-2 pr-4">$3</td><td class="py-2 pr-4">$15</td><td class="py-2">日常主力 ⭐</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">Claude Haiku 4.5</td><td class="py-2 pr-4">$1</td><td class="py-2 pr-4">$5</td><td class="py-2">快速轻量</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">GPT-5.4</td><td class="py-2 pr-4">$3</td><td class="py-2 pr-4">$15</td><td class="py-2">OpenAI 旗舰</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">GPT-5.3 Codex</td><td class="py-2 pr-4">$1.75</td><td class="py-2 pr-4">$14</td><td class="py-2">Codex 专用</td>
							</tr>
						</tbody>
					</table>
				</div>

				<p class={`mt-6 leading-7 ${mutedClass}`}>VIP 倍率（按累计充值自动升级）：</p>
				<div class="mt-3 overflow-x-auto">
					<table class={`w-full border-collapse text-sm ${mutedClass}`}>
						<thead>
							<tr class={`border-b ${borderClass} text-left`}>
								<th class="py-2 pr-4 font-semibold">等级</th>
								<th class="py-2 pr-4 font-semibold">门槛</th>
								<th class="py-2 pr-4 font-semibold">倍率</th>
								<th class="py-2 font-semibold">说明</th>
							</tr>
						</thead>
						<tbody>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">VIP0</td><td class="py-2 pr-4">默认</td><td class="py-2 pr-4">1.00x</td><td class="py-2">按官方价</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">VIP1</td><td class="py-2 pr-4">累充 $500</td><td class="py-2 pr-4">0.97x</td><td class="py-2">3% 折扣</td>
							</tr>
							<tr class={`border-b ${borderClass}`}>
								<td class="py-2 pr-4">VIP2</td><td class="py-2 pr-4">累充 $2000</td><td class="py-2 pr-4">0.95x</td><td class="py-2">5% 折扣</td>
							</tr>
							<tr>
								<td class="py-2 pr-4">VIP3</td><td class="py-2 pr-4">累充 $5000</td><td class="py-2 pr-4">0.93x</td><td class="py-2">7% 折扣</td>
							</tr>
						</tbody>
					</table>
				</div>

				<p class={`mt-6 leading-7 ${mutedClass}`}>扣费示例：用 Sonnet 4.6 处理 5000 输入 + 2000 输出 Token</p>
				<ul class={`mt-2 space-y-1 leading-7 ${mutedClass}`}>
					<li>输入：5000 ÷ 1,000,000 × $3 = $0.015</li>
					<li>输出：2000 ÷ 1,000,000 × $15 = $0.030</li>
					<li>合计 $0.045，约 4.5 分钱</li>
				</ul>
				<p class={`mt-2 text-sm ${faintClass}`}>VIP3 再打 0.93 折，实扣 $0.042。</p>
			</section>

			<section class={`mt-8 border-t pt-8 ${borderClass}`}>
				<h2 class="text-2xl font-semibold">下一步</h2>
				<p class={`mt-3 leading-7 ${mutedClass}`}>
					拿到 API Key 后 → <a class="text-brand" href="{base}/docs/install">安装配置工具</a>，把 Codex CLI、Claude Code 或其他兼容工具指向 {BRAND_NAME} 网关。
				</p>
				<p class={`mt-3 text-sm ${faintClass}`}>余额、订单、用量和密钥状态都以登录后的控制台为准。</p>
			</section>
		</article>
	</div>
</section>
