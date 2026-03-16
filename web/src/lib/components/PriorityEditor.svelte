<script lang="ts">
	import { api } from '$lib/api';

	let { accountId, weight, weightMode, autoWeight, onupdate }:
		{ accountId: string; weight: number; weightMode: string; autoWeight: number; onupdate: (mode: string, weight: number) => void } = $props();

	let isAuto = $derived(weightMode === 'auto');
	let input = $state('');
	let saving = $state(false);
	let error = $state('');

	$effect(() => { input = String(weight); });

	async function save() {
		const val = parseInt(input);
		if (isNaN(val)) return;
		saving = true;
		error = '';
		try {
			await api(`/accounts/${accountId}/weight`, {
				method: 'POST',
				body: JSON.stringify({ mode: 'manual', weight: val })
			});
			onupdate('manual', val);
		} catch (e: any) {
			error = e.message;
		} finally {
			saving = false;
		}
	}

	async function switchToAuto() {
		error = '';
		try {
			await api(`/accounts/${accountId}/weight`, {
				method: 'POST',
				body: JSON.stringify({ mode: 'auto' })
			});
			onupdate('auto', weight);
		} catch (e: any) {
			error = e.message;
		}
	}

	async function switchToManual() {
		error = '';
		try {
			await api(`/accounts/${accountId}/weight`, {
				method: 'POST',
				body: JSON.stringify({ mode: 'manual', weight })
			});
			onupdate('manual', weight);
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

{#if isAuto}
	<span class="g">auto</span> <span class="muted">(weight: {autoWeight})</span>
	<button class="link" style="font-size:12px;margin-left:6px" onclick={switchToManual}>[switch to manual]</button>
{:else}
	<input
		class="pri-edit"
		type="text"
		bind:value={input}
		onkeydown={(e) => { if (e.key === 'Enter') save(); }}
	>
	<button class="link" style="font-size:12px" onclick={save} disabled={saving}>
		{saving ? 'saving...' : '[save]'}
	</button>
	<button class="link" style="font-size:12px;margin-left:6px" onclick={switchToAuto}>[switch to auto]</button>
{/if}
<div class="hint muted">higher weight = higher pick probability</div>
{#if error}<span class="error-msg">{error}</span>{/if}

<style>
	.pri-edit {
		width: 40px;
		font: 13px monospace;
		border: 1px solid #ccc;
		padding: 0 4px;
	}
	.hint {
		font-size: 12px;
		margin-top: 4px;
	}
	@media (prefers-color-scheme: dark) {
		.pri-edit { background: #252525; color: #ccc; border-color: #555; }
	}
</style>
