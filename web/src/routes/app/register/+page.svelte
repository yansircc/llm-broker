<script lang="ts">
	import { base } from '$app/paths';
	import { customerApi } from '$lib/customer-api';

	let email = $state('');
	let password = $state('');
	let name = $state('');
	let error = $state('');
	let loading = $state(false);

	async function register(e: Event) {
		e.preventDefault();
		if (!email.trim() || !password) return;
		loading = true;
		error = '';
		try {
			await customerApi('/auth/register', {
				method: 'POST',
				body: JSON.stringify({ email: email.trim(), password, name: name.trim() || undefined })
			});
			window.location.href = `${base}/app/dashboard`;
		} catch (err: any) {
			error = err.message || 'registration failed';
		} finally {
			loading = false;
		}
	}
</script>

<h2>customer registration</h2>

<form onsubmit={register}>
	<fieldset>
		<legend>create account</legend>
		<label for="name">name</label>
		<input id="name" type="text" autocomplete="name" bind:value={name}>
		<label for="email">email</label>
		<input id="email" type="text" autocomplete="email" bind:value={email}>
		<label for="password">password</label>
		<input id="password" type="password" autocomplete="new-password" bind:value={password}>
		{#if error}<p class="error-msg">{error}</p>{/if}
		<button type="submit" disabled={loading || !email.trim() || !password}>{loading ? 'creating...' : 'register'}</button>
		<span class="add-link"><a href="{base}/app/login">[login]</a></span>
	</fieldset>
</form>
