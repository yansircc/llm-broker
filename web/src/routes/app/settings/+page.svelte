<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { CustomerMe } from '$lib/customer-types';
	import { fmtDate } from '$lib/format';

	let me = $state<CustomerMe | null>(null);
	let error = $state('');
	let passwordMessage = $state('');
	let passwordError = $state('');
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let loading = $state(false);
	let changingPassword = $state(false);

	$effect(() => {
		loadMe();
	});

	async function loadMe() {
		loading = true;
		error = '';
		try {
			me = await customerApi<CustomerMe>('/me');
		} catch (e: any) {
			error = e.message || 'failed to load account';
		} finally {
			loading = false;
		}
	}

	async function changePassword(e: Event) {
		e.preventDefault();
		passwordMessage = '';
		passwordError = '';
		if (newPassword.length < 8) {
			passwordError = '新密码至少需要 8 位';
			return;
		}
		if (newPassword !== confirmPassword) {
			passwordError = '两次输入的新密码不一致';
			return;
		}
		changingPassword = true;
		try {
			await customerApi('/me/password', {
				method: 'POST',
				body: JSON.stringify({
					current_password: currentPassword,
					new_password: newPassword
				})
			});
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			passwordMessage = '密码已更新';
		} catch (e: any) {
			passwordError = e.message || '密码更新失败';
		} finally {
			changingPassword = false;
		}
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">settings</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">个人设置</h1>
		<p class="mt-2 text-sm text-muted">账户信息和登录密码。</p>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadMe}>刷新</button>
</div>

{#if error}
	<p class="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{:else if loading}
	<p class="text-sm text-faint">正在加载...</p>
{:else if me}
	<section class="max-w-2xl rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">账户信息</h2>
		<div class="mt-5 grid gap-4">
			<div>
				<div class="mb-1 text-xs text-faint">邮箱</div>
				<div class="rounded-md border border-line bg-black/25 px-3 py-2">{me.user.email}</div>
			</div>
			<div>
				<div class="mb-1 text-xs text-faint">名称</div>
				<div class="rounded-md border border-line bg-black/25 px-3 py-2">{me.user.name || '-'}</div>
			</div>
			<div>
				<div class="mb-1 text-xs text-faint">状态</div>
				<div class="rounded-md border border-line bg-black/25 px-3 py-2">{me.user.status}</div>
			</div>
			<div>
				<div class="mb-1 text-xs text-faint">创建时间</div>
				<div class="rounded-md border border-line bg-black/25 px-3 py-2">{me.user.created_at ? fmtDate(me.user.created_at) : '-'}</div>
			</div>
		</div>
	</section>

	<section class="mt-5 max-w-2xl rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">修改用户名</h2>
		<p class="mt-2 text-sm text-faint">用户自行修改用户名的视觉入口已保留；本分支尚未提供更新用户名 API。</p>
		<div class="mt-5 flex flex-col gap-3 sm:flex-row">
			<input class="h-11 rounded-md border border-line bg-black/30 px-3 text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="新用户名" value={me.user.name || ''} disabled>
			<button class="h-11 rounded-md border border-line bg-black/20 px-5 text-sm text-faint" type="button" disabled>更新</button>
		</div>
	</section>

	<section class="mt-5 max-w-2xl rounded-lg border border-line bg-card/60 p-5">
		<h2 class="m-0 text-base font-semibold">修改密码</h2>
		<form class="mt-5 grid gap-4" onsubmit={changePassword}>
			<div>
				<label class="mb-1.5 block text-sm text-muted" for="current-password">当前密码</label>
				<input id="current-password" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="password" autocomplete="current-password" bind:value={currentPassword}>
			</div>
			<div>
				<label class="mb-1.5 block text-sm text-muted" for="new-password">新密码</label>
				<input id="new-password" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="password" autocomplete="new-password" bind:value={newPassword}>
			</div>
			<div>
				<label class="mb-1.5 block text-sm text-muted" for="confirm-password">确认新密码</label>
				<input id="confirm-password" class="h-11 w-full rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" type="password" autocomplete="new-password" bind:value={confirmPassword}>
			</div>
			{#if passwordError}
				<p class="m-0 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{passwordError}</p>
			{/if}
			{#if passwordMessage}
				<p class="m-0 rounded-md border border-brand/30 bg-brand/10 px-4 py-3 text-sm text-brand">{passwordMessage}</p>
			{/if}
			<button class="h-11 w-full rounded-md bg-brand text-sm font-semibold text-black disabled:opacity-50 sm:w-fit sm:px-6" type="submit" disabled={changingPassword || !currentPassword || !newPassword || !confirmPassword}>
				{changingPassword ? '更新中...' : '更新密码'}
			</button>
		</form>
	</section>
{/if}
