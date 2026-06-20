<script lang="ts">
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL, BRAND_SUPPORT_WECHAT } from '$lib/brand';
	import { customerApi } from '$lib/customer-api';
	import type { ReferralSummary } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';

	const showWechat = BRAND_SUPPORT_WECHAT && BRAND_SUPPORT_WECHAT !== '待配置';

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
		<div class="font-mono text-xs uppercase tracking-wider text-brand">affiliate</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">分销中心</h1>
		<p class="mt-2 text-sm text-muted">推广 {BRAND_NAME}，邀请注册和首充后获得奖励。</p>
		<p class="mt-2 text-xs text-faint">提现和等级调整请联系 {BRAND_SUPPORT_EMAIL}。</p>
	</div>
	<div class="flex gap-2">
		<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" type="button" disabled>申请提现</button>
		<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadReferral}>刷新</button>
	</div>
</div>

{#if showWechat}
	<section class="mb-6 rounded-lg border border-brand/30 bg-brand/[0.06] p-5">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<h2 class="m-0 text-base font-semibold">加管理员微信，领取专属推广支持</h2>
				<p class="mt-1 text-sm text-muted">微信号 <span class="font-mono text-brand">{BRAND_SUPPORT_WECHAT}</span></p>
			</div>
			<button class="inline-flex items-center gap-2 rounded-md border border-line bg-card px-4 py-2 text-sm hover:border-brand/50" type="button" onclick={() => copy(BRAND_SUPPORT_WECHAT, 'wechat')}>
				<Icon name="copy" size={16} />
				{copied === 'wechat' ? '已复制' : '复制微信号'}
			</button>
		</div>
	</section>
{/if}

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else if referral}
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<div class="relative rounded-lg border border-line bg-card/70 p-5">
			<Icon name="coins" size={18} class="absolute right-4 top-4 text-brand/40" />
			<div class="text-sm text-faint">累计佣金</div>
			<div class="mt-3 font-mono text-2xl font-bold text-brand">{fmtCost(referral.credits_usd)}</div>
		</div>
		<div class="relative rounded-lg border border-line bg-card/70 p-5">
			<Icon name="referrals" size={18} class="absolute right-4 top-4 text-brand/40" />
			<div class="text-sm text-faint">我的客户</div>
			<div class="mt-3 font-mono text-2xl font-bold">{referral.signups}</div>
		</div>
		<div class="relative rounded-lg border border-line bg-card/70 p-5">
			<Icon name="arrow-up-right" size={18} class="absolute right-4 top-4 text-brand/40" />
			<div class="text-sm text-faint">合伙人等级</div>
			<div class="mt-3 text-2xl font-bold">普通</div>
		</div>
		<div class="relative rounded-lg border border-line bg-card/70 p-5">
			<Icon name="gift" size={18} class="absolute right-4 top-4 text-brand/40" />
			<div class="text-sm text-faint">申请提现</div>
			<div class="mt-3 text-2xl font-bold text-faint">待开通</div>
		</div>
	</div>

	<section class="mt-6 rounded-lg border border-brand/20 bg-brand/[0.04] p-5">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
			<div>
				<h2 class="m-0 text-base font-semibold">推广链接</h2>
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
		<div class="mt-4 grid gap-3 sm:grid-cols-[1fr_180px]">
			<div class="break-all rounded-md border border-line bg-black/35 p-3 font-mono text-sm text-brand">{referral.url || '-'}</div>
			<div class="rounded-md border border-line bg-black/35 p-3">
				<div class="text-xs text-faint">邀请码</div>
				<div class="mt-1 font-mono text-lg text-brand">{referral.code}</div>
			</div>
		</div>
	</section>

	<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<h2 class="m-0 text-base font-semibold">佣金等级</h2>
			<button class="rounded-md border border-line bg-black/20 px-3 py-2 text-sm text-faint" type="button" disabled>提现</button>
		</div>
		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="rounded-md border border-brand/40 bg-brand/[0.06] p-4">
				<div class="flex items-center justify-between">
					<div class="font-medium">普通合伙人</div>
					<span class="rounded bg-brand/15 px-2 py-0.5 text-xs font-medium text-brand">当前等级</span>
				</div>
				<p class="mt-1 text-sm text-faint">15% 基础佣金等级。</p>
				<p class="mt-2 text-xs text-faint">升级条件：注册即可</p>
			</div>
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="font-medium">高级合伙人</div>
				<p class="mt-1 text-sm text-faint">25% 更高佣金等级，适合持续推广。</p>
				<p class="mt-2 text-xs text-faint">升级条件：付费下级 ≥ 50 人或累计佣金 ≥ ¥1,000</p>
			</div>
			<div class="rounded-md border border-line bg-black/20 p-4">
				<div class="font-medium">超级合伙人</div>
				<p class="mt-1 text-sm text-faint">40% 团队规模化分销等级。</p>
				<p class="mt-2 text-xs text-faint">升级条件：付费下级 ≥ 260 人或累计佣金 ≥ ¥5,000</p>
			</div>
		</div>
	</section>

	<section class="mt-6 rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">我的客户</h2>
		<div class="mt-5 overflow-x-auto">
			<table class="w-full min-w-[680px] text-left text-sm">
				<thead class="bg-white/[0.03] font-mono text-xs text-faint">
					<tr><th class="px-5 py-3 font-medium">用户</th><th class="px-5 py-3 font-medium">邮箱</th><th class="px-5 py-3 font-medium">注册时间</th><th class="px-5 py-3 font-medium">累计充值</th></tr>
				</thead>
				<tbody>
					<tr><td class="px-5 py-10 text-center text-faint" colspan="4">客户明细后端待补</td></tr>
				</tbody>
			</table>
		</div>
	</section>
{/if}
