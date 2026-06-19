<script lang="ts">
	import '$lib/global.css';
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import Logo from '$lib/components/Logo.svelte';
	import type { CustomerMe, CustomerUser } from '$lib/customer-types';

	interface Props {
		children: import('svelte').Snippet;
	}

	const consoleNavGroups = [
		{ label: '总览', items: [{ href: '/console/dashboard', label: '控制台' }] },
		{
			label: '账号池',
			items: [
				{ href: '/console/accounts', label: '上游账号' },
				{ href: '/console/capacity', label: '容量观测' },
				{ href: '/console/migrations', label: '迁移工具' }
			]
		},
		{ label: '客户', items: [{ href: '/console/users', label: '用户与密钥' }] },
		{
			label: '计费',
			items: [
				{ href: '/console/billing', label: '充值计费' },
				{ href: '/console/settings', label: '系统设置' }
			]
		},
		{ label: '观测', items: [{ href: '/console/activity', label: '运行事件' }] }
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
	let currentUser = $state<CustomerUser | null>(null);
	let consoleHref = $state('/app/dashboard');

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

	onMount(() => {
		void loadCurrentUser();
	});

	async function loadCurrentUser() {
		try {
			const res = await fetch('/api/me', {
				credentials: 'same-origin',
				headers: { Accept: 'application/json' }
			});
			if (!res.ok) return;
			const payload = (await res.json()) as CustomerMe;
			currentUser = payload.user;
			consoleHref = payload.redirect_to ?? (payload.user?.role === 'admin' ? '/console/dashboard' : '/app/dashboard');
		} catch {
			currentUser = null;
		}
	}
</script>

{#if showConsoleShell()}
	<div class="admin-shell">
		<header class="top-glass">
			<div class="nav-inner">
				<Logo href={`${base}/console/dashboard`} label="CDX Console" />
				<nav class="nav-links" aria-label="管理后台导航">
					<a href="{base}/console/dashboard" class:active={activeNav('/console/dashboard')}>管理后台</a>
					<a href="{base}/app/dashboard">用户端</a>
					<a href="{base}/">官网</a>
				</nav>
			</div>
		</header>
		<main class="admin-grid">
			<aside class="admin-side">
				<nav class="admin-menu" aria-label="控制台导航">
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
					{#if currentUser}
						<a class="rounded-md bg-brand px-3 py-2 text-sm font-semibold text-black" href="{base}{consoleHref}">控制台</a>
					{:else}
						<a class="rounded-md border border-line px-3 py-2 text-sm hover:border-brand/50" href="{base}/app/login">登录</a>
						<a class="rounded-md bg-brand px-3 py-2 text-sm font-semibold text-black" href="{base}/app/register">开始使用</a>
					{/if}
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
