<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { customerApi } from '$lib/customer-api';
	import type { BillingSummary, CustomerMe } from '$lib/customer-types';
	import { fmtCost } from '$lib/format';

	interface Props {
		children: import('svelte').Snippet;
	}

	const navGroups = [
		{ label: '', items: [{ href: '/app/dashboard', label: '仪表盘' }] },
		{
			label: 'API',
			items: [
				{ href: '/app/keys', label: 'API 密钥' },
				{ href: '/app/usage', label: '使用记录' }
			]
		},
		{
			label: '账单',
			items: [
				{ href: '/app/billing', label: '充值' },
				{ href: '/app/orders', label: '我的订单' },
				{ href: '/app/balance-history', label: '额度记录' }
			]
		},
		{ label: '邀请', items: [{ href: '/app/referrals', label: '邀请奖励' }] },
		{ label: '', items: [{ href: '/app/settings', label: '个人设置' }] }
	];

	let { children }: Props = $props();
	let me = $state<CustomerMe | null>(null);
	let billing = $state<BillingSummary | null>(null);
	let error = $state('');
	let mobileOpen = $state(false);

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
						<span class="relative flex h-8 w-8 items-center justify-center rounded-md border border-brand/50 bg-black text-xs font-bold text-brand shadow-[0_0_28px_-12px_rgba(0,255,65,0.9)]">CD</span>
						<span>CDX Console</span>
					</a>
				</div>

				<div class="flex min-w-0 items-center gap-3">
					<a href="{base}/app/billing" class="hidden rounded-full border border-line bg-card px-3 py-1.5 font-mono text-sm hover:border-brand/50 sm:inline-flex">
						<span class="mr-1 text-brand">$</span>{billing ? fmtCost(billing.balance_usd).replace('$', '') : '0.00'}
					</a>
					<div class="hidden min-w-0 text-right text-xs text-faint sm:block">
						<div class="truncate text-slate-200">{me?.user.email ?? (error || 'loading session...')}</div>
						<div>{me?.user.status ?? '-'}</div>
					</div>
					<button class="h-9 rounded-md border border-line bg-card px-3 text-sm text-slate-200 hover:border-brand/50" onclick={logout}>退出</button>
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
										class={`block rounded-md px-3 py-2 text-sm transition-colors ${activeNav(item.href) ? 'bg-brand font-semibold text-black' : 'text-slate-300 hover:bg-white/[0.04] hover:text-white'}`}
									>
										{item.label}
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
					<aside class="absolute left-0 top-16 h-[calc(100vh-4rem)] w-72 overflow-y-auto border-r border-line bg-bg p-4">
						<nav class="space-y-5">
							{#each navGroups as group}
								<div>
									{#if group.label}
										<div class="mb-2 px-3 font-mono text-[11px] uppercase tracking-wider text-faint">{group.label}</div>
									{/if}
									{#each group.items as item}
										<a
											href="{base}{item.href}"
											onclick={() => (mobileOpen = false)}
											class={`mb-1 block rounded-md px-3 py-2 text-sm ${activeNav(item.href) ? 'bg-brand font-semibold text-black' : 'text-slate-300 hover:bg-white/[0.04]'}`}
										>
											{item.label}
										</a>
									{/each}
								</div>
							{/each}
						</nav>
					</aside>
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
