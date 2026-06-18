<script lang="ts">
	import { base } from '$app/paths';
	import { page } from '$app/stores';
	import { customerApi } from '$lib/customer-api';

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
			if (referralCode.trim()) body.referral_code = referralCode.trim();
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
