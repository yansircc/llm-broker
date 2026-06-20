<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	type ModelCard = {
		provider: string;
		name: string;
		id: string;
		desc: string;
		tags: string[];
		input: string;
		output: string;
		cacheRead?: string;
		perImage?: string;
		note?: string;
	};

	type ModelGroup = {
		provider: string;
		summary: string;
		models: ModelCard[];
	};

	const groups: ModelGroup[] = [
		{
			provider: 'Anthropic',
			summary: 'Claude 全系列满血支持，共用同一 Key、账本和套餐，按官方定价享汇率优惠。',
			models: [
				{ provider: 'Anthropic', name: 'Claude Opus 4.8', id: 'claude-opus-4-8', desc: '最新旗舰推理模型，超强编码与多步推理', tags: ['推理', '编码', '最强', '最新'], input: '$5', output: '$25', cacheRead: '$0.5' },
				{ provider: 'Anthropic', name: 'Claude Opus 4.7', id: 'claude-opus-4-7', desc: '旗舰推理模型', tags: ['推理', '编码', '最强'], input: '$5', output: '$25', cacheRead: '$0.5' },
				{ provider: 'Anthropic', name: 'Claude Opus 4.6', id: 'claude-opus-4-6', desc: '最强推理模型，适合复杂推理与高价值工程任务', tags: ['推理', '编码', '最强'], input: '$5', output: '$25', cacheRead: '$0.5' },
				{ provider: 'Anthropic', name: 'Claude Opus 4.5', id: 'claude-opus-4-5', desc: '高性能推理模型', tags: ['推理', '编码'], input: '$5', output: '$25', cacheRead: '$0.5' },
				{ provider: 'Anthropic', name: 'Claude Sonnet 4.7', id: 'claude-sonnet-4-7', desc: '最新高效编码模型', tags: ['编码', '高效', '主力'], input: '$3', output: '$15', cacheRead: '$0.3' },
				{ provider: 'Anthropic', name: 'Claude Sonnet 4.6', id: 'claude-sonnet-4-6', desc: '日常主力编码模型，速度与性能的最佳组合', tags: ['编码', '高效', '主力'], input: '$3', output: '$15', cacheRead: '$0.3' },
				{ provider: 'Anthropic', name: 'Claude Sonnet 4.5', id: 'claude-sonnet-4-5', desc: '高效编码模型', tags: ['编码', '高效'], input: '$3', output: '$15', cacheRead: '$0.3' },
				{ provider: 'Anthropic', name: 'Claude Haiku 4.5', id: 'claude-haiku-4-5', desc: '最快模型，适合轻量补全与快速迭代', tags: ['快速', '轻量'], input: '$0.8', output: '$4', cacheRead: '$0.08' }
			]
		},
		{
			provider: 'OpenAI',
			summary: 'GPT、o 系列与 Codex 全面可用，兼容 OpenAI 接口，按官方定价计费。',
			models: [
				{ provider: 'OpenAI', name: 'GPT-5.4', id: 'gpt-5.4', desc: 'OpenAI 最新旗舰模型，融合推理与编码', tags: ['推理', '编码', '多模态'], input: '$5', output: '$15' },
				{ provider: 'OpenAI', name: 'GPT-5', id: 'gpt-5', desc: 'GPT-5 基础版', tags: ['推理', '多模态'], input: '$5', output: '$15' },
				{ provider: 'OpenAI', name: 'GPT-5 Mini', id: 'gpt-5-mini', desc: 'GPT-5 轻量版，性价比更高', tags: ['快速', '高效'], input: '$1.5', output: '$6' },
				{ provider: 'OpenAI', name: 'GPT-4o', id: 'gpt-4o', desc: 'GPT-4o 多模态模型', tags: ['多模态'], input: '$2.5', output: '$10' },
				{ provider: 'OpenAI', name: 'o3', id: 'o3', desc: '推理模型，擅长数学/科学/编码', tags: ['推理', '编码'], input: '$10', output: '$40' },
				{ provider: 'OpenAI', name: 'o3-pro', id: 'o3-pro', desc: 'o3 增强版，更长思考更可靠', tags: ['推理', '最强'], input: '$20', output: '$80' },
				{ provider: 'OpenAI', name: 'o4-mini', id: 'o4-mini', desc: '最新轻量推理模型', tags: ['推理', '快速'], input: '$1.1', output: '$4.4' },
				{ provider: 'OpenAI', name: 'Codex Mini', id: 'codex-mini-latest', desc: '专为代码生成优化', tags: ['编码'], input: '$1.5', output: '$6' }
			]
		},
		{
			provider: 'Google',
			summary: 'Gemini 全系列可用，长上下文与多模态能力，适合高性价比批量处理。',
			models: [
				{ provider: 'Google', name: 'Gemini 3.5 Flash', id: 'gemini-3.5-flash', desc: 'Google 最新旗舰模型，最强推理与多模态能力', tags: ['推理', '多模态', '最新'], input: '$0.15', output: '$0.6' },
				{ provider: 'Google', name: 'Gemini 2.5 Pro', id: 'gemini-2.5-pro', desc: 'Google 顶级推理模型，百万上下文，支持多模态', tags: ['推理', '多模态', '长上下文'], input: '$1.25', output: '$10' },
				{ provider: 'Google', name: 'Gemini 2.5 Flash', id: 'gemini-2.5-flash', desc: 'Google 高速模型，性价比极高', tags: ['快速', '高效', '多模态'], input: '$0.15', output: '$0.6' },
				{ provider: 'Google', name: 'Gemini 2.5 Flash Lite', id: 'gemini-2.5-flash-lite', desc: 'Google 最轻量模型，最快速最低价', tags: ['快速', '轻量', '低价'], input: '$0.05', output: '$0.2' }
			]
		},
		{
			provider: '图像生成',
			summary: '兼容 OpenAI Images 接口，新建生图分组 Key 即可文生图、改图，按张计费。',
			models: [
				{ provider: '图像生成', name: 'OpenAI 图像生成', id: 'gpt-image-1', desc: '文生图，OpenAI Images 接口兼容（/v1/images/generations）', tags: ['文生图', 'OpenAI 兼容'], input: '', output: '', perImage: '$0.04', note: '需使用「生图分组」的 API Key · 也支持 dall-e-3 / gemini-image' }
			]
		}
	];
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-28 text-center">
		<span class="inline-flex items-center gap-2 rounded-full border border-brand/40 bg-brand/[0.06] px-3 py-1 text-xs font-medium text-brand">支持模型</span>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">一个 Key，调用所有模型</h1>
		<p class="mx-auto mt-4 max-w-3xl text-muted">
			Claude 全系列满血支持，GPT / o3 / Codex 同步可用。按官方定价计费，通过 {BRAND_NAME} 享 1.4 折汇率优惠。
		</p>
		<div class="mt-16 flex flex-wrap justify-center gap-3 text-sm">
			<a class="rounded-md bg-brand px-4 py-2 font-semibold text-black" href="{base}/app/register">创建 API Key</a>
			<a class="rounded-md border border-line bg-card/60 px-4 py-2 hover:border-brand/50" href="{base}/pricing">查看价格表</a>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-28">
	<div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
		{#each groups as group (group.provider)}
			<a class="rounded-lg border border-line bg-card/60 p-5 hover:border-brand/50" href="{base}/models#{group.provider}">
				<div class="font-mono text-xs uppercase tracking-wider text-brand">{group.provider}</div>
				<div class="mt-3 text-3xl font-bold">{group.models.length}</div>
				<p class="mt-2 text-sm text-faint">{group.summary}</p>
			</a>
		{/each}
	</div>
</section>

{#each groups as group (group.provider)}
	<section class="border-t border-line" id={group.provider}>
		<div class="mx-auto max-w-6xl px-5 py-28">
			<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
				<div>
					<div class="font-mono text-xs uppercase tracking-wider text-brand">{group.provider}</div>
					<h2 class="mt-2 text-2xl font-bold">{group.provider} 模型</h2>
					<p class="mt-2 max-w-3xl text-sm text-faint">{group.summary}</p>
				</div>
				<div class="rounded-md border border-line bg-card/60 px-3 py-2 font-mono text-xs text-faint">{group.models.length} models</div>
			</div>

			<div class="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
				{#each group.models as model (model.id)}
					<article class="flex flex-col rounded-lg border border-line bg-card/60 p-5">
						<div class="flex items-start justify-between gap-3">
							{#if model.provider === 'Anthropic'}
								<span class="rounded-full bg-brand px-2 py-0.5 text-[11px] font-semibold text-black">{model.provider}</span>
							{:else}
								<span class="rounded-full border border-line px-2 py-0.5 font-mono text-[11px] text-faint">{model.provider}</span>
							{/if}
						</div>
						<h3 class="mt-4 text-lg font-semibold">{model.name}</h3>
						<p class="mt-2 min-h-12 text-sm text-faint">{model.desc}</p>
						<div class="mt-3 flex flex-wrap gap-1.5">
							{#each model.tags as tag (tag)}
								<span class="rounded-full border border-line bg-black/20 px-2 py-0.5 text-[11px] text-muted">{tag}</span>
							{/each}
						</div>
						<div class="mt-4 grid gap-2 rounded-md border border-line bg-black/20 p-3 text-xs">
							{#if model.perImage}
								<div class="flex justify-between gap-3">
									<span class="text-faint">每张</span>
									<span class="font-mono text-brand">{model.perImage}/张</span>
								</div>
							{:else}
								<div class="flex justify-between gap-3">
									<span class="text-faint">输入</span>
									<span class="font-mono text-brand">{model.input}/MTok</span>
								</div>
								<div class="flex justify-between gap-3">
									<span class="text-faint">输出</span>
									<span class="font-mono text-brand">{model.output}/MTok</span>
								</div>
								{#if model.cacheRead}
									<div class="flex justify-between gap-3">
										<span class="text-faint">缓存读取</span>
										<span class="font-mono text-brand">{model.cacheRead}/MTok</span>
									</div>
								{/if}
							{/if}
						</div>
						<div class="mt-4">
							<div class="text-[11px] text-faint">模型 ID</div>
							<div class="mt-1 font-mono text-xs text-brand">{model.id}</div>
						</div>
						{#if model.note}
							<p class="mt-3 text-[11px] leading-relaxed text-faint">{model.note}</p>
						{/if}
					</article>
				{/each}
			</div>
		</div>
	</section>
{/each}

<section class="border-t border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="grid gap-6 md:grid-cols-3">
			<div class="rounded-lg border border-line bg-card/60 p-6">
				<h3 class="text-lg font-semibold text-brand">满血无阉割</h3>
				<p class="mt-2 text-sm leading-relaxed text-muted">所有模型保持原版能力，支持工具调用、联网搜索和长上下文。</p>
			</div>
			<div class="rounded-lg border border-line bg-card/60 p-6">
				<h3 class="text-lg font-semibold text-brand">100% 缓存支持</h3>
				<p class="mt-2 text-sm leading-relaxed text-muted">缓存读取 / 写入按官方价格计费，大幅降低重复请求成本。</p>
			</div>
			<div class="rounded-lg border border-line bg-card/60 p-6">
				<h3 class="text-lg font-semibold text-brand">官方定价</h3>
				<p class="mt-2 text-sm leading-relaxed text-muted">按 Anthropic / OpenAI 官方价格计费，{BRAND_NAME} 只赚汇率差。</p>
			</div>
		</div>
		<div class="mt-12 text-center">
			<h2 class="text-3xl font-bold tracking-tight">3 分钟开始你的 AI 编码之旅</h2>
			<p class="mx-auto mt-3 max-w-2xl text-muted">注册账号 → 充值 → 创建 API Key → 开始使用。就是这么简单。</p>
			<div class="mt-6 flex justify-center">
				<a class="rounded-md bg-brand px-10 py-3 text-base font-semibold text-black shadow-lg shadow-brand/20" href="{base}/app/register">立即注册</a>
			</div>
			<p class="mt-4 text-sm text-faint">1 元 = 1 刀额度 · 额度永不过期 · 支持微信 / 支付宝 / USDT</p>
		</div>
	</div>
</section>
