<script lang="ts">
	import { base } from '$app/paths';
	import { onMount } from 'svelte';

	const packs = [20, 50, 100, 500, 1000, 5000];
	interface ModelPrice {
		model: string;
		input_usd_per_million: number;
		output_usd_per_million: number;
		cache_read_usd_per_million: number;
		cache_create_usd_per_million: number;
	}

	let modelPrices = $state<ModelPrice[]>([]);

	onMount(async () => {
		const res = await fetch('/api/public/model-prices', { headers: { Accept: 'application/json' } }).catch(() => null);
		if (res?.ok) {
			modelPrices = await res.json();
		}
	});

	function priceText(price: ModelPrice) {
		return `输入 $${price.input_usd_per_million}/M，输出 $${price.output_usd_per_million}/M`;
	}
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-6xl px-5 py-16 text-center">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">pricing</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">透明按量计费</h1>
		<p class="mt-4 text-muted">人民币充值，USD 额度展示；按实际 token 消耗扣费。</p>
	</div>
</section>

<section class="mx-auto max-w-6xl px-5 py-14">
	<h2 class="text-2xl font-bold">充值套餐</h2>
	<p class="mt-2 text-sm text-faint">套餐展示 USD 额度；实际人民币应付金额以下单页为准。</p>
	<div class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each packs as amount}
			<div class="rounded-lg border border-line bg-card/60 p-6">
				<h3 class="text-lg font-semibold">充值额度</h3>
				<div class="mt-4 text-3xl font-bold text-brand">${amount.toLocaleString()}</div>
				<p class="mt-2 text-sm text-faint">额度永不过期，按 token 使用扣除。</p>
				<a class="mt-6 inline-flex h-10 items-center rounded-md bg-brand px-4 text-sm font-semibold text-black" href="{base}/app/billing">去充值</a>
			</div>
		{/each}
	</div>
</section>

<section class="border-y border-line bg-surface/30">
	<div class="mx-auto max-w-6xl px-5 py-14">
		<h2 class="text-2xl font-bold">模型计费</h2>
		<p class="mt-2 text-sm text-faint">模型单价由后台 `model_prices` 配置，消费按输入/输出/cache token 计算。</p>
		<div class="mt-6 overflow-x-auto rounded-lg border border-line bg-card/60">
			<table class="w-full min-w-[640px] text-left text-sm">
				<thead class="bg-white/[0.03] font-mono text-xs text-faint">
					<tr><th class="px-5 py-3 font-medium">模型</th><th class="px-5 py-3 font-medium">价格</th><th class="px-5 py-3 font-medium">说明</th></tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each modelPrices as price}
						<tr>
							<td class="px-5 py-3 font-mono text-brand">{price.model}</td>
							<td class="px-5 py-3">{priceText(price)}</td>
							<td class="px-5 py-3 text-muted">cache read ${price.cache_read_usd_per_million}/M，cache create ${price.cache_create_usd_per_million}/M</td>
						</tr>
					{:else}
						<tr><td class="px-5 py-3 text-muted" colspan="3">正在加载模型价格...</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
</section>
