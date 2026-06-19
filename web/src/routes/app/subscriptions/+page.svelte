<script lang="ts">
	const plans = [
		['入门版月卡', '¥99/月', '$10/天额度', '轻量体验和低频编码'],
		['轻量版月卡', '¥199/月', '$20/天额度', '个人日常开发'],
		['标准版月卡', '¥499/月', '$50/天额度', '重度代码编写、长文档分析'],
		['高级版月卡', '¥999/月', '$100/天额度', '高频任务和多工具并用'],
		['团队版月卡', '¥1,888/月', '$200/天额度', '极高频用户、小型工作室'],
		['商业版月卡', '¥3,888/月', '$500/天额度', '团队稳定消耗'],
		['企业版月卡', '联系销售', '专属额度与并发', '企业账号和专属支持']
	];
</script>

<div class="mb-8">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">subscriptions</div>
	<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">我的订阅</h1>
	<p class="mt-2 text-sm text-muted">查看月卡状态、每日额度和刷新规则。</p>
</div>

<section class="rounded-lg border border-line bg-card/60 p-5">
	<h2 class="m-0 text-base font-semibold">当前订阅</h2>
	<div class="mt-5 rounded-lg border border-dashed border-line bg-black/20 px-6 py-12 text-center">
		<div class="text-lg font-semibold">暂无有效订阅</div>
		<p class="mt-2 text-sm text-faint">当前本项目只有按量充值后端，月卡订阅能力待补。到期后自动停止，不会继续扣费。</p>
	</div>
</section>

<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
	<h2 class="m-0 text-base font-semibold">月卡说明</h2>
	<ul class="mt-4 grid gap-2 text-sm text-muted md:grid-cols-2">
		<li class="rounded-md border border-line bg-black/20 p-3">每日固定额度。</li>
		<li class="rounded-md border border-line bg-black/20 p-3">UTC+8 每日刷新。</li>
		<li class="rounded-md border border-line bg-black/20 p-3">未用完额度不结转。</li>
		<li class="rounded-md border border-line bg-black/20 p-3">调用时优先消耗月卡额度。</li>
		<li class="rounded-md border border-line bg-black/20 p-3">到期后自动停止。</li>
		<li class="rounded-md border border-line bg-black/20 p-3">升级套餐时按剩余天数折算余额。</li>
	</ul>
	<div class="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
		{#each plans as plan}
			<div class="rounded-lg border border-line bg-black/20 p-5">
				<h3 class="text-base font-semibold">{plan[0]}</h3>
				<div class="mt-3 text-2xl font-bold">{plan[1]}</div>
				<div class="mt-1 font-mono text-sm text-brand">{plan[2]}</div>
				<p class="mt-3 text-sm text-faint">{plan[3]}</p>
			</div>
		{/each}
	</div>
</section>

<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
	<h2 class="m-0 text-base font-semibold">升级套餐</h2>
	<p class="mt-2 text-sm text-faint">可视化入口已保留；订阅购买、升级、续费和退回余额需要后端订阅结算模型补齐后启用。</p>
</section>
