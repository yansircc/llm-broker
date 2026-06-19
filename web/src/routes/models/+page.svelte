<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	type ModelCard = {
		provider: string;
		name: string;
		id: string;
		useCase: string;
		input: string;
		output: string;
		cache: string;
		status: 'current' | 'planned' | 'reserved';
	};

	type ModelGroup = {
		provider: string;
		summary: string;
		models: ModelCard[];
	};

	const groups: ModelGroup[] = [
		{
			provider: 'Anthropic',
			summary: 'Claude 家族将接入同一 Key、账本和套餐表达；正式可用以后端模型目录为准。',
			models: [
				{ provider: 'Anthropic', name: 'Claude Opus 4.8', id: 'claude-opus-4-8', useCase: '复杂推理、长上下文架构设计', input: '$15.00/MTok', output: '$75.00/MTok', cache: 'write $18.75 · read $1.50', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Opus 4.7', id: 'claude-opus-4-7', useCase: '高难度代码审查和重构规划', input: '$15.00/MTok', output: '$75.00/MTok', cache: 'write $18.75 · read $1.50', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Opus 4.6', id: 'claude-opus-4-6', useCase: '复杂代理任务和深度分析', input: '$15.00/MTok', output: '$75.00/MTok', cache: 'write $18.75 · read $1.50', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Opus 4.5', id: 'claude-opus-4-5', useCase: '大型工程理解和方案推演', input: '$15.00/MTok', output: '$75.00/MTok', cache: 'write $18.75 · read $1.50', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Sonnet 4.7', id: 'claude-sonnet-4-7', useCase: '日常编码、测试和文档生成', input: '$3.00/MTok', output: '$15.00/MTok', cache: 'write $3.75 · read $0.30', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Sonnet 4.6', id: 'claude-sonnet-4-6', useCase: '稳定代码编辑和工具调用', input: '$3.00/MTok', output: '$15.00/MTok', cache: 'write $3.75 · read $0.30', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Sonnet 4.5', id: 'claude-sonnet-4-5', useCase: '主力开发模型和多文件修改', input: '$3.00/MTok', output: '$15.00/MTok', cache: 'write $3.75 · read $0.30', status: 'planned' },
				{ provider: 'Anthropic', name: 'Claude Haiku 4.5', id: 'claude-haiku-4-5', useCase: '轻量分类、摘要和快速补全', input: '$0.80/MTok', output: '$4.00/MTok', cache: 'write $1.00 · read $0.08', status: 'planned' }
			]
		},
		{
			provider: 'OpenAI',
			summary: '当前服务重点是 Codex 中转；其余 OpenAI 模型按目标商业页保留价格位。',
			models: [
				{ provider: 'OpenAI', name: 'GPT-5.4', id: 'gpt-5.4', useCase: '复杂任务规划、深度推理', input: '$5.00/MTok', output: '$15.00/MTok', cache: 'read $0.50', status: 'reserved' },
				{ provider: 'OpenAI', name: 'GPT-5', id: 'gpt-5', useCase: '通用推理和代码生成', input: '$2.50/MTok', output: '$10.00/MTok', cache: 'read $0.25', status: 'reserved' },
				{ provider: 'OpenAI', name: 'GPT-5 Mini', id: 'gpt-5-mini', useCase: '低成本批量任务和工具调用', input: '$0.25/MTok', output: '$2.00/MTok', cache: 'read $0.025', status: 'reserved' },
				{ provider: 'OpenAI', name: 'GPT-4o', id: 'gpt-4o', useCase: '多模态对话和通用应用', input: '$2.50/MTok', output: '$10.00/MTok', cache: 'read $1.25', status: 'reserved' },
				{ provider: 'OpenAI', name: 'o3', id: 'o3', useCase: '高强度推理和问题求解', input: '$2.00/MTok', output: '$8.00/MTok', cache: 'read $0.50', status: 'reserved' },
				{ provider: 'OpenAI', name: 'o3-pro', id: 'o3-pro', useCase: '更高推理预算的复杂分析', input: '$20.00/MTok', output: '$80.00/MTok', cache: 'read $5.00', status: 'reserved' },
				{ provider: 'OpenAI', name: 'o4-mini', id: 'o4-mini', useCase: '低延迟推理和自动化流程', input: '$1.10/MTok', output: '$4.40/MTok', cache: 'read $0.275', status: 'reserved' },
				{ provider: 'OpenAI', name: 'Codex Mini', id: 'codex-mini-latest', useCase: 'Codex CLI、编程代理和代码修改', input: '$1.50/MTok', output: '$6.00/MTok', cache: '按后端账单记录', status: 'current' }
			]
		},
		{
			provider: 'Google',
			summary: 'Gemini 组用于目标模型目录和套餐表达展示，当前不作为已上线承诺。',
			models: [
				{ provider: 'Google', name: 'Gemini 3.5 Flash', id: 'gemini-3.5-flash', useCase: '快速多模态和低延迟摘要', input: '$0.50/MTok', output: '$3.00/MTok', cache: 'read $0.125', status: 'reserved' },
				{ provider: 'Google', name: 'Gemini 2.5 Pro', id: 'gemini-2.5-pro', useCase: '长上下文推理和复杂分析', input: '$1.25/MTok', output: '$10.00/MTok', cache: 'read $0.31', status: 'reserved' },
				{ provider: 'Google', name: 'Gemini 2.5 Flash', id: 'gemini-2.5-flash', useCase: '高性价比批量处理', input: '$0.30/MTok', output: '$2.50/MTok', cache: 'read $0.075', status: 'reserved' },
				{ provider: 'Google', name: 'Gemini 2.5 Flash Lite', id: 'gemini-2.5-flash-lite', useCase: '轻量分类、抽取和路由', input: '$0.10/MTok', output: '$0.40/MTok', cache: 'read $0.025', status: 'reserved' }
			]
		},
		{
			provider: '图像生成',
			summary: '图像模型作为目标商业页条目展示，开放状态以控制台入口和后端能力为准。',
			models: [
				{ provider: 'OpenAI', name: 'OpenAI 图像生成', id: 'gpt-image-1', useCase: '文生图、改图和视觉素材生成', input: '按请求/尺寸', output: '按张计费', cache: '不适用', status: 'reserved' }
			]
		}
	];

	const statusCopy = {
		current: '当前 Codex 中转',
		planned: '即将接入',
		reserved: '价格位保留'
	};
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-16 text-center">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">models</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">目标模型目录</h1>
		<p class="mx-auto mt-4 max-w-3xl text-muted">
			{BRAND_NAME} 公开页按参考站结构展示模型分组和计费字段。当前实际可用能力集中在 Codex 中转；Claude 家族将接入同一套餐表达，其他条目仅为目录占位，不代表现在可调用。
		</p>
		<div class="mt-8 flex flex-wrap justify-center gap-3 text-sm">
			<span class="rounded-md border border-brand/50 bg-brand/[0.08] px-3 py-2 text-brand">21 个目标模型条目</span>
			<span class="rounded-md border border-line bg-card/60 px-3 py-2 text-faint">输入 / 输出 / cache / 模型 ID</span>
			<a class="rounded-md bg-brand px-4 py-2 font-semibold text-black" href="{base}/app/register">创建 API Key</a>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-14">
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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
		<div class="mx-auto max-w-6xl px-5 py-14">
			<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
				<div>
					<div class="font-mono text-xs uppercase tracking-wider text-brand">{group.provider}</div>
					<h2 class="mt-2 text-2xl font-bold">{group.provider} 模型</h2>
					<p class="mt-2 max-w-3xl text-sm text-faint">{group.summary}</p>
				</div>
				<div class="rounded-md border border-line bg-card/60 px-3 py-2 font-mono text-xs text-faint">{group.models.length} models</div>
			</div>

			<div class="mt-8 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
				{#each group.models as model (model.id)}
					<article class={`flex min-h-[300px] flex-col rounded-lg border bg-card/60 p-5 ${model.status === 'current' ? 'border-brand/60' : 'border-line'}`}>
						<div class="flex items-start justify-between gap-3">
							<div class="font-mono text-xs text-faint">{model.provider}</div>
							<span class={`rounded-md px-2 py-1 text-xs ${model.status === 'current' ? 'bg-brand text-black' : 'border border-line text-faint'}`}>{statusCopy[model.status]}</span>
						</div>
						<h3 class="mt-4 text-lg font-semibold">{model.name}</h3>
						<p class="min-h-12 text-sm text-faint">{model.useCase}</p>
						<div class="mt-4 rounded-md border border-line bg-black/20 p-3 font-mono text-xs text-brand">{model.id}</div>
						<div class="mt-4 grid gap-2 text-xs">
							<div class="flex justify-between gap-3 rounded-md border border-line bg-black/20 p-2">
								<span class="text-faint">input</span>
								<span class="font-mono text-brand">{model.input}</span>
							</div>
							<div class="flex justify-between gap-3 rounded-md border border-line bg-black/20 p-2">
								<span class="text-faint">output</span>
								<span class="font-mono text-brand">{model.output}</span>
							</div>
							<div class="rounded-md border border-line bg-black/20 p-2">
								<div class="text-faint">cache</div>
								<div class="mt-1 font-mono text-muted">{model.cache}</div>
							</div>
						</div>
					</article>
				{/each}
			</div>
		</div>
	</section>
{/each}

<section class="border-t border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-14">
		<div class="rounded-lg border border-line bg-card/60 p-6">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">availability</div>
			<h2 class="mt-3 text-2xl font-bold">实际可用性</h2>
			<p class="mt-3 text-sm leading-relaxed text-muted">
				本页覆盖参考站要求的完整公开模型结构，但不是运行时模型源。运行时模型、价格和准入结果必须以服务端模型目录、账单配置和控制台展示为准。
			</p>
		</div>
	</div>
</section>
