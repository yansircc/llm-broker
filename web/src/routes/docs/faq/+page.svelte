<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_DESCRIPTION, BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';

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
					a: `${BRAND_NAME} 是${BRAND_DESCRIPTION}。用户只需要一把 API Key，就能把支持的 AI 编程工具接到同一套网关、余额和用量记录。`
				},
				{
					q: '国内能直接用吗？需要自己准备海外账号吗？',
					a: '目标体验是降低账号、网络和支付门槛。你只需要注册平台账号、充值、创建 API Key，再按文档配置工具。'
				},
				{
					q: '现在支持哪些工具？',
					a: '当前 Codex 可用，Claude 家族即将接入。OpenAI 兼容工具可以按模型和接口能力接入，实际可用范围以控制台和模型列表为准。'
				},
				{
					q: 'Cursor 可以用吗？',
					a: '只要工具支持自定义 OpenAI 兼容 base URL 和 API Key，就可以按同样方式配置。不同版本入口可能不同，优先使用工具自己的 API 设置页。'
				}
			]
		},
		{
			title: '定价与计费',
			items: [
				{
					q: 'PAYGO 和月卡有什么区别？',
					a: 'PAYGO 是按量充值、按实际 token 消耗扣费，适合新手试用和不稳定用量。月卡适合高频稳定使用，具体额度和周期以控制台为准。'
				},
				{
					q: '为什么推荐新手先用 PAYGO？',
					a: '新手通常还在验证工具、模型和工作流，小额按量充值能先确认链路跑通，避免一开始购买过大的固定套餐。'
				},
				{
					q: '实际扣费怎么计算？',
					a: '按模型、输入 token、输出 token 和缓存 token 计算。每次请求的扣费会进入用量记录，余额是请求准入的来源。'
				},
				{
					q: '充值后多久能用？',
					a: '支付成功并入账后即可使用。如果订单状态没有更新，先在订单页刷新；仍异常时带订单号联系支持。'
				},
				{
					q: '余额会过期吗？',
					a: '余额规则以控制台展示和服务条款为准。不要把本页当成账务凭证，最终以账户账本为准。'
				}
			]
		},
		{
			title: '功能与能力',
			items: [
				{
					q: '可以创建多个 API Key 吗？',
					a: '可以。建议按项目、工具或成员拆分 Key，方便统计用量、定位异常和单独禁用泄露密钥。'
				},
				{
					q: '模型不存在怎么办？',
					a: '先确认模型名来自当前模型列表。当前 Codex 可用，Claude 家族即将接入；未上线模型不要写入工具配置。'
				},
				{
					q: '可以让 AI 读取我的项目文件吗？',
					a: '这取决于本地工具权限。终端工具在你的项目目录运行时可以读取它有权限访问的文件，网关只负责 API 请求转发和计费。'
				},
				{
					q: '能不能上传图片或文件？',
					a: '取决于工具和模型接口是否支持相应输入。先用小文件验证，不要上传密钥、凭证、合同或客户隐私数据。'
				}
			]
		},
		{
			title: '安全与稳定',
			items: [
				{
					q: 'API Key 泄露怎么办？',
					a: '立即到控制台禁用或删除对应 Key，再创建新 Key。按项目拆分 Key 可以降低泄露影响范围。'
				},
				{
					q: '服务稳定性怎么保障？',
					a: '网关会做账号池调度、上游状态观测和失败处理。稳定性仍受上游模型、网络、账号容量和本地工具配置共同影响。'
				},
				{
					q: '平台会保存我的请求内容吗？',
					a: '系统会保留计费、审计和排障所需的请求元数据。敏感项目不要发送密钥、密码、Cookie 或无法外发的内部资料。'
				},
				{
					q: '问题排查需要提供什么？',
					a: `请提供错误原文、请求时间、模型名、API Key 后四位和订单号。不要发送完整 API Key。支持邮箱：${BRAND_SUPPORT_EMAIL}。`
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
