<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { ReferralSummary } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';

	let referral = $state<ReferralSummary | null>(null);
	let error = $state('');
	let loading = $state(false);
	let copied = $state('');

	$effect(() => {
		loadReferral();
	});

	async function loadReferral() {
		loading = true;
		error = '';
		try {
			referral = await customerApi<ReferralSummary>('/referrals');
		} catch (e: any) {
			error = e.message || 'failed to load referrals';
		} finally {
			loading = false;
		}
	}

	async function copy(text: string, id: string) {
		await navigator.clipboard.writeText(text);
		copied = id;
		setTimeout(() => {
			if (copied === id) copied = '';
		}, 1800);
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">referrals</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">邀请奖励</h1>
		<p class="mt-2 text-sm text-muted">受邀方注册后自动获得奖励；邀请方在受邀方首次付费后获得奖励。</p>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadReferral}>刷新</button>
</div>

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else if referral}
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">邀请码</div>
			<div class="mt-3 font-mono text-2xl font-bold text-brand">{referral.code}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">注册邀请</div>
			<div class="mt-3 font-mono text-2xl font-bold">{referral.signups}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">已付费邀请</div>
			<div class="mt-3 font-mono text-2xl font-bold">{referral.paid_invitees ?? 0}</div>
		</div>
		<div class="rounded-lg border border-line bg-card/70 p-5">
			<div class="text-sm text-faint">已获奖励</div>
			<div class="mt-3 font-mono text-2xl font-bold text-brand">{fmtCost(referral.credits_usd)}</div>
		</div>
	</div>

	<section class="mt-6 rounded-lg border border-brand/20 bg-brand/[0.04] p-5">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
			<div>
				<h2 class="m-0 text-base font-semibold">邀请链接</h2>
				<p class="mt-1 text-sm text-faint">复制后发给新用户，注册时会自动带入邀请码。</p>
			</div>
			<div class="flex gap-2">
				<button class="rounded-md border border-line bg-card px-3 py-2 text-sm hover:border-brand/50" onclick={() => copy(referral?.url ?? '', 'url')}>
					{copied === 'url' ? '已复制' : '复制链接'}
				</button>
				<button class="rounded-md border border-line bg-card px-3 py-2 text-sm hover:border-brand/50" onclick={() => copy(referral?.code ?? '', 'code')}>
					{copied === 'code' ? '已复制' : '复制码'}
				</button>
			</div>
		</div>
		<div class="mt-4 break-all rounded-md border border-line bg-black/35 p-3 font-mono text-sm text-brand">{referral.url || '-'}</div>
	</section>

	<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">奖励规则</h2>
		<div class="mt-4 grid gap-3 sm:grid-cols-2">
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="font-medium">受邀方</div>
				<p class="mt-1 text-sm text-faint">注册账号后自动获得新用户奖励。</p>
			</div>
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="font-medium">邀请方</div>
				<p class="mt-1 text-sm text-faint">受邀方完成首次支付后，自动获得邀请奖励。</p>
			</div>
		</div>
	</section>
{/if}
