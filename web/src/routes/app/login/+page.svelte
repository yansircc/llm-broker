<script lang="ts">
	import { base } from '$app/paths';
	import { customerApi } from '$lib/customer-api';
	import type { AuthResponse } from '$lib/customer-types';
	import { onMount, tick } from 'svelte';

	let email = $state('');
	let password = $state('');
	let turnstileEnabled = $state(false);
	let turnstileSiteKey = $state('');
	let turnstileToken = $state('');
	let turnstileEl = $state<HTMLDivElement | null>(null);
	let turnstileWidgetId = $state<string | number | null>(null);
	let error = $state('');
	let loading = $state(false);

	onMount(async () => {
		await loadPublicConfig();
	});

	async function login(e: Event) {
		e.preventDefault();
		if (!email.trim() || !password) return;
		if (turnstileEnabled && !turnstileToken) {
			error = '请先完成人机验证';
			return;
		}
		loading = true;
		error = '';
		try {
			const res = await customerApi<AuthResponse>('/auth/login', {
				method: 'POST',
				body: JSON.stringify({ email: email.trim(), password, turnstile_token: turnstileToken || undefined })
			});
			window.location.href = `${base}${res.redirect_to ?? '/app/dashboard'}`;
		} catch (err: any) {
			error = err.message || 'login failed';
			resetTurnstile();
		} finally {
			loading = false;
		}
	}

	async function loadPublicConfig() {
		const res = await fetch('/api/public/config', { headers: { Accept: 'application/json' } }).catch(() => null);
		if (!res?.ok) return;
		const cfg = await res.json();
		turnstileEnabled = !!cfg.turnstile_enabled;
		turnstileSiteKey = cfg.turnstile_site_key || '';
		if (turnstileEnabled && turnstileSiteKey) {
			await loadTurnstileScript();
			await tick();
			renderTurnstile();
		}
	}

	function loadTurnstileScript() {
		return new Promise<void>((resolve, reject) => {
			if ((window as any).turnstile) return resolve();
			const existing = document.getElementById('cf-turnstile-script') as HTMLScriptElement | null;
			if (existing) {
				existing.addEventListener('load', () => resolve(), { once: true });
				existing.addEventListener('error', () => reject(new Error('turnstile script failed')), { once: true });
				return;
			}
			const script = document.createElement('script');
			script.id = 'cf-turnstile-script';
			script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
			script.async = true;
			script.defer = true;
			script.onload = () => resolve();
			script.onerror = () => reject(new Error('turnstile script failed'));
			document.head.appendChild(script);
		});
	}

	function renderTurnstile() {
		if (!turnstileEl || !(window as any).turnstile || !turnstileSiteKey) return;
		turnstileWidgetId = (window as any).turnstile.render(turnstileEl, {
			sitekey: turnstileSiteKey,
			theme: 'dark',
			callback: (token: string) => (turnstileToken = token),
			'expired-callback': () => (turnstileToken = '')
		});
	}

	function resetTurnstile() {
		turnstileToken = '';
		if (turnstileWidgetId !== null && (window as any).turnstile) {
			(window as any).turnstile.reset(turnstileWidgetId);
		}
	}
</script>

<main class="min-h-screen bg-bg text-slate-100">
	<div class="mx-auto grid min-h-screen max-w-6xl items-center gap-10 px-5 py-10 lg:grid-cols-[1.1fr_0.9fr]">
		<section>
			<a href="{base}/" class="inline-flex items-center gap-3 font-semibold">
				<span class="flex h-9 w-9 items-center justify-center rounded-md border border-brand/50 bg-black text-xs font-bold text-brand">CD</span>
				<span>CDX</span>
			</a>
			<div class="mt-10 font-mono text-xs uppercase tracking-wider text-brand">customer console</div>
			<h1 class="mt-4 max-w-xl text-5xl font-bold leading-tight tracking-tight">API keys, prepaid balance, token usage.</h1>
			<p class="mt-5 max-w-lg text-base text-muted">登录后管理 OpenAI/Codex relay 访问、充值额度、查看 token 消费和邀请奖励。</p>
			<div class="mt-8 rounded-lg border border-line bg-card/60 p-5 font-mono text-sm text-brand">
				<div>$ export OPENAI_API_KEY=cdx_live_***</div>
				<div>$ curl /v1/responses</div>
				<div class="text-faint">&lt; balance checked, token metered</div>
			</div>
		</section>

		<section class="rounded-xl border border-line bg-card/70 p-6 shadow-2xl">
			<div>
				<div class="font-mono text-xs uppercase tracking-wider text-brand">login</div>
				<h2 class="mt-2 text-2xl font-bold">登录 CDX</h2>
			</div>
			<form class="mt-7 space-y-4" onsubmit={login}>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="email">邮箱</label>
					<input id="email" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="email" autocomplete="email" bind:value={email}>
				</div>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="password">密码</label>
					<input id="password" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="password" autocomplete="current-password" bind:value={password}>
				</div>
				{#if turnstileEnabled}
					<div bind:this={turnstileEl}></div>
				{/if}
				{#if error}
					<p class="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">{error}</p>
				{/if}
				<button class="h-11 w-full rounded-md bg-brand text-sm font-semibold text-black disabled:opacity-50" type="submit" disabled={loading || !email.trim() || !password}>
					{loading ? '登录中...' : '登录'}
				</button>
				<a class="flex h-11 w-full items-center justify-center rounded-md border border-line text-sm hover:border-brand/50" href="{base}/app/register">创建账号</a>
			</form>
		</section>
	</div>
</main>
