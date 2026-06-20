<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { BRAND_NAME } from '$lib/brand';
	import { customerApi } from '$lib/customer-api';
	import type { BillingSummary, CustomerMe } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';
	import Icon from '$lib/components/Icon.svelte';

	interface Props {
		children: import('svelte').Snippet;
	}

	const navGroups = [
		{ label: '', items: [{ href: '/app/dashboard', label: '仪表盘', icon: 'dashboard' }] },
		{
			label: 'API',
			items: [
				{ href: '/app/keys', label: 'API 密钥', icon: 'key' },
				{ href: '/app/key-test', label: 'Key 测试', icon: 'test' },
				{ href: '/app/images', label: 'AI 生图', icon: 'image', badge: 'NEW' },
				{ href: '/app/usage', label: '使用记录', icon: 'usage' }
			]
		},
		{
			label: '账单',
			items: [
				{ href: '/app/billing', label: '充值 / 订阅', icon: 'billing' },
				{ href: '/app/orders', label: '我的订单', icon: 'orders' },
				{ href: '/app/subscriptions', label: '我的订阅', icon: 'calendar' },
				{ href: '/app/balance-history', label: '额度记录', icon: 'history' }
			]
		},
		{ label: '', items: [{ href: '/app/redeem', label: '兑换码', icon: 'gift' }] },
		{
			label: '分销',
			items: [
				{ href: '/app/referrals', label: '分销中心', icon: 'referrals' },
				{ href: '/app/referrals/earnings', label: '佣金明细', icon: 'coins' }
			]
		},
		{ label: '', items: [{ href: '/app/settings', label: '个人设置', icon: 'settings' }] }
	];

	const accountMenuItems = [
		{ href: '/app/dashboard', label: '仪表盘' },
		{ href: '/app/keys', label: 'API 密钥' },
		{ href: '/app/billing', label: '充值' },
		{ href: '/app/settings', label: '个人设置' }
	];

	let { children }: Props = $props();
	let me = $state<CustomerMe | null>(null);
	let billing = $state<BillingSummary | null>(null);
	let error = $state('');
	let mobileOpen = $state(false);
	let accountOpen = $state(false);

	$effect(() => {
		if (showCustomerNav()) {
			loadShell();
		}
	});

	function showCustomerNav() {
		const path = $page.url.pathname;
		return path !== `${base}/app/login` && path !== `${base}/app/register`;
	}

	function activeNav(href: string) {
		const path = $page.url.pathname;
		const full = `${base}${href}`;
		return path === full || path.startsWith(full + '/');
	}

	async function loadShell() {
		error = '';
		try {
			const [meResult, billingResult] = await Promise.all([
				customerApi<CustomerMe>('/me'),
				customerApi<BillingSummary>('/billing/summary').catch(() => null)
			]);
			me = meResult;
			billing = billingResult;
		} catch (e: any) {
			error = e.message || 'failed to load session';
		}
	}

	async function logout() {
		try {
			await customerApi('/auth/logout', { method: 'POST' });
		} finally {
			window.location.href = `${base}/app/login`;
		}
	}
</script>

