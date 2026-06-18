<script lang="ts">
	import '$lib/global.css';
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import Logo from '$lib/components/Logo.svelte';

	interface Props {
		children: import('svelte').Snippet;
	}

	const navItems = [
		{ href: '/dashboard', label: 'ops' },
		{ href: '/accounts', label: 'accounts' },
		{ href: '/users', label: 'users' },
		{ href: '/activity', label: 'activity' },
		{ href: '/admin-billing', label: 'billing' },
		{ href: '/migrations', label: 'migration' }
	];

	let { children }: Props = $props();

	function activeNav(href: string) {
		const path = $page.url.pathname;
		const full = `${base}${href}`;
		return path === full || path.startsWith(full + '/');
	}

	function showAdminNav() {
		const path = $page.url.pathname;
		const adminPrefixes = ['/dashboard', '/accounts', '/users', '/activity', '/admin-billing', '/migrations', '/add-account', '/cells'];
		return adminPrefixes.some((prefix) => path === `${base}${prefix}` || path.startsWith(`${base}${prefix}/`));
	}
</script>

{#if showAdminNav()}
	<div class="admin-shell">
		<header class="top-glass">
			<div class="nav-inner">
				<Logo href={`${base}/dashboard`} label="CDX Admin" />
				<nav class="nav-links" aria-label="Admin navigation">
					<a href="{base}/dashboard" class:active={activeNav('/dashboard')}>control</a>
					<a href="{base}/app/dashboard">customer app</a>
					<a href="{base}/">site</a>
				</nav>
			</div>
		</header>
		<main class="admin-grid">
			<aside class="admin-side">
				<div class="eyebrow">operations</div>
				<nav class="nav-links" aria-label="Operations navigation">
					{#each navItems as item (item.href)}
						<a href="{base}{item.href}" class:active={activeNav(item.href)}>{item.label}</a>
					{/each}
				</nav>
			</aside>
			<section class="admin-content">
				{@render children()}
			</section>
		</main>
	</div>
{:else}
	{@render children()}
{/if}
