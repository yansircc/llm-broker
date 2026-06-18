<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import type { CustomerApiKey, CustomerApiKeyCreated } from '$lib/customer-types';
	import { fmtDate, timeAgo } from '$lib/format';

	let keys = $state<CustomerApiKey[]>([]);
	let created = $state<CustomerApiKeyCreated | null>(null);
	let name = $state('');
	let error = $state('');
	let loading = $state(false);
	let creating = $state(false);

	$effect(() => {
		loadKeys();
	});

	async function loadKeys() {
		loading = true;
		error = '';
		try {
			keys = await customerApi<CustomerApiKey[]>('/keys');
		} catch (e: any) {
			error = e.message || 'failed to load keys';
		} finally {
			loading = false;
		}
	}

	async function createKey() {
		const keyName = name.trim() || 'default';
		creating = true;
		error = '';
		try {
			created = await customerApi<CustomerApiKeyCreated>('/keys', {
				method: 'POST',
				body: JSON.stringify({ name: keyName })
			});
			name = '';
			keys = [created, ...keys.filter((key) => key.id !== created?.id)];
		} catch (e: any) {
			error = e.message || 'failed to create key';
		} finally {
			creating = false;
		}
	}
</script>

<span class="refresh"><button class="link" onclick={loadKeys}>[refresh]</button></span>
<h2>api keys</h2>

<div class="bar">
	<input type="text" placeholder="key name" bind:value={name} disabled={creating} style="width:180px;max-width:180px;margin-right:6px;">
	<button class="link" onclick={createKey} disabled={creating}>{creating ? '[creating...]' : '[create]'}</button>
</div>

{#if created}
	<div class="bar">
		<div><span class="g">&#10003; created</span> {created.name} token: <span style="user-select:all;">{created.token}</span> <button class="link" onclick={() => { created = null; }}>[dismiss]</button></div>
		<div class="r" style="font-size:11px;margin-top:2px;">copy now - this token will not be shown again</div>
	</div>
{/if}

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading keys...</p>
{:else if keys.length === 0}
	<p class="muted">no keys</p>
{:else}
	<table>
		<thead>
			<tr><th>name</th><th>prefix</th><th>status</th><th>created</th><th>last used</th></tr>
		</thead>
		<tbody>
			{#each keys as key (key.id)}
				<tr>
					<td>{key.name}</td>
					<td class="muted">{key.prefix ?? '-'}</td>
					<td>{key.status}</td>
					<td>{fmtDate(key.created_at)}</td>
					<td>{key.last_used_at ? timeAgo(key.last_used_at) : '-'}</td>
				</tr>
			{/each}
		</tbody>
	</table>
{/if}
