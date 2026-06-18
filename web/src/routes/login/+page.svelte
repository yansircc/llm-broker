<script lang="ts">
	import { base } from '$app/paths';
	import Logo from '$lib/components/Logo.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';

	let token = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleLogin(e: Event) {
		e.preventDefault();
		if (!token.trim()) return;
		loading = true;
		error = '';
		try {
			const res = await fetch('/admin/login', {
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token: token.trim() })
			});
			if (!res.ok) {
				const text = await res.text();
				error = text || `Login failed (${res.status})`;
				return;
			}
			window.location.href = `${base}/dashboard`;
		} catch (err) {
			error = 'Network error';
		} finally {
			loading = false;
		}
	}
</script>

<div class="auth-page">
	<div class="auth-grid">
		<section class="auth-intro">
			<div>
				<Logo href={`${base}/`} label="CDX Admin" />
				<div class="eyebrow auth-eyebrow">operator access</div>
				<h1>Broker control plane.</h1>
				<p class="lede">Account pool state, relay failures, user billing, and payment orders stay behind the admin token gate.</p>
			</div>
			<TerminalPanel title="admin boundary" lines={['surface: /admin/*', 'auth: token session', 'scope: operations']} />
		</section>

		<section class="auth-card">
			<div class="page-header compact">
				<div>
					<div class="eyebrow">login</div>
					<h2>Admin Login</h2>
				</div>
			</div>
			<form onsubmit={handleLogin}>
				<label for="admin-token">admin token</label>
				<input id="admin-token" type="password" bind:value={token} placeholder="enter admin token">
				{#if error}
					<p class="error-msg">{error}</p>
				{/if}
				<button class="primary-btn" type="submit" disabled={loading || !token.trim()}>
					{loading ? 'Logging in...' : 'Login'}
				</button>
			</form>
		</section>
	</div>
</div>
