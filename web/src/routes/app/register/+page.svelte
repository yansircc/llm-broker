<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { customerApi } from '$lib/customer-api';
	import Logo from '$lib/components/Logo.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';

	let email = $state('');
	let password = $state('');
	let name = $state('');
	let referralCode = $state('');
	let referralInitialized = $state(false);
	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		if (referralInitialized) return;
		referralCode = $page.url.searchParams.get('ref') ?? $page.url.searchParams.get('code') ?? '';
		referralInitialized = true;
	});

	async function register(e: Event) {
		e.preventDefault();
		if (!email.trim() || !password) return;
		loading = true;
		error = '';
		try {
			const body: Record<string, string | undefined> = {
				email: email.trim(),
				password,
				name: name.trim() || undefined
			};
			if (referralCode.trim()) {
				body.referral_code = referralCode.trim();
			}
			await customerApi('/auth/register', {
				method: 'POST',
				body: JSON.stringify(body)
			});
			window.location.href = `${base}/app/dashboard`;
		} catch (err: any) {
			error = err.message || 'registration failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="auth-page">
	<div class="auth-grid">
		<section class="auth-intro">
			<div>
				<Logo href={`${base}/`} label="CDX" />
				<div class="eyebrow auth-eyebrow">prepaid relay</div>
				<h1>Create a CDX account.</h1>
				<p class="lede">Recharge RMB, receive USD-denominated credits, and consume them by OpenAI token usage.</p>
			</div>
			<TerminalPanel title="account bootstrap" lines={['signup: customer', 'invitee credit: on registration', 'inviter credit: after paid order']} />
		</section>

		<section class="auth-card">
			<div class="page-header compact">
				<div>
					<div class="eyebrow">register</div>
					<h2>Customer Registration</h2>
				</div>
			</div>
			<form onsubmit={register}>
				<label for="name">name</label>
				<input id="name" type="text" autocomplete="name" bind:value={name}>
				<label for="email">email</label>
				<input id="email" type="text" autocomplete="email" bind:value={email}>
				<label for="password">password</label>
				<input id="password" type="password" autocomplete="new-password" bind:value={password}>
				<label for="referral">referral code</label>
				<input id="referral" type="text" autocomplete="off" bind:value={referralCode}>
				{#if error}<p class="error-msg">{error}</p>{/if}
				<button class="primary-btn" type="submit" disabled={loading || !email.trim() || !password}>
					{loading ? 'Creating...' : 'Create account'}
				</button>
				<a class="secondary-btn" href="{base}/app/login">Login</a>
			</form>
		</section>
	</div>
</div>
