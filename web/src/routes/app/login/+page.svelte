<script lang="ts">
	import { base } from '$app/paths';
	import { customerApi } from '$lib/customer-api';
	import Logo from '$lib/components/Logo.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function login(e: Event) {
		e.preventDefault();
		if (!email.trim() || !password) return;
		loading = true;
		error = '';
		try {
			await customerApi('/auth/login', {
				method: 'POST',
				body: JSON.stringify({ email: email.trim(), password })
			});
			window.location.href = `${base}/app/dashboard`;
		} catch (err: any) {
			error = err.message || 'login failed';
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
				<div class="eyebrow auth-eyebrow">customer console</div>
				<h1>API keys, prepaid balance, token usage.</h1>
				<p class="lede">Sign in to manage relay access, recharge credits, and inspect referral rewards.</p>
			</div>
			<TerminalPanel title="session gate" lines={['auth: customer', 'surface: openai/responses', 'billing: prepaid ledger']} />
		</section>

		<section class="auth-card">
			<div class="page-header compact">
				<div>
					<div class="eyebrow">login</div>
					<h2>Customer Login</h2>
				</div>
			</div>
			<form onsubmit={login}>
				<label for="email">email</label>
				<input id="email" type="text" autocomplete="email" bind:value={email}>
				<label for="password">password</label>
				<input id="password" type="password" autocomplete="current-password" bind:value={password}>
				{#if error}<p class="error-msg">{error}</p>{/if}
				<button class="primary-btn" type="submit" disabled={loading || !email.trim() || !password}>
					{loading ? 'Logging in...' : 'Login'}
				</button>
				<a class="secondary-btn" href="{base}/app/register">Create account</a>
			</form>
		</section>
	</div>
</div>
