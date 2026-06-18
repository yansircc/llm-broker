<script lang="ts">
	import { base } from '$app/paths';
	import { customerApi } from '$lib/customer-api';

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

<h2>customer login</h2>

<form onsubmit={login}>
	<fieldset>
		<legend>authenticate</legend>
		<label for="email">email</label>
		<input id="email" type="text" autocomplete="email" bind:value={email}>
		<label for="password">password</label>
		<input id="password" type="password" autocomplete="current-password" bind:value={password}>
		{#if error}<p class="error-msg">{error}</p>{/if}
		<button type="submit" disabled={loading || !email.trim() || !password}>{loading ? 'logging in...' : 'login'}</button>
		<span class="add-link"><a href="{base}/app/register">[register]</a></span>
	</fieldset>
</form>
