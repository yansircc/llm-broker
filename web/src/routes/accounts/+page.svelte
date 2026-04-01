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

	type SortKey = 'email' | 'status' | 'cell' | 'weight' | 'cooldown' | 'last_used' | `window:${number}`;
	type SortDirection = 'asc' | 'desc';

	interface SortState {
		key: SortKey;
		dir: SortDirection;
	}

	let accounts = $state<AccountListItem[]>([]);
	let providers = $state<ProviderOption[]>([]);
	let error = $state('');
	let providerError = $state('');
	let lastRefresh = $state('');
	let groupSorts = $state<Record<string, SortState>>({});

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

	function defaultSortState(): SortState {
		return { key: 'email', dir: 'asc' };
	}

	function defaultDirection(key: SortKey): SortDirection {
		switch (key) {
			case 'weight':
			case 'cooldown':
			case 'last_used':
				return 'desc';
			default:
				return 'asc';
		}
	}

	function sortStateFor(provider: string): SortState {
		return groupSorts[provider] ?? defaultSortState();
	}

	function setSort(provider: string, key: SortKey) {
		const current = sortStateFor(provider);
		groupSorts[provider] = current.key === key
			? { key, dir: current.dir === 'asc' ? 'desc' : 'asc' }
			: { key, dir: defaultDirection(key) };
	}

	function isSortActive(provider: string, key: SortKey): boolean {
		return sortStateFor(provider).key === key;
	}

	function sortIndicator(provider: string, key: SortKey): string {
		if (!isSortActive(provider, key)) return '';
		return sortStateFor(provider).dir === 'asc' ? '↑' : '↓';
	}

	function ariaSort(provider: string, key: SortKey): 'ascending' | 'descending' | 'none' {
		if (!isSortActive(provider, key)) return 'none';
		return sortStateFor(provider).dir === 'asc' ? 'ascending' : 'descending';
	}

	function windowSortKey(index: number): SortKey {
		return `window:${index}`;
	}

	function parseWindowSortIndex(key: SortKey): number | null {
		if (!key.startsWith('window:')) return null;
		const index = Number.parseInt(key.slice('window:'.length), 10);
		return Number.isFinite(index) ? index : null;
	}

	function timestamp(value: string | null | undefined): number | null {
		if (!value) return null;
		const result = new Date(value).getTime();
		return Number.isNaN(result) ? null : result;
	}

	function cellLabel(account: AccountListItem): string {
		return account.cell?.name ?? account.cell_id ?? 'legacy direct';
	}

	function statusRank(status: string): number {
		switch (status) {
			case 'active':
				return 0;
			case 'blocked':
				return 1;
			case 'error':
				return 2;
			case 'disabled':
				return 9;
			default:
				return 5;
		}
	}

	function windowRemain(account: AccountListItem, index: number): number | null {
		if (account.status === 'blocked' || account.status === 'disabled') {
			return null;
		}
		const window = windowAt(account, index);
		if (!window) return null;
		return 100 - window.pct;
	}

	function compareNumbers(left: number, right: number, dir: SortDirection): number {
		if (left === right) return 0;
		const diff = left - right;
		return dir === 'asc' ? diff : -diff;
	}

	function compareNullableNumbers(left: number | null, right: number | null, dir: SortDirection): number {
		if (left == null && right == null) return 0;
		if (left == null) return 1;
		if (right == null) return -1;
		return compareNumbers(left, right, dir);
	}

	function compareStrings(left: string, right: string, dir: SortDirection): number {
		const diff = left.localeCompare(right);
		return dir === 'asc' ? diff : -diff;
	}

	function compareDisabledLast(left: AccountListItem, right: AccountListItem): number {
		const leftDisabled = left.status === 'disabled';
		const rightDisabled = right.status === 'disabled';
		if (leftDisabled === rightDisabled) return 0;
		return leftDisabled ? 1 : -1;
	}

	function compareAccounts(left: AccountListItem, right: AccountListItem, sort: SortState): number {
		switch (sort.key) {
			case 'email':
				return compareStrings(left.email, right.email, sort.dir);
			case 'status':
				return compareNumbers(statusRank(left.status), statusRank(right.status), sort.dir);
			case 'cell':
				return compareStrings(cellLabel(left), cellLabel(right), sort.dir);
			case 'weight':
				return compareNumbers(left.weight, right.weight, sort.dir);
			case 'cooldown':
				return compareNullableNumbers(timestamp(left.cooldown_until), timestamp(right.cooldown_until), sort.dir);
			case 'last_used':
				return compareNullableNumbers(timestamp(left.last_used_at), timestamp(right.last_used_at), sort.dir);
			default: {
				const index = parseWindowSortIndex(sort.key);
				if (index == null) return 0;
				return compareNullableNumbers(windowRemain(left, index), windowRemain(right, index), sort.dir);
			}
		}
	}

	function sortAccounts(provider: string, items: AccountListItem[]): AccountListItem[] {
		const sort = sortStateFor(provider);
		return [...items].sort((left, right) => {
			const disabled = compareDisabledLast(left, right);
			if (disabled !== 0) return disabled;

			const primary = compareAccounts(left, right, sort);
			if (primary !== 0) return primary;

			return left.email.localeCompare(right.email);
		});
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
				if (!group!.window_labels[index]) {
					let lbl = window.label;
					if (window.sub_label) lbl += ` ${window.sub_label}`;
					group!.window_labels[index] = lbl;
				}
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
				existing.accounts = sortAccounts(existing.provider, existing.accounts);
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
			leftover.accounts = sortAccounts(leftover.provider, leftover.accounts);
			ordered.push(leftover);
		}
		return ordered;
	}

	function activeCount(items: AccountListItem[]): number {
		return items.filter((account) => account.status === 'active').length;
	}

	function availableCount(items: AccountListItem[], surface: 'native' | 'compat'): number {
		return items.filter((account) => surface === 'native' ? account.available_native : account.available_compat).length;
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
			<div class="sub">
				{group.accounts.length} total
				&middot; active {activeCount(group.accounts)}
				&middot; available(native) {availableCount(group.accounts, 'native')}
				&middot; available(compat) {availableCount(group.accounts, 'compat')}
			</div>
			{#if group.accounts.length === 0}
				<p class="muted">no {group.label} accounts</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th aria-sort={ariaSort(group.provider, 'email')}>
								<button type="button" class="link sort-link {isSortActive(group.provider, 'email') ? 'sort-active' : ''}" onclick={() => setSort(group.provider, 'email')}>
									account <span class="sort-indicator">{sortIndicator(group.provider, 'email')}</span>
								</button>
							</th>
							<th aria-sort={ariaSort(group.provider, 'status')}>
								<button type="button" class="link sort-link {isSortActive(group.provider, 'status') ? 'sort-active' : ''}" onclick={() => setSort(group.provider, 'status')}>
									status <span class="sort-indicator">{sortIndicator(group.provider, 'status')}</span>
								</button>
							</th>
							<th aria-sort={ariaSort(group.provider, 'cell')}>
								<button type="button" class="link sort-link {isSortActive(group.provider, 'cell') ? 'sort-active' : ''}" onclick={() => setSort(group.provider, 'cell')}>
									cell <span class="sort-indicator">{sortIndicator(group.provider, 'cell')}</span>
								</button>
							</th>
							<th aria-sort={ariaSort(group.provider, 'weight')}>
								<button type="button" class="link sort-link {isSortActive(group.provider, 'weight') ? 'sort-active' : ''}" onclick={() => setSort(group.provider, 'weight')}>
									weight <span class="sort-indicator">{sortIndicator(group.provider, 'weight')}</span>
								</button>
							</th>
							<th aria-sort={ariaSort(group.provider, 'cooldown')}>
								<button type="button" class="link sort-link {isSortActive(group.provider, 'cooldown') ? 'sort-active' : ''}" onclick={() => setSort(group.provider, 'cooldown')}>
									cooldown <span class="sort-indicator">{sortIndicator(group.provider, 'cooldown')}</span>
								</button>
							</th>
							<th aria-sort={ariaSort(group.provider, 'last_used')}>
								<button type="button" class="link sort-link {isSortActive(group.provider, 'last_used') ? 'sort-active' : ''}" onclick={() => setSort(group.provider, 'last_used')}>
									last used <span class="sort-indicator">{sortIndicator(group.provider, 'last_used')}</span>
								</button>
							</th>
							{#each group.window_labels as label, index (`${group.provider}:${label}:${index}`)}
								<th class="num" aria-sort={ariaSort(group.provider, windowSortKey(index))}>
									<button type="button" class="link sort-link sort-link-num {isSortActive(group.provider, windowSortKey(index)) ? 'sort-active' : ''}" onclick={() => setSort(group.provider, windowSortKey(index))}>
										{label} <span class="sort-indicator">{sortIndicator(group.provider, windowSortKey(index))}</span>
									</button>
								</th>
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
								<td>{account.weight}{#if account.weight_mode === 'auto'} <span class="muted">(auto)</span>{/if}</td>
								<Countdown until={account.cooldown_until} tag="td" variant="cooldown" />
								<td>{timeAgo(account.last_used_at ?? '')}</td>
								{#each group.window_labels as label, index (`${account.id}:${label}:${index}`)}
									{@const window = windowAt(account, index)}
									<td class="num">
										{#if account.status === 'blocked' || account.status === 'disabled'}
											<span class="muted">&ndash;</span>
										{:else if window}
											{@const remain = 100 - window.pct}
											<span class={remainClass(remain)}>{remain}%</span>{#if window.sub_pct != null}{@const subRemain = 100 - window.sub_pct}/<span class={remainClass(subRemain)}>{subRemain}%</span>{/if} <span class="muted">{remainTime(window.reset ?? null)}</span>
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

<style>
	.sort-link {
		color: inherit;
		text-decoration: none;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-weight: inherit;
	}

	.sort-link:hover {
		text-decoration: underline;
	}

	.sort-link-num {
		justify-content: flex-end;
		width: 100%;
	}

	.sort-active {
		font-weight: bold;
	}

	.sort-indicator {
		display: inline-block;
		min-width: 1ch;
		color: #888;
	}
</style>
