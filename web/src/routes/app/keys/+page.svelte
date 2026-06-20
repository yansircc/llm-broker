<script lang="ts">
	import { browser } from '$app/environment';
	import { customerApi } from '$lib/customer-api';
	import type { CustomerApiKey, CustomerApiKeyCreated } from '$lib/customer-types';
	import { timeAgo } from '$lib/format';

	let keys = $state<CustomerApiKey[]>([]);
	let created = $state<CustomerApiKeyCreated | null>(null);
	let name = $state('');
	let dailyBudget = $state('');
	let monthlyBudget = $state('');
	let search = $state('');
	let statusFilter = $state('all');
	let error = $state('');
	let loading = $state(false);
	let creating = $state(false);
	let saving = $state('');
	let copied = $state('');
	let origin = $state('https://your-domain.example');
	let drafts = $state<Record<string, { name: string; status: string; daily: string; monthly: string }>>({});
	const filteredKeys = $derived(
		keys.filter((key) => {
			const q = search.trim().toLowerCase();
			const matchesStatus = statusFilter === 'all' || key.status === statusFilter;
			const matchesSearch = !q || [key.name, key.prefix, key.status].some((value) => String(value ?? '').toLowerCase().includes(q));
			return matchesStatus && matchesSearch;
		})
	);

	$effect(() => {
		if (browser) origin = window.location.origin;
		loadKeys();
	});

	async function loadKeys() {
		loading = true;
		error = '';
		try {
			keys = await customerApi<CustomerApiKey[]>('/keys');
			syncDrafts();
		} catch (e: any) {
			error = e.message || 'failed to load keys';
		} finally {
			loading = false;
		}
	}

	async function createKey() {
		creating = true;
		error = '';
		try {
			created = await customerApi<CustomerApiKeyCreated>('/keys', {
				method: 'POST',
				body: JSON.stringify({
					name: name.trim() || 'default',
					daily_budget_usd: parseBudget(dailyBudget),
					monthly_budget_usd: parseBudget(monthlyBudget)
				})
			});
			keys = [created, ...keys.filter((key) => key.id !== created?.id)];
			syncDrafts();
			name = '';
			dailyBudget = '';
			monthlyBudget = '';
		} catch (e: any) {
			error = e.message || 'failed to create key';
		} finally {
			creating = false;
		}
	}

	function parseBudget(value: string) {
		const n = Number(value);
		return Number.isFinite(n) && n > 0 ? n : 0;
	}

	function syncDrafts() {
		const next: Record<string, { name: string; status: string; daily: string; monthly: string }> = {};
		for (const key of keys) {
			next[key.id] = {
				name: key.name,
				status: key.status,
				daily: key.daily_budget_usd ? String(key.daily_budget_usd) : '',
				monthly: key.monthly_budget_usd ? String(key.monthly_budget_usd) : ''
			};
		}
		drafts = next;
	}

	async function saveKey(key: CustomerApiKey) {
		const draft = drafts[key.id];
		if (!draft) return;
		saving = key.id;
		error = '';
		try {
			const updated = await customerApi<CustomerApiKey>(`/keys/${key.id}`, {
				method: 'PATCH',
				body: JSON.stringify({
					name: draft.name.trim() || key.name,
					status: draft.status,
					daily_budget_usd: parseBudget(draft.daily),
					monthly_budget_usd: parseBudget(draft.monthly)
				})
			});
			keys = keys.map((item) => (item.id === updated.id ? updated : item));
			syncDrafts();
		} catch (e: any) {
			error = e.message || 'failed to save key';
		} finally {
			saving = '';
		}
	}

	async function deleteKey(key: CustomerApiKey) {
		if (!window.confirm(`Delete API key "${key.name}"?`)) return;
		error = '';
		try {
			await customerApi(`/keys/${key.id}`, { method: 'DELETE' });
			keys = keys.filter((item) => item.id !== key.id);
			syncDrafts();
		} catch (e: any) {
			error = e.message || 'failed to delete key';
		}
	}

	async function copy(text: string, id: string) {
		await navigator.clipboard.writeText(text);
		copied = id;
		setTimeout(() => {
			if (copied === id) copied = '';
		}, 1800);
	}
</script>

<div class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
	<div>
		<div class="font-mono text-xs uppercase tracking-wider text-brand">access</div>
		<h1 class="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">API 密钥</h1>
	</div>
	<button class="h-10 rounded-md border border-line bg-card px-4 text-sm hover:border-brand/50" onclick={loadKeys}>刷新</button>
</div>

