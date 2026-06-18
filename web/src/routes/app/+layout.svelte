<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { customerApi } from '$lib/customer-api';
	import Logo from '$lib/components/Logo.svelte';
	import type { CustomerMe } from '$lib/customer-types';

	interface Props {
		children: import('svelte').Snippet;
	}

	const navItems = [
		{ href: '/app/dashboard', label: 'dashboard' },
		{ href: '/app/keys', label: 'keys' },
		{ href: '/app/billing', label: 'billing' },
		{ href: '/app/referrals', label: 'referrals' }
	];

	let { children }: Props = $props();
	let me = $state<CustomerMe | null>(null);
	let error = $state('');

	$effect(() => {
		if (showCustomerNav()) {
			loadMe();
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

	async function loadMe() {
		error = '';
		try {
			me = await customerApi<CustomerMe>('/me');
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
	<div class="customer-shell">
		<header class="top-glass">
			<div class="nav-inner">
				<div>
					<Logo href={`${base}/app/dashboard`} label="CDX Console" />
					<div class="sub">
						{#if me}
							{me.user.email} · {me.user.status}{me.user.plan ? ` / ${me.user.plan}` : ''}
						{:else if error}
							<span class="r">{error}</span>
						{:else}
							<span class="loading">loading session...</span>
						{/if}
					</div>
				</div>
				<nav class="nav-links" aria-label="Customer navigation">
					{#each navItems as item (item.href)}
						<a href="{base}{item.href}" class:active={activeNav(item.href)}>{item.label}</a>
					{/each}
					<button class="link" onclick={logout}>logout</button>
				</nav>
			</div>
		</header>
		<main class="shell-main">
			{@render children()}
		</main>
	</div>
{:else}
	{@render children()}
{/if}
