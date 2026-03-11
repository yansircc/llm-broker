<script lang="ts">
	import { base } from '$app/paths';
	import { api } from '$lib/api';
	import type { AccountListItem } from '$lib/admin-types';
	import { remainClass, remainTime, timeAgo, dotClass } from '$lib/format';
	import Countdown from '$lib/components/Countdown.svelte';
	import { addAccountPath, type ProviderOption } from '$lib/providers';

	interface AccountGroup {
		provider: string;
		label: string;
		accounts: AccountListItem[];
		window_labels: string[];
	}

	let accounts = $state<AccountListItem[]>([]);
	let providers = $state<ProviderOption[]>([]);
	let error = $state('');
	let providerError = $state('');
	let lastRefresh = $state('');

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		error = '';
		providerError = '';
		try {
			const [accountList, providerList] = await Promise.all([
				api<AccountListItem[]>('/accounts'),
				api<ProviderOption[]>('/providers').catch((e: any) => {
					providerError = e.message;
					return [];
				})
			]);
			accounts = accountList;
			providers = providerList;
			lastRefresh = new Date().toLocaleTimeString('en-GB', { hour12: false });
		} catch (e: any) {
			error = e.message;
		}
	}

	function windowAt(account: AccountListItem, index: number) {
		return account.windows[index] ?? null;
	}

	function groupAccounts(items: AccountListItem[]): AccountGroup[] {
		const groups = new Map<string, AccountGroup>();
		for (const account of items) {
			let group = groups.get(account.provider);
			if (!group) {
				group = {
					provider: account.provider,
					label: account.provider,
					accounts: [],
					window_labels: []
				};
				groups.set(account.provider, group);
			}
			group.accounts.push(account);
			account.windows.forEach((window, index) => {
				if (!group!.window_labels[index]) group!.window_labels[index] = window.label;
			});
		}
		return [...groups.values()].sort((a, b) => a.provider.localeCompare(b.provider));
	}

	function displayGroups(items: AccountListItem[], availableProviders: ProviderOption[]): AccountGroup[] {
		const grouped = new Map(groupAccounts(items).map((group) => [group.provider, group]));
		const ordered: AccountGroup[] = [];

		for (const provider of availableProviders) {
			const existing = grouped.get(provider.id);
			if (existing) {
				existing.label = provider.label;
				ordered.push(existing);
				grouped.delete(provider.id);
				continue;
			}
			ordered.push({
				provider: provider.id,
				label: provider.label,
				accounts: [],
				window_labels: []
			});
		}

		for (const leftover of [...grouped.values()].sort((a, b) => a.provider.localeCompare(b.provider))) {
			ordered.push(leftover);
		}
		return ordered;
	}
</script>

{#if error}
	<p class="error-msg">{error}</p>
{:else}
	<span class="refresh"><button class="link" onclick={loadAll}>[refresh]</button> <span class="muted">{lastRefresh}</span></span>
	<div class="sub">{accounts.length} accounts</div>

	{@const accountGroups = displayGroups(accounts, providers)}
	{#if accountGroups.length === 0}
		<p class="muted">no providers available</p>
	{:else}
		{#each accountGroups as group (group.provider)}
			<h2>{group.provider} accounts <a href={addAccountPath(base, group.provider)} class="add-link">[+ add]</a></h2>
			{#if group.accounts.length === 0}
				<p class="muted">no {group.label} accounts</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th>email</th>
							<th>status</th>
							<th>cell</th>
							<th>pri</th>
							<th>cooldown</th>
							<th>last used</th>
							{#each group.window_labels as label, index (`${group.provider}:${label}:${index}`)}
								<th class="num">{label}</th>
							{/each}
						</tr>
					</thead>
					<tbody>
						{#each group.accounts as account (account.id)}
							<tr>
								<td><a href="{base}/accounts/{account.id}">{account.email}</a></td>
								<td><span class={dotClass(account.status)}>{account.status}</span></td>
								<td>
									{#if account.cell_id}
										<a href="{base}/cells/{account.cell_id}">{account.cell?.name ?? account.cell_id}</a>
									{:else}
										<span class="muted">legacy direct</span>
									{/if}
								</td>
								<td>{account.priority}{#if account.priority_mode === 'auto'} <span class="muted">(a)</span>{/if}</td>
								<Countdown until={account.cooldown_until} tag="td" variant="cooldown" />
								<td>{timeAgo(account.last_used_at ?? '')}</td>
								{#each group.window_labels as label, index (`${account.id}:${label}:${index}`)}
									{@const window = windowAt(account, index)}
									<td class="num">
										{#if account.status === 'blocked' || account.status === 'disabled'}
											<span class="muted">&ndash;</span>
										{:else if window}
											{@const remain = 100 - window.pct}
											<span class={remainClass(remain)}>{remain}%</span> <span class="muted">{remainTime(window.reset ?? null)}</span>
										{:else}
											<span class="muted">&ndash;</span>
										{/if}
									</td>
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		{/each}
	{/if}

	{#if providerError}
		<p class="error-msg">{providerError}</p>
	{/if}
{/if}
