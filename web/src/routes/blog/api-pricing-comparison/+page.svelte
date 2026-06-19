<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<article class="mx-auto max-w-3xl px-5 py-14">
		<a class="font-mono text-sm text-brand hover:text-brand" href="{base}/blog">← 返回博客</a>
		<div class="mt-8 font-mono text-xs uppercase tracking-wider text-brand">pricing</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">2026 AI API 定价全对比</h1>
		<p class="mt-4 text-muted">
			公开价格通常只写输入和输出，但真实成本还取决于 cache、上下文长度、重试、工具调用和套餐折算。{BRAND_NAME} 的公开页按这些维度组织价格说明。
		</p>

		<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
			<section>
				<h2 class="text-2xl font-bold text-slate-100">四个计费维度</h2>
				<div class="mt-4 grid gap-3 sm:grid-cols-2">
					<div class="rounded-lg border border-line bg-card/70 p-4">
						<h3 class="text-base font-semibold text-slate-100">输入 token</h3>
						<p>用户消息、系统提示和工具上下文都会进入输入账单。</p>
					</div>
					<div class="rounded-lg border border-line bg-card/70 p-4">
						<h3 class="text-base font-semibold text-slate-100">输出 token</h3>
						<p>模型生成越长，输出成本越高，通常高于输入单价。</p>
					</div>
					<div class="rounded-lg border border-line bg-card/70 p-4">
						<h3 class="text-base font-semibold text-slate-100">Cache</h3>
						<p>长提示复用时，cache read/write 会改变实际单次成本。</p>
					</div>
					<div class="rounded-lg border border-line bg-card/70 p-4">
						<h3 class="text-base font-semibold text-slate-100">套餐折算</h3>
						<p>按量、月卡和企业预存会影响人民币付款和 USD 账本之间的关系。</p>
					</div>
				</div>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">为什么同一模型账单会不同</h2>
				<p>
					AI 编程任务通常包含大量仓库上下文、补丁内容和工具调用结果。即使用户只发出一句指令，真实请求也可能带上几万 token。比较 API 价格时必须看完整调用链，而不是只看模型标价。
				</p>
			</section>

			<section>
				<h2 class="text-2xl font-bold text-slate-100">{BRAND_NAME} 的展示口径</h2>
				<p>
					定价页同时展示 PAYGO、月卡、模型明细、VIP 和企业方案。当前可用能力以 Codex 中转为主，Claude 家族接入后沿用同一套餐表达；其他模型价格位用于目标商业页结构对齐。
				</p>
			</section>
		</div>
	</article>
</main>
