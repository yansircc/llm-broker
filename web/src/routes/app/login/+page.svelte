<script lang="ts">
	import { base } from '$app/paths';
	import { BRAND_NAME } from '$lib/brand';
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
	const authHighlights = [
		['1 元用 1 刀', '按 USD 额度入账，按实际调用扣费。'],
		['满血不掺水', '模型能力以实际已接入服务为准。'],
		['一把 Key 接入 8+ 工具', 'Codex 当前可用，Claude 家族接入后共用同一控制台。'],
		['1 元 = 1 刀', '充值额度以账户余额为唯一消费来源。'],
		['永不断线', '多账号池调度，异常状态可观察、可切换。']
	];

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
	<a href="{base}/" class="absolute left-4 top-4 text-sm text-faint transition-colors hover:text-white md:left-8 md:top-8">← 返回首页</a>
	<div class="mx-auto grid min-h-screen max-w-6xl items-center gap-10 px-5 py-16 lg:grid-cols-[1fr_0.95fr]">
		<section>
			<a href="{base}/" class="inline-flex items-center gap-3 font-semibold">
				<span class="brand-mark" aria-hidden="true"></span>
				<span class="text-2xl">{BRAND_NAME}</span>
			</a>
			<h1 class="mt-8 max-w-xl text-3xl font-semibold tracking-tight">1 元用 1 刀，登录 {BRAND_NAME} 控制台</h1>
			<p class="mt-4 max-w-lg text-sm leading-relaxed text-muted">当前提供 Codex 中转；Claude Opus / Sonnet / Haiku 家族正在接入，后续沿用同一套密钥、用量和账本。</p>
			<div class="mt-8 grid max-w-lg gap-3 text-sm text-muted">
				{#each authHighlights as item}
					<div class="rounded-lg border border-line bg-card/60 p-4">
						<div class="font-medium text-slate-100">{item[0]}</div>
						<div class="mt-1 text-xs leading-relaxed text-faint">{item[1]}</div>
					</div>
				{/each}
			</div>
		</section>

		<section class="rounded-xl border border-line bg-card/70 p-6 shadow-2xl">
			<div class="text-center">
				<a href="{base}/" class="inline-flex items-center justify-center gap-2 font-semibold">
					<span class="brand-mark" aria-hidden="true"></span>
					<span>{BRAND_NAME}</span>
				</a>
				<h2 class="mt-6 text-2xl font-semibold tracking-tight">登录</h2>
			</div>
			<form class="mt-7 space-y-4" onsubmit={login}>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="email">邮箱</label>
					<input id="email" class="h-10 w-full rounded-md border border-line bg-transparent px-3 text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="请输入邮箱" type="email" autocomplete="email" bind:value={email}>
				</div>
				<div>
					<label class="mb-1.5 block text-sm text-muted" for="password">密码</label>
					<input id="password" class="h-10 w-full rounded-md border border-line bg-transparent px-3 text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="请输入密码" type="password" autocomplete="current-password" bind:value={password}>
				</div>
				{#if turnstileEnabled}
					<div bind:this={turnstileEl}></div>
				{/if}
				{#if error}
					<p class="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">{error}</p>
				{/if}
				<button class="h-10 w-full rounded-md bg-brand text-sm font-semibold text-black disabled:opacity-50" type="submit" disabled={loading || !email.trim() || !password}>
					{loading ? '登录中...' : '登录'}
				</button>
				<div class="flex items-center justify-between text-xs text-muted">
					<button class="border-0 bg-transparent p-0 text-xs text-faint hover:text-brand disabled:cursor-not-allowed disabled:opacity-60" type="button" disabled>忘记密码？</button>
					<span>没有账户？<a class="ml-1 text-brand hover:underline" href="{base}/app/register">注册</a></span>
				</div>
				<p class="text-center text-xs text-faint">登录即表示同意 <a class="underline underline-offset-4 hover:text-white" href="{base}/terms-of-service">服务条款</a> 和 <a class="underline underline-offset-4 hover:text-white" href="{base}/privacy-policy">隐私政策</a></p>
			</form>
		</section>
	</div>
</main>
