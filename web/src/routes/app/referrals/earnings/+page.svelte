<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
</script>

<div class="mb-8">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">affiliate earnings</div>
	<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">佣金明细</h1>
	<p class="mt-2 text-sm text-muted">查看你的推广佣金收入和提现记录。</p>
</div>

<section class="rounded-lg border border-line bg-card/60 p-5">
	<div class="flex flex-col items-center px-6 py-16 text-center">
		<Icon name="coins" size={32} class="text-faint" />
		<div class="mt-4 text-lg font-semibold">暂无佣金记录</div>
		<p class="mt-2 text-sm text-faint">通过你的推广链接邀请用户注册并消费后，佣金将显示在这里。</p>
	</div>
</section>
