<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';

	const activeHref = '/docs/faq';
	const docsNav = [
		{ href: '/docs', label: `${BRAND_NAME} 文档`, summary: '产品说明、文档导航和快速开始' },
		{ href: '/docs/getting-started', label: '新手入门', summary: '理解 OpenAI、Anthropic、API 和终端工具' },
		{ href: '/docs/pricing', label: '注册与充值', summary: '注册、套餐、充值、API 密钥和模型分组' },
		{ href: '/docs/install', label: '安装配置', summary: 'Node.js、Claude Code、Codex CLI 和环境变量' },
		{ href: '/docs/usage', label: '入门使用手册', summary: '启动工具、切换模型、管理对话和查看用量' },
		{ href: '/docs/troubleshooting', label: '故障排查', summary: '代理、权限、401、余额、模型和网络问题' },
		{ href: '/docs/faq', label: '常见问题', summary: '基础、计费、能力、安全和稳定性问题' }
	];
	const faqGroups = [
		{
			title: '基础问题',
			items: [
				{
					q: `${BRAND_NAME} 是什么？`,
					a: `${BRAND_NAME} 是 Claude Code 的超强平替，一站式 AI API 中转服务。我们通过独家企业级 Token 源合作，为开发者提供稳定、实惠的 Claude 全系列模型调用，兼容 Claude Code、Codex、Cursor、VS Code 等所有主流 AI 编码工具。`
				},
				{
					q: `为什么 ${BRAND_NAME} 的价格这么便宜？`,
					a: `${BRAND_NAME} 与多家云服务商建立了深度企业级合作，拥有独家批量采购渠道。我们不通过模型溢价赚钱，而是通过规模化采购和汇率差释放红利给开发者。举个例子：Claude Sonnet 4.6 官方输入价格 $3/百万 Token，按官方汇率约 ¥21.6，在 ${BRAND_NAME} 只需 ¥3——相当于官方 1.4 折。`
				},
				{
					q: '国内能直接用吗？需要梯子吗？',
					a: `不需要梯子。${BRAND_NAME} 是中转服务，所有请求通过我们的企业级服务器集群转发。您只需将工具的 API 地址设为网关地址，全球任何地方直连，无需任何代理工具。`
				},
				{
					q: '会封号吗？安全吗？',
					a: `绝对不会。所有 API 请求通过 ${BRAND_NAME} 的多条独立上游线路中转，由我们的企业账号池统一与 AI 厂商交互。封号风险完全由我们承担，您的 ${BRAND_NAME} 账号和 API Key 永远安全。我们还提供自动故障切换，即使某条线路异常也不影响您的使用。`
				},
				{
					q: '支持哪些模型？',
					a: '目前支持：Claude Opus 4.8 / 4.7 / 4.6 / 4.5（最强推理）；Claude Sonnet 4.7 / 4.6 / 4.5（日常编码主力）；Claude Haiku 4.5（快速轻量）；GPT-5 / GPT-5.4 / o3 / o4-mini / Codex（OpenAI 系列）；Gemini 3.5 Flash / 2.5 Pro / 2.5 Flash / 2.5 Flash Lite（Google 系列）。一个 API Key 即可调用全部模型。'
				}
			]
		},
		{
			title: '计费问题',
			items: [
				{
					q: 'RMB 充值为什么显示美元？',
					a: '平台以 USD 为计费单位（与官方一致），兑换比例为 1 RMB = 1 USD 额度。例如充值 100 元，获得 $100 额度。'
				},
				{
					q: '实际扣费怎么计算？',
					a: '实际扣费 = 模型官方价格 × VIP 倍率。例如你用 Claude Sonnet 4.6 写了一段代码，输入 5000 Token、输出 2000 Token：输入 5000 ÷ 1,000,000 × $3 = $0.015，输出 2000 ÷ 1,000,000 × $15 = $0.03，合计 $0.045，约 4.5 分钱。如果你是 VIP3（0.93 倍率），实际只扣 $0.045 × 0.93 = $0.042。每次调用的详细扣费可在「使用记录」中查看。'
				},
				{
					q: '月卡的每日额度能累积吗？',
					a: '不能。月卡提供每日固定 USD 额度（如标准版 $50/天），每日零点（UTC+8）自动刷新。当日未使用完的额度不结转到次日，但每天都有充沛新额度可用。月卡额度优先消耗，用完后自动切换到按量付费余额，不会中断使用。如果用量波动大，建议选择按量付费（PAYGO），额度永不过期。'
				}
			]
		},
		{
			title: '使用问题',
			items: [
				{
					q: '支持哪些开发工具？',
					a: `几乎所有支持自定义 API 端点的 AI 工具：Claude Code、OpenAI Codex CLI、Cursor、VS Code (Continue / Cline)、JetBrains IDE、Gemini CLI，以及任何兼容 OpenAI 或 Anthropic 格式的工具。只要支持自定义 API 端点，就能接入 ${BRAND_NAME}。配置只需 30 秒，详见文档中心。`
				},
				{
					q: '可以创建多个 API Key 吗？',
					a: '可以。你可以创建任意数量的 Key，按项目或用途区分。所有 Key 共享同一账户余额，每个 Key 的用量独立可追踪。'
				},
				{
					q: '如何查看使用记录？',
					a: '登录后进入使用记录页面，可按 Key、模型、时间筛选，支持导出 CSV。'
				}
			]
		},
		{
			title: '退款相关',
			items: [
				{
					q: '退款政策是什么？',
					a: '按量付费余额不退款但永不过期；月卡支持下单后 1 天内退款申请；退款金额 = 实付金额 - 已消耗算力 - 平台服务费（实付 × 20%）；超出 1 天可转为余额；建议新用户先小额充值测试。'
				}
			]
		},
		{
			title: '其他',
			items: [
				{
					q: '服务稳定性如何？',
					a: `${BRAND_NAME} 采用行业领先的多线路冗余架构：多条独立上游线路（含 AWS 企业版、Bedrock 专线等）；智能调度引擎实时监控每条线路的健康状态、延迟和成功率；自动故障切换在毫秒级完成；7×24 系统监控；历史可用性 99.9%+。我们与多家顶级云服务商签订了企业级 SLA 协议，确保 Token 源的持续供应。`
				},
				{
					q: '支持企业用户吗？',
					a: `支持。我们提供企业对公转账、开具发票（满 500 起开，6%-10% 税点）、定制 API 部署方案和专属客服。如需企业合作请联系 ${BRAND_SUPPORT_EMAIL}。`
				},
				{
					q: 'API Key 泄露怎么办？',
					a: '立即到控制台禁用或删除对应 Key，再创建新 Key。按项目拆分 Key 可以降低泄露影响范围。'
				}
			]
		}
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
	<title>常见问题 | {BRAND_NAME}</title>
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
			<div class="font-mono text-xs uppercase tracking-wider text-brand">FAQ</div>
			<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">常见问题</h1>
			<p class={`max-w-3xl text-base leading-7 ${mutedClass}`}>
				这些答案用于快速定位概念和配置问题；账户余额、订单、模型列表和套餐以登录后的控制台为准。
			</p>

			<div class="mt-10 space-y-10">
				{#each faqGroups as group (group.title)}
					<section class={`border-t pt-8 ${borderClass}`}>
						<h2 class="text-2xl font-semibold">{group.title}</h2>
						<div class="mt-5 space-y-4">
							{#each group.items as item (item.q)}
								<section class={`rounded-lg border p-5 ${panelClass}`}>
									<h3 class="text-lg font-semibold">{item.q}</h3>
									<p class={`mt-2 text-sm leading-6 ${mutedClass}`}>{item.a}</p>
								</section>
							{/each}
						</div>
					</section>
				{/each}
			</div>
		</article>
	</div>
</section>