{#if showCustomerNav()}
	<div class="min-h-screen bg-bg text-slate-100">
		<header class="sticky top-0 z-50 border-b border-line bg-bg/85 backdrop-blur-md">
			<div class="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6">
				<div class="flex items-center gap-3">
					<button
						class="inline-flex h-9 w-9 items-center justify-center rounded-md border border-line bg-card text-slate-200 lg:hidden"
						aria-label="菜单"
						onclick={() => (mobileOpen = !mobileOpen)}
					>
						☰
					</button>
					<a href="{base}/app/dashboard" class="flex items-center gap-3 font-semibold tracking-tight">
						<span class="brand-mark" aria-hidden="true"></span>
						<span class="text-xl">{BRAND_NAME}</span>
					</a>
				</div>

				<div class="flex min-w-0 items-center gap-3">
					<button class="hidden h-8 items-center gap-1 rounded-md px-2 text-sm text-faint hover:bg-white/[0.04] hover:text-white sm:inline-flex" type="button"><Icon name="globe" size={15} />ZH</button>
					<a href="{base}/app/billing" class="hidden items-center rounded-full bg-brand px-3 py-1.5 font-mono text-sm font-semibold text-black hover:bg-[#35ff67] sm:inline-flex">
						<span class="mr-1">$</span>{billing ? fmtCost(billing.balance_usd).replace('$', '') : '0.00'}
					</a>
					<div class="hidden min-w-0 text-right text-xs text-faint sm:block">
						<div class="truncate text-slate-200">{me?.user.email ?? 'loading session...'}</div>
						<div>{me?.user.status ?? '-'}</div>
					</div>
						<div class="relative">
							<button
								class="flex h-9 w-9 items-center justify-center rounded-full border border-line bg-card text-sm font-semibold text-slate-200 hover:border-brand/50"
								onclick={() => (accountOpen = !accountOpen)}
								aria-label="账户菜单"
								aria-expanded={accountOpen}
								type="button"
							>
								{me?.user.email?.slice(0, 1).toUpperCase() ?? 'T'}
							</button>
							{#if accountOpen}
								<div class="absolute right-0 top-11 z-50 w-56 overflow-hidden rounded-lg border border-line bg-card shadow-2xl shadow-black/40">
									<div class="border-b border-line px-4 py-3 text-xs text-faint">
										<div class="truncate font-mono text-slate-200">{me?.user.email ?? 'loading session...'}</div>
										<div class="mt-1">{me?.user.status ?? '-'}</div>
									</div>
									<div class="p-1">
										{#each accountMenuItems as item}
											<a
												href="{base}{item.href}"
												onclick={() => (accountOpen = false)}
												class="block rounded-md px-3 py-2 text-sm text-slate-300 hover:bg-white/[0.04] hover:text-white"
											>
												{item.label}
											</a>
										{/each}
										<button class="mt-1 w-full rounded-md px-3 py-2 text-left text-sm text-red-300 hover:bg-red-500/10" type="button" onclick={logout}>退出登录</button>
									</div>
								</div>
							{/if}
						</div>
					</div>
			</div>
		</header>

		<div class="mx-auto flex max-w-7xl">
			<aside class="sticky top-16 hidden h-[calc(100vh-4rem)] w-60 shrink-0 overflow-y-auto border-r border-line p-4 lg:block">
				<nav class="space-y-6" aria-label="Customer navigation">
					{#each navGroups as group}
						<div>
							{#if group.label}
								<div class="mb-2 px-3 font-mono text-[11px] uppercase tracking-wider text-faint">{group.label}</div>
							{/if}
							<div class="space-y-1">
								{#each group.items as item}
									<a
										href="{base}{item.href}"
										class={`flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors ${activeNav(item.href) ? 'bg-brand font-semibold text-black' : 'text-slate-300 hover:bg-white/[0.04] hover:text-white'}`}
									>
										<Icon name={item.icon} size={16} />
										<span class="flex-1">{item.label}</span>
										{#if item.badge}
											<span class={`rounded px-1.5 py-0.5 font-mono text-[10px] ${activeNav(item.href) ? 'bg-black/10 text-black' : 'bg-brand/10 text-brand'}`}>{item.badge}</span>
										{/if}
									</a>
								{/each}
							</div>
						</div>
					{/each}
				</nav>
			</aside>

			{#if mobileOpen}
				<div class="fixed inset-0 z-40 lg:hidden">
					<button class="absolute inset-0 bg-black/60" aria-label="关闭菜单" onclick={() => (mobileOpen = false)}></button>
					<div class="absolute left-0 top-16 h-[calc(100vh-4rem)] w-72 overflow-y-auto border-r border-line bg-bg p-4" role="dialog" aria-modal="true" aria-labelledby="mobile-navigation-title">
						<div class="mb-4 flex items-center justify-between gap-3">
							<div>
								<div class="font-mono text-[11px] uppercase tracking-wider text-brand">Navigation dialog</div>
								<h2 id="mobile-navigation-title" class="m-0 mt-1 text-base font-semibold">导航</h2>
							</div>
							<button class="h-8 w-8 rounded-md border border-line bg-card p-0 text-sm text-faint hover:text-white" type="button" aria-label="关闭菜单" onclick={() => (mobileOpen = false)}>×</button>
						</div>
						<nav class="space-y-5" aria-label="Mobile customer navigation">
							{#each navGroups as group}
								<div>
									{#if group.label}
										<div class="mb-2 px-3 font-mono text-[11px] uppercase tracking-wider text-faint">{group.label}</div>
									{/if}
									{#each group.items as item}
										<a
											href="{base}{item.href}"
											onclick={() => (mobileOpen = false)}
											class={`mb-1 flex items-center gap-2.5 rounded-md px-3 py-2 text-sm ${activeNav(item.href) ? 'bg-brand font-semibold text-black' : 'text-slate-300 hover:bg-white/[0.04]'}`}
										>
											<Icon name={item.icon} size={16} />
											<span class="flex-1">{item.label}</span>
											{#if item.badge}
												<span class={`rounded px-1.5 py-0.5 font-mono text-[10px] ${activeNav(item.href) ? 'bg-black/10 text-black' : 'bg-brand/10 text-brand'}`}>{item.badge}</span>
											{/if}
										</a>
									{/each}
								</div>
							{/each}
						</nav>
					</div>
				</div>
			{/if}

			<main class="min-w-0 flex-1 px-4 py-8 sm:px-8">
				<div class="mx-auto max-w-6xl">
					{@render children()}
				</div>
			</main>
		</div>
	</div>
{:else}
	{@render children()}
{/if}
