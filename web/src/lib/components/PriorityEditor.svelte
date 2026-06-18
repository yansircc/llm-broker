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
	<span class="g">自动</span> <span class="muted">权重 {autoWeight}</span>
	<button class="link compact" onclick={switchToManual}>切换手动</button>
{:else}
	<div class="priority-row">
		<input
			class="pri-edit"
			type="text"
			bind:value={input}
			onkeydown={(e) => { if (e.key === 'Enter') save(); }}
		>
		<button class="link compact" onclick={save} disabled={saving}>
			{saving ? '保存中...' : '保存'}
		</button>
		<button class="link compact" onclick={switchToAuto}>切换自动</button>
	</div>
{/if}
<div class="hint muted">权重越高，被调度选中的概率越高</div>
{#if error}<span class="error-msg">{error}</span>{/if}

<style>
	.priority-row {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.pri-edit {
		width: 64px;
		max-width: 64px;
		padding: 6px 8px;
		font: 13px ui-monospace, SFMono-Regular, Menlo, monospace;
	}

	button.compact {
		padding: 5px 8px;
		font-size: 12px;
	}

	.hint {
		font-size: 12px;
		margin-top: 4px;
	}
</style>
