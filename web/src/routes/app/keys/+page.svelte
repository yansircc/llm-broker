<script lang="ts">
	import { customerApi } from '$lib/customer-api';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import type { CustomerApiKey, CustomerApiKeyCreated } from '$lib/customer-types';
	import { fmtDate, timeAgo } from '$lib/format';

	let keys = $state<CustomerApiKey[]>([]);
	let created = $state<CustomerApiKeyCreated | null>(null);
	let name = $state('');
	let error = $state('');
	let loading = $state(false);
	let creating = $state(false);
	let copied = $state(false);

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
			copied = false;
		} catch (e: any) {
			error = e.message || 'failed to create key';
		} finally {
			creating = false;
		}
	}

	async function copyToken() {
		if (!created?.token) return;
		await navigator.clipboard.writeText(created.token);
		copied = true;
		setTimeout(() => { copied = false; }, 2000);
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">access</div>
		<h1>API Keys</h1>
		<p class="lede">Create customer relay keys for OpenAI Responses and compatible clients.</p>
	</div>
	<div class="page-actions">
		<button class="link" onclick={loadKeys}>refresh</button>
	</div>
</div>

<section class="panel form-panel">
	<div class="section-header flush">
		<h2>Create Key</h2>
	</div>
	<div class="form-row">
		<label for="key-name">name</label>
		<input id="key-name" type="text" placeholder="default" bind:value={name} disabled={creating}>
		<button class="primary-btn" onclick={createKey} disabled={creating}>{creating ? 'Creating...' : 'Create'}</button>
	</div>
</section>

{#if created}
	<div class="panel">
		<div class="section-header flush">
			<div>
				<h2>New Token</h2>
				<p class="hint">This token is shown once.</p>
			</div>
			<div class="page-actions">
				<button class="link" onclick={copyToken}>{copied ? 'copied' : 'copy'}</button>
				<button class="link" onclick={() => { created = null; copied = false; }}>dismiss</button>
			</div>
		</div>
		<div class="copy-value mono">{created.token}</div>
	</div>
{/if}

{#if error}
	<p class="error-msg">{error}</p>
{:else if loading}
	<p class="loading">loading keys...</p>
{:else if keys.length === 0}
	<p class="muted">no keys</p>
{:else}
	<div class="table-wrap">
		<table>
			<thead>
				<tr><th>name</th><th>prefix</th><th>status</th><th>created</th><th>last used</th></tr>
			</thead>
			<tbody>
				{#each keys as key (key.id)}
					<tr>
						<td>{key.name}</td>
						<td class="muted mono">{key.prefix ?? '-'}</td>
						<td><StatusBadge status={key.status} /></td>
						<td>{fmtDate(key.created_at)}</td>
						<td>{key.last_used_at ? timeAgo(key.last_used_at) : '-'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
