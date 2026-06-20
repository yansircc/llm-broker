<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';

	const commissionTiers = [
		['15%', '基础合伙人', '注册即可，适合教程作者、工具站和个人社群推广。'],
		['25%', '进阶合伙人', '付费下级 ≥ 50 人或累计佣金 ≥ ¥1,000，渠道持续带来有效注册。'],
		['40%', '核心合伙人', '付费下级 ≥ 260 人或累计佣金 ≥ ¥5,000，适合服务商与技术社区。']
	];

	const steps = [
		['注册并获取链接', '创建账号后进入分销中心，复制专属推广链接。'],
		['发布教程或推荐', '把链接放到文章、视频、社群、客户交付文档或工具清单里。'],
		['客户永久绑定', '客户通过你的链接注册后会绑定到你的渠道，后续充值继续按规则统计佣金。'],
		['获得佣金结算', '客户充值后按等级实时统计佣金，提现无手续费。']
	];

	const reasons = [
		['开发者需求明确', 'AI API 中转是高频、可复购的开发基础设施，不依赖一次性冲动购买。'],
		['无需囤货交付', '合伙人不需要管理上游账号、支付通道或运行时容量。'],
		['客户永久绑定', '客户通过你的链接注册后长期绑定，后续充值持续计佣。'],
		['佣金实时结算', '佣金实时入账，提现无手续费，满足条件自动升级等级。']
	];
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-28 text-center">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">affiliate partner</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">分销合伙人计划</h1>
		<p class="mx-auto mt-4 max-w-3xl text-muted">
			推广 {BRAND_NAME}，把一个 Key 调用全部主流模型的能力推荐给开发者、技术团队和 AI 工具用户。客户通过你的链接注册并充值后，按等级获得 15%-40% 持续佣金，客户永久绑定。
		</p>
		<div class="mt-16 flex flex-wrap justify-center gap-3">
			<a class="rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/app/register">开始推广</a>
				<a class="rounded-md border border-line px-5 py-3 text-sm hover:border-brand/50" href="{base}/contact">加微信获取教程</a>
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-28">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">commission</div>
	<h2 class="mt-2 text-2xl font-bold">三级佣金</h2>
	<p class="mt-2 text-sm text-faint">分享链接即可赚取持续佣金，满足条件自动升级等级。</p>
			<div class="mt-16 grid gap-6 md:grid-cols-3">
		{#each commissionTiers as tier (tier[0])}
			<article class="rounded-lg border border-line bg-card/60 p-6">
				<div class="font-mono text-4xl font-bold text-brand">{tier[0]}</div>
				<h3 class="mt-4 text-lg font-semibold">{tier[1]}</h3>
				<p class="mt-2 text-sm text-faint">{tier[2]}</p>
			</article>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">three steps</div>
		<h2 class="mt-2 text-2xl font-bold">三步开始赚钱</h2>
		<div class="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
			{#each steps as step, index (step[0])}
				<article class="rounded-lg border border-line bg-card/60 p-6">
					<div class="font-mono text-sm text-brand">STEP 0{index + 1}</div>
					<h3 class="mt-3 text-lg font-semibold">{step[0]}</h3>
					<p class="mt-2 text-sm text-faint">{step[1]}</p>
				</article>
			{/each}
		</div>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-28">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">why us</div>
	<h2 class="mt-2 text-2xl font-bold">为什么选择 {BRAND_NAME}</h2>
	<div class="mt-16 grid gap-6 md:grid-cols-2">
		{#each reasons as reason (reason[0])}
			<article class="rounded-lg border border-line bg-card/60 p-6">
				<h3 class="text-lg font-semibold">{reason[0]}</h3>
				<p class="mt-2 text-sm text-faint">{reason[1]}</p>
			</article>
		{/each}
	</div>
</section>

<section class="border-t border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-28">
		<div class="rounded-lg border border-brand/40 bg-card/70 p-6 text-center">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">wechat tutorial</div>
			<h2 class="mt-3 text-2xl font-bold">获取微信教程和推广素材</h2>
			<p class="mx-auto mt-3 max-w-2xl text-sm text-muted">
				联系支持团队说明你的渠道类型，我们会提供微信沟通入口、配置教程、推广素材和结算说明。邮箱：<span class="font-mono text-brand">{BRAND_SUPPORT_EMAIL}</span>
			</p>
			<a class="mt-6 inline-flex rounded-md bg-brand px-5 py-3 text-sm font-semibold text-black" href="{base}/contact">联系分销合作</a>
		</div>
	</div>
</section>
