<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { CustomerMe } from '$lib/customer-types';
	import { fmtDate } from '$lib/format';

	let me = $state<CustomerMe | null>(null);
	let error = $state('');
	let loading = $state(false);

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
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">settings</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">个人设置</h1>
		<p class="mt-2 text-sm text-muted">账户信息。邮箱和密码修改后续单独接入。</p>
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
{/if}
