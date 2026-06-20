<script lang="ts">
	import { base } from '$app/paths';
	import Icon from '$lib/components/Icon.svelte';
</script>

<div class="mb-8">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">subscriptions</div>
	<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">我的订阅</h1>
	<p class="mt-2 text-sm text-muted">管理你的月卡订阅。</p>
</div>

<section class="rounded-lg border border-line bg-card/60 p-5">
	<h2 class="m-0 text-base font-semibold">当前订阅</h2>
	<div class="mt-5 flex flex-col items-center rounded-lg border border-dashed border-line bg-black/20 px-6 py-12 text-center">
		<Icon name="calendar" size={32} class="text-faint" />
		<div class="mt-4 text-lg font-semibold">暂无有效订阅</div>
		<p class="mt-2 text-sm text-faint">订阅月卡后即可享受每日固定额度，到期自动停止，不会继续扣费。</p>
		<a class="mt-5 inline-flex h-10 items-center rounded-md bg-brand px-4 text-sm font-semibold text-black hover:bg-brand/90" href={`${base}/app/billing`}>浏览套餐</a>
	</div>
</section>

<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
	<h2 class="m-0 text-base font-semibold">月卡说明</h2>
	<ul class="mt-4 space-y-2 text-sm text-muted">
		<li>· 每日固定额度。</li>
		<li>· UTC+8 每日刷新。</li>
		<li>· 未用完额度不结转。</li>
		<li>· 调用时优先消耗月卡额度。</li>
		<li>· 到期后自动停止。</li>
		<li>· 升级套餐时按剩余天数折算余额。</li>
	</ul>
</section>
