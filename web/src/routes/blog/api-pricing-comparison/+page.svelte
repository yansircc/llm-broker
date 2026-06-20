<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';

	const author = `${BRAND_NAME} 团队`;
	const date = '2026-04-07';
	const readTime = '9 min read';

	const formattedDate = new Intl.DateTimeFormat('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	}).format(new Date(date));
</script>

<article class="mx-auto max-w-3xl px-5 py-16">
	<a class="font-mono text-sm text-brand hover:text-brand" href="{base}/blog">← 返回博客</a>
	<div class="mt-8 flex flex-wrap gap-2">
		<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">analysis</span>
		<span class="rounded-full border border-line bg-black/20 px-3 py-1 font-mono text-xs text-brand">pricing</span>
	</div>
	<h1 class="mt-4 text-4xl font-bold tracking-tight sm:text-5xl">2026 年 AI API 定价全对比：Claude / GPT / Gemini 谁更划算（含实测成本）</h1>
	<div class="mt-4 flex flex-wrap items-center gap-2 text-xs text-faint">
		<span>{formattedDate}</span><span>·</span><span>{readTime}</span><span>·</span><span>{author}</span>
	</div>
	<p class="mt-4 text-muted">
		基于 2026 年 5 月 Anthropic、OpenAI、Google、AWS Bedrock 官网公开定价的横向对比。包含 SWE-bench / GPQA 性能基准、实测每千行代码成本，以及通过 {BRAND_NAME} 中转后的国内最终成本测算。
	</p>

	<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
		<section>
			<h2 class="text-2xl font-bold text-slate-100">官方定价对比</h2>
			<p>以各家旗舰与主力编码模型为例，单价按每百万 Token（MTok）计：</p>
			<div class="mt-4 overflow-x-auto rounded-xl border border-line">
				<table class="w-full min-w-[560px] text-left text-sm">
					<thead class="bg-white/[0.03] font-mono text-xs text-faint">
						<tr><th class="px-4 py-3 font-medium">模型</th><th class="px-4 py-3 font-medium">输入</th><th class="px-4 py-3 font-medium">输出</th><th class="px-4 py-3 font-medium">定位</th></tr>
					</thead>
					<tbody class="divide-y divide-line">
						<tr><td class="px-4 py-3 font-medium text-slate-200">Claude Opus 4.8</td><td class="px-4 py-3 font-mono text-brand">$5</td><td class="px-4 py-3 font-mono text-brand">$25</td><td class="px-4 py-3">旗舰推理 / 架构</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">Claude Sonnet 4.6</td><td class="px-4 py-3 font-mono text-brand">$3</td><td class="px-4 py-3 font-mono text-brand">$15</td><td class="px-4 py-3">日常编码主力</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">Claude Haiku 4.5</td><td class="px-4 py-3 font-mono text-brand">$0.8</td><td class="px-4 py-3 font-mono text-brand">$4</td><td class="px-4 py-3">轻量 / 批量</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">GPT-5.4</td><td class="px-4 py-3 font-mono text-brand">$5</td><td class="px-4 py-3 font-mono text-brand">$15</td><td class="px-4 py-3">复杂编码 / 多步推理</td></tr>
						<tr><td class="px-4 py-3 font-medium text-slate-200">Gemini 2.5 Pro</td><td class="px-4 py-3 font-mono text-brand">$1.25</td><td class="px-4 py-3 font-mono text-brand">$10</td><td class="px-4 py-3">长上下文 / 多模态</td></tr>
					</tbody>
				</table>
			</div>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">性能基准</h2>
			<p>价格之外更要看性价比。在 SWE-bench（真实代码修复）与 GPQA（研究生级推理）两组公开基准上，Claude Opus / Sonnet 与 GPT-5.4 处于第一梯队，Gemini 2.5 Pro 在长上下文任务上有优势，Haiku / Flash 级模型则用更低成本覆盖轻量场景。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">实测成本</h2>
			<p>以一次典型编码对话为基准（约 5,000 输入 + 2,000 输出 Token）：</p>
			<ul class="mt-2 list-disc space-y-1.5 pl-5">
				<li>Sonnet 4.6：5,000 ÷ 1M × $3 + 2,000 ÷ 1M × $15 ≈ <span class="font-mono text-brand">$0.045</span></li>
				<li>Haiku 4.5：同口径约 <span class="font-mono text-brand">$0.012</span></li>
				<li>Opus 4.8：同口径约 <span class="font-mono text-brand">$0.075</span></li>
			</ul>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">通过 {BRAND_NAME} 的最终成本</h2>
			<p>{BRAND_NAME} 以 <span class="font-mono text-brand">1 RMB = 1 USD</span> 结算，按官方汇率约 7.2 折算，等于官方约 1.4 折。上面 Sonnet 单次 $0.045，折合约 ¥0.045，充值 ¥1 可对话约 22 次。缓存命中还能进一步压低成本。</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">结论</h2>
			<p>日常编码 Sonnet 性价比最高；预算极敏感可用 Gemini Flash / Haiku；复杂推理与架构设计再上 Opus / o3。无论选哪个，通过 {BRAND_NAME} 一把 Key 即可按需切换，成本可控。</p>
		</section>
	</div>

	<div class="mt-12 rounded-2xl border border-brand/40 bg-brand/10 p-8 text-center">
		<h2 class="text-2xl font-bold text-slate-100">准备好使用 AI API 了吗？</h2>
		<p class="mt-3 text-sm text-muted">免费试用 {BRAND_NAME}，1 元 = 1 刀，额度永不过期。</p>
		<a class="mt-6 inline-block rounded-md bg-brand px-6 py-3 text-sm font-semibold text-black hover:opacity-90" href="{base}/app/register">立即注册</a>
	</div>
</article>
