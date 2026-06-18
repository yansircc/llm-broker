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
				<h2 class="mt-2 text-2xl font-bold">客户登录</h2>
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
