<script lang="ts">
	import '$lib/global.css';
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import Logo from '$lib/components/Logo.svelte';

	interface Props {
		children: import('svelte').Snippet;
	}

	const consoleNavGroups = [
		{ label: 'Overview', items: [{ href: '/console/dashboard', label: 'Dashboard' }] },
		{
			label: 'Accounts',
			items: [
				{ href: '/console/accounts', label: 'Account Pool' },
				{ href: '/console/migrations', label: 'Migration' }
			]
		},
		{ label: 'Customers', items: [{ href: '/console/users', label: 'Users' }] },
		{ label: 'Billing', items: [{ href: '/console/billing', label: 'Billing' }] },
		{ label: 'Activity', items: [{ href: '/console/activity', label: 'Events' }] }
	];
	const publicNavItems = [
		{ href: '/pricing', label: '定价' },
		{ href: '/models', label: '模型' },
		{ href: '/docs', label: '文档' },
		{ href: '/contact', label: '联系' }
	];
	const publicPaths = ['/', '/pricing', '/models', '/contact', '/privacy-policy', '/terms-of-service', '/acceptable-use-policy'];
	const publicPrefixes = ['/docs'];

	let { children }: Props = $props();

	function activeNav(href: string) {
		const path = $page.url.pathname;
		const full = `${base}${href}`;
		return path === full || path.startsWith(full + '/');
	}

	function activePublicNav(href: string) {
		return activeNav(href);
	}

	function matchesRoute(route: string) {
		const path = $page.url.pathname;
		const full = `${base}${route}`;
		return path === full;
	}

	function matchesPrefix(prefix: string) {
		const path = $page.url.pathname;
		const full = `${base}${prefix}`;
		return path === full || path.startsWith(full + '/');
	}

	function showConsoleShell() {
		return matchesPrefix('/console');
	}

	function showPublicShell() {
		return publicPaths.some(matchesRoute) || publicPrefixes.some(matchesPrefix);
	}
</script>

{#if showConsoleShell()}
	<div class="admin-shell">
		<header class="top-glass">
			<div class="nav-inner">
				<Logo href={`${base}/console/dashboard`} label="CDX Console" />
				<nav class="nav-links" aria-label="Admin navigation">
					<a href="{base}/console/dashboard" class:active={activeNav('/console/dashboard')}>control</a>
					<a href="{base}/app/dashboard">customer app</a>
					<a href="{base}/">site</a>
				</nav>
			</div>
		</header>
		<main class="admin-grid">
			<aside class="admin-side">
				<nav class="admin-menu" aria-label="Console navigation">
					{#each consoleNavGroups as group (group.label)}
						<div class="admin-menu-group">
							<div class="admin-menu-label">{group.label}</div>
							{#each group.items as item (item.href)}
								<a href="{base}{item.href}" class:active={activeNav(item.href)}>{item.label}</a>
							{/each}
						</div>
					{/each}
				</nav>
			</aside>
			<section class="admin-content">
				{@render children()}
			</section>
		</main>
	</div>
{:else if showPublicShell()}
	<div class="flex min-h-screen flex-col bg-bg text-slate-100">
		<header class="sticky top-0 z-50 border-b border-line bg-bg/85 backdrop-blur-md">
			<div class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
				<a href="{base}/" class="flex items-center gap-3 font-semibold">
					<span class="flex h-8 w-8 items-center justify-center rounded-md border border-brand/50 bg-black text-xs font-bold text-brand">CD</span>
					<span>CDX</span>
				</a>
				<nav class="hidden items-center gap-5 text-sm text-slate-300 sm:flex" aria-label="Site navigation">
					{#each publicNavItems as item (item.href)}
						<a class={activePublicNav(item.href) ? 'text-brand' : 'hover:text-brand'} href="{base}{item.href}">{item.label}</a>
					{/each}
				</nav>
				<div class="flex items-center gap-2">
					<a class="rounded-md border border-line px-3 py-2 text-sm hover:border-brand/50" href="{base}/app/login">登录</a>
					<a class="rounded-md bg-brand px-3 py-2 text-sm font-semibold text-black" href="{base}/app/register">开始使用</a>
				</div>
			</div>
			<nav class="mx-auto flex max-w-6xl gap-4 overflow-x-auto px-5 pb-3 text-sm text-slate-300 sm:hidden" aria-label="Site navigation mobile">
				{#each publicNavItems as item (item.href)}
					<a class={activePublicNav(item.href) ? 'text-brand' : 'hover:text-brand'} href="{base}{item.href}">{item.label}</a>
				{/each}
			</nav>
		</header>
		<main class="flex-1">
			{@render children()}
		</main>
		<footer class="border-t border-line">
			<div class="mx-auto flex max-w-6xl flex-col gap-3 px-5 py-8 text-sm text-faint sm:flex-row sm:items-center sm:justify-between">
				<span>CDX relay</span>
				<div class="flex flex-wrap gap-4">
					<a class="hover:text-brand" href="{base}/terms-of-service">服务条款</a>
					<a class="hover:text-brand" href="{base}/privacy-policy">隐私政策</a>
					<a class="hover:text-brand" href="{base}/acceptable-use-policy">使用政策</a>
				</div>
			</div>
		</footer>
	</div>
{:else}
	{@render children()}
{/if}
