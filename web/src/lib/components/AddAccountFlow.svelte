<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import { addAccountPath, type ProviderOption } from '$lib/providers';

	interface Props {
		providerID: string;
	}

	let { providerID }: Props = $props();

	let providers = $state<ProviderOption[]>([]);
	let provider = $state<ProviderOption | null>(null);
	let loadingProvider = $state(true);
	let providerError = $state('');
	let generating = $state(false);
	let exchanging = $state(false);
	let sessionId = $state('');
	let authUrl = $state('');
	let callbackInput = $state('');
	let genError = $state('');
	let exchangeError = $state('');
	let result = $state<{ id: string; email: string; status: string } | null>(null);

	$effect(() => {
		void loadProvider(providerID);
	});

	async function loadProvider(id: string) {
		loadingProvider = true;
		providerError = '';
		provider = null;
		reset();
		try {
			providers = await api<ProviderOption[]>('/providers');
			provider = providers.find((option) => option.id === id) ?? null;
			if (!provider) {
				providerError = `unknown provider: ${id}`;
			}
		} catch (e: any) {
			providerError = e.message;
		} finally {
			loadingProvider = false;
		}
	}

	async function generateAuthUrl() {
		if (!provider) return;
		generating = true;
		genError = '';
		try {
			const data = await api<{ session_id: string; auth_url: string }>(`/accounts/generate-auth-url?provider=${provider.id}`, { method: 'POST' });
			sessionId = data.session_id;
			authUrl = data.auth_url;
		} catch (e: any) {
			genError = e.message;
		} finally {
			generating = false;
		}
	}

	async function exchangeCode() {
		if (!provider || !callbackInput.trim()) return;
		exchanging = true;
		exchangeError = '';
		try {
			const data = await api<{ id: string; email: string; status: string }>('/accounts/exchange-code', {
				method: 'POST',
				body: JSON.stringify({
					provider: provider.id,
					session_id: sessionId,
					callback_url: callbackInput.trim()
				})
			});
			result = data;
		} catch (e: any) {
			exchangeError = e.message;
		} finally {
			exchanging = false;
		}
	}

	function reset() {
		sessionId = '';
		authUrl = '';
		callbackInput = '';
		genError = '';
		exchangeError = '';
		result = null;
	}
</script>

<h2>add account</h2>

{#if loadingProvider}
	<p class="loading">loading provider...</p>
{:else if providerError}
	<p class="error-msg">{providerError}</p>
	{#if providers.length > 0}
		<div class="bar">
			available providers:
			{#each providers as option, i (option.id)}
				{#if i > 0}<span class="muted">|</span>{/if}
				<a href={addAccountPath(base, option.id)}>{option.label}</a>
			{/each}
		</div>
	{/if}
{:else if provider}
	<div class="bar" style="margin-bottom:12px">
		provider: <b>{provider.label}</b>
	</div>

	<h2>authorize {#if sessionId}<span class="g">&#10003;</span>{/if}</h2>
	{#if !sessionId}
		<p class="hint">generate an OAuth URL, open it in browser, login and authorize.</p>
		<button class="link" onclick={generateAuthUrl} disabled={generating}>
			{generating ? '[generating...]' : '[generate auth url]'}
		</button>
		{#if genError}
			<p class="error-msg">{genError}</p>
		{/if}
	{:else}
		<div class="bar">
			session: <b>{sessionId}</b> <span class="muted">(expires in 10m)</span>
			<br><br>
			auth url:<br>
			<a href={authUrl} target="_blank" class="auth-url">{authUrl}</a>
			<br><br>
			<span class="muted">open this URL, login and authorize. then paste the callback below.</span>
		</div>
	{/if}

	{#if sessionId}
		<h2>exchange code {#if result}<span class="g">&#10003;</span>{/if}</h2>
		{#if !result}
			<label for="callback-input">callback url or code</label>
			<input id="callback-input" type="text" bind:value={callbackInput} placeholder={provider.callback_placeholder}>
			<p class="hint">{provider.callback_hint}</p>
			{#if exchangeError}
				<p class="error-msg">{exchangeError}</p>
			{/if}
			<button class="link" onclick={exchangeCode} disabled={exchanging || !callbackInput.trim()}>
				{exchanging ? '[exchanging...]' : '[exchange code]'}
			</button>
		{:else}
			<div class="bar">
				<span class="g">&#10003; account created</span>
				<br><br>
				email: <b>{result.email}</b><br>
				status: <b class="g">{result.status}</b><br>
				<br>
				<a href="{base}/accounts/{result.id}">view account &rarr;</a>
			</div>
		{/if}
	{/if}
{/if}

<p style="margin-top:16px;font-size:12px"><a href="{base}/dashboard">&larr; back</a></p>
