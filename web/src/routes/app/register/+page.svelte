<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { customerApi } from '$lib/customer-api';
	import type { AuthResponse } from '$lib/customer-types';
	import { onMount, tick } from 'svelte';

	let email = $state('');
	let password = $state('');
	let name = $state('');
	let referralCode = $state('');
	let referralInitialized = $state(false);
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

	$effect(() => {
		if (referralInitialized) return;
		referralCode = $page.url.searchParams.get('ref') ?? $page.url.searchParams.get('code') ?? '';
		referralInitialized = true;
	});

	async function register(e: Event) {
		e.preventDefault();
		if (!email.trim() || !password) return;
		if (turnstileEnabled && !turnstileToken) {
			error = '请先完成人机验证';
			return;
		}
		loading = true;
		error = '';
		try {
			const body: Record<string, string | undefined> = {
				email: email.trim(),
				password,
				name: name.trim() || undefined,
				turnstile_token: turnstileToken || undefined
			};
			if (referralCode.trim()) body.referral_code = referralCode.trim();
			const res = await customerApi<AuthResponse>('/auth/register', {
				method: 'POST',
				body: JSON.stringify(body)
			});
			window.location.href = `${base}${res.redirect_to ?? '/app/dashboard'}`;
		} catch (err: any) {
			error = err.message || 'registration failed';
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
			<div class="mt-10 font-mono text-xs uppercase tracking-wider text-brand">prepaid relay</div>
			<h1 class="mt-4 max-w-xl text-5xl font-bold leading-tight tracking-tight">创建 CDX 账号。</h1>
			<p class="mt-5 max-w-lg text-base text-muted">充值人民币获得 USD 额度，OpenAI/Codex 请求按实际 token 消费扣费。</p>
			<div class="mt-8 rounded-lg border border-line bg-card/60 p-5 text-sm text-muted">
				<div class="font-medium text-slate-100">邀请奖励规则</div>
				<div class="mt-2">受邀方注册后自动获得奖励；邀请方在受邀方首次付费后获得奖励。</div>
			</div>
		</section>

		<section class="rounded-xl border border-line bg-card/70 p-6 shadow-2xl">
			<div>
				<div class="font-mono text-xs uppercase tracking-wider text-brand">register</div>
				<h2 class="mt-2 text-2xl font-bold">客户注册</h2>
			</div>
			<form class="mt-7 space-y-4" onsubmit={register}>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="name">名称</label>
					<input id="name" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="text" autocomplete="name" bind:value={name}>
				</div>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="email">邮箱</label>
					<input id="email" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="email" autocomplete="email" bind:value={email}>
				</div>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="password">密码</label>
					<input id="password" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="password" autocomplete="new-password" bind:value={password}>
					<div class="mt-1 text-xs text-faint">至少 8 个字符。</div>
				</div>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="referral">邀请码</label>
					<input id="referral" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="text" autocomplete="off" bind:value={referralCode}>
				</div>
				{#if turnstileEnabled}
					<div bind:this={turnstileEl}></div>
				{/if}
				{#if error}
					<p class="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">{error}</p>
				{/if}
				<button class="h-11 w-full rounded-md bg-brand text-sm font-semibold text-black disabled:opacity-50" type="submit" disabled={loading || !email.trim() || !password}>
					{loading ? '创建中...' : '创建账号'}
				</button>
				<a class="flex h-11 w-full items-center justify-center rounded-md border border-line text-sm hover:border-brand/50" href="{base}/app/login">已有账号，去登录</a>
			</form>
		</section>
	</div>
</main>