{#if error}
	<p class="mb-4 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">{error}</p>
{/if}

<section class="rounded-lg border border-line bg-card/60 p-5">
	<div class="grid gap-4 lg:grid-cols-[1fr_1.2fr]">
			<div>
				<h2 class="m-0 text-base font-semibold">创建密钥</h2>
				<p class="mt-1 text-sm text-faint">平台 Key 会安全保存，可随时回来管理。完整 token 仅在创建时返回一次，请立即复制保存。</p>
			<div class="mt-4 flex flex-col gap-3 sm:flex-row">
				<input class="h-11 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand" placeholder="default" bind:value={name} disabled={creating}>
				<input class="h-11 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand sm:w-40" inputmode="decimal" placeholder="日预算 $，可空" bind:value={dailyBudget} disabled={creating}>
				<input class="h-11 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand sm:w-40" inputmode="decimal" placeholder="月预算 $，可空" bind:value={monthlyBudget} disabled={creating}>
				<button class="h-11 min-w-[96px] whitespace-nowrap rounded-md bg-brand px-5 text-sm font-semibold text-black disabled:opacity-50" onclick={createKey} disabled={creating}>
					{creating ? '创建中...' : '创建密钥'}
				</button>
			</div>
		</div>
		<div class="rounded-md border border-line bg-black/25 p-4">
				<div class="text-xs text-faint">推荐接入地址</div>
				<div class="mt-2 space-y-1 font-mono text-sm">
					<div>OpenAI 兼容（Codex / GPT）: <span class="text-brand">{origin}/v1</span></div>
					<div>Anthropic（Claude 家族）: <span class="text-brand">{origin}</span></div>
					<div>Responses base_url: <span class="text-brand">{origin}/openai</span></div>
				</div>
			</div>
	</div>
</section>

{#if created}
	<section class="mt-5 rounded-lg border border-brand/30 bg-brand/[0.05] p-5">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
				<div>
					<h2 class="m-0 text-base font-semibold">新密钥</h2>
					<p class="mt-1 text-sm text-faint">当前版本只在创建时显示一次完整 token，完整回看能力已列入功能缺口。</p>
				</div>
			<div class="flex gap-2">
				<button class="rounded-md border border-line bg-card px-3 py-2 text-sm hover:border-brand/50" onclick={() => copy(created?.token ?? '', 'new-token')}>
					{copied === 'new-token' ? '已复制' : '复制'}
				</button>
				<button class="rounded-md border border-line bg-card px-3 py-2 text-sm hover:border-brand/50" onclick={() => (created = null)}>关闭</button>
			</div>
		</div>
		<div class="mt-4 break-all rounded-md border border-line bg-black/40 p-3 font-mono text-sm text-brand">{created.token}</div>
	</section>
{/if}

<section class="mt-6 rounded-lg border border-line bg-card/60">
	<div class="flex flex-col gap-3 border-b border-line px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
		<h2 class="m-0 text-base font-semibold">密钥列表</h2>
		<div class="flex flex-col gap-2 sm:flex-row">
			<input class="h-10 max-w-sm rounded-md border border-line bg-black/30 px-3 text-sm outline-none placeholder:text-faint focus:border-brand" placeholder="搜索名称、Key 或分组..." bind:value={search}>
			<select class="h-10 rounded-md border border-line bg-black/30 px-3 text-sm outline-none focus:border-brand sm:w-36" bind:value={statusFilter}>
				<option value="all">全部状态</option>
				<option value="active">active</option>
				<option value="disabled">disabled</option>
			</select>
		</div>
	</div>
	{#if loading}
		<p class="p-5 text-sm text-faint">正在加载...</p>
	{:else if filteredKeys.length === 0}
		<p class="p-5 text-sm text-faint">暂无 API 密钥。</p>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full min-w-[760px] text-left text-sm">
				<thead class="bg-white/[0.03] font-mono text-xs text-faint">
					<tr>
						<th class="px-5 py-3 font-medium">名称</th>
						<th class="px-5 py-3 font-medium">API 密钥</th>
						<th class="px-5 py-3 font-medium">分组</th>
						<th class="px-5 py-3 font-medium">速率限制</th>
						<th class="px-5 py-3 font-medium">过期</th>
						<th class="px-5 py-3 font-medium">状态</th>
						<th class="px-5 py-3 font-medium">上次使用</th>
						<th class="px-5 py-3 font-medium">操作</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each filteredKeys as key (key.id)}
						<tr class="hover:bg-white/[0.02]">
							<td class="px-5 py-3">
								<input class="h-9 w-40 rounded-md border border-line bg-black/30 px-2 text-sm outline-none focus:border-brand" bind:value={drafts[key.id].name}>
							</td>
							<td class="px-5 py-3 font-mono text-faint">{key.prefix ?? 'sk-***'}</td>
							<td class="px-5 py-3"><span class="rounded-full border border-line px-2 py-1 text-xs text-faint">默认分组</span></td>
							<td class="px-5 py-3 text-xs text-faint">
								<div>日：{drafts[key.id].daily || '不限'}</div>
								<div>月：{drafts[key.id].monthly || '不限'}</div>
							</td>
							<td class="px-5 py-3 text-faint">永久</td>
							<td class="px-5 py-3">
								<select class="h-9 rounded-md border border-line bg-black/30 px-2 text-sm outline-none focus:border-brand" bind:value={drafts[key.id].status}>
									<option value="active">active</option>
									<option value="disabled">disabled</option>
								</select>
							</td>
							<td class="px-5 py-3">{key.last_used_at ? timeAgo(key.last_used_at) : '-'}</td>
							<td class="px-5 py-3">
								<div class="flex gap-2">
									<button class="rounded-md border border-line bg-card px-3 py-1.5 text-xs hover:border-brand/50 disabled:opacity-50" onclick={() => saveKey(key)} disabled={saving === key.id}>{saving === key.id ? '保存中' : '保存'}</button>
									<button class="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-1.5 text-xs text-red-300 hover:border-red-400" onclick={() => deleteKey(key)}>删除</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
