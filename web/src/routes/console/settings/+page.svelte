<script lang="ts">
	import { api } from '$lib/api';

	interface RuntimeSpec {
		key: string;
		group: string;
		label: string;
		kind: string;
		restart_required: boolean;
		help?: string;
		default: unknown;
	}

	interface RuntimeSettingView {
		key: string;
		value: unknown;
		updated_at: string;
		updated_by: string;
	}

	interface IntegrationView {
		id: string;
		kind: string;
		provider: string;
		display_name: string;
		enabled: boolean;
		priority: number;
		config: Record<string, unknown>;
		secret_configured: boolean;
		secret_fingerprint: string;
		updated_at: string;
		updated_by: string;
	}

	interface SettingsResponse {
		runtime_specs: RuntimeSpec[];
		runtime_settings: Record<string, RuntimeSettingView>;
		billing_settings: Record<string, string>;
		integrations: IntegrationView[];
	}

	type IntegrationDraft = {
		kind: string;
		provider: string;
		display_name: string;
		enabled: boolean;
		priority: number;
		configText: string;
		secretsText: string;
	};

	const billingLabels: Record<string, string> = {
		cny_to_usd_rate_micros: '人民币兑换 USD 比例',
		referral_new_user_bonus_micros: '受邀注册奖励',
		referral_inviter_bonus_micros: '邀请人付费后奖励',
		low_balance_alert_threshold_micros: '低余额提醒阈值'
	};

	let data = $state<SettingsResponse | null>(null);
	let runtimeDraft = $state<Record<string, unknown>>({});
	let billingDraft = $state<Record<string, string>>({});
	let integrationDrafts = $state<Record<string, IntegrationDraft>>({});
	let newIntegration = $state<IntegrationDraft>({
		kind: 'payment',
		provider: 'zpay',
		display_name: '7pay / ZPay',
		enabled: false,
		priority: 100,
		configText: '{\n  "pid": "",\n  "cid": ""\n}',
		secretsText: ''
	});
	let testEmailTo = $state('');
	let error = $state('');
	let message = $state('');
	let loading = $state(false);
	let saving = $state('');

	$effect(() => {
		loadSettings();
	});

	async function loadSettings() {
		loading = true;
		error = '';
		try {
			data = await api<SettingsResponse>('/settings');
			runtimeDraft = {};
			for (const spec of data.runtime_specs) {
				runtimeDraft[spec.key] = data.runtime_settings[spec.key]?.value ?? spec.default;
			}
			billingDraft = { ...data.billing_settings };
			integrationDrafts = {};
			for (const integration of data.integrations) {
				integrationDrafts[integration.id] = {
					kind: integration.kind,
					provider: integration.provider,
					display_name: integration.display_name,
					enabled: integration.enabled,
					priority: integration.priority,
					configText: JSON.stringify(integration.config ?? {}, null, 2),
					secretsText: ''
				};
			}
		} catch (e: any) {
			error = e.message || '加载设置失败';
		} finally {
			loading = false;
		}
	}

	async function saveSettings() {
		saving = 'settings';
		error = '';
		message = '';
		try {
			const settings: Record<string, unknown> = {};
			for (const spec of data?.runtime_specs ?? []) {
				settings[spec.key] = normalizeRuntimeValue(spec, runtimeDraft[spec.key]);
			}
			await api('/settings', {
				method: 'PATCH',
				body: JSON.stringify({ settings, billing_settings: billingDraft })
			});
			message = '设置已保存';
			await loadSettings();
		} catch (e: any) {
			error = e.message || '保存设置失败';
		} finally {
			saving = '';
		}
	}

	async function saveIntegration(id: string) {
		const draft = integrationDrafts[id];
		if (!draft) return;
		saving = id;
		error = '';
		message = '';
		try {
			await api(`/integrations/${id}`, {
				method: 'PATCH',
				body: JSON.stringify(integrationPayload(draft))
			});
			message = '集成已保存';
			await loadSettings();
		} catch (e: any) {
			error = e.message || '保存集成失败';
		} finally {
			saving = '';
		}
	}

	async function createIntegration() {
		saving = 'new';
		error = '';
		message = '';
		try {
			await api('/integrations', {
				method: 'POST',
				body: JSON.stringify(integrationPayload(newIntegration))
			});
			message = '集成已创建';
			newIntegration.secretsText = '';
			await loadSettings();
		} catch (e: any) {
			error = e.message || '创建集成失败';
		} finally {
			saving = '';
		}
	}

	async function testIntegration(integration: IntegrationView) {
		saving = `test:${integration.id}`;
		error = '';
		message = '';
		try {
			const body = integration.kind === 'email' ? { to: testEmailTo } : {};
			await api(`/integrations/${integration.id}/test`, { method: 'POST', body: JSON.stringify(body) });
			message = '测试通过';
		} catch (e: any) {
			error = e.message || '测试失败';
		} finally {
			saving = '';
		}
	}

	function integrationPayload(draft: IntegrationDraft) {
		const payload: Record<string, unknown> = {
			kind: draft.kind,
			provider: draft.provider,
			display_name: draft.display_name,
			enabled: draft.enabled,
			priority: Number(draft.priority),
			config: parseJSON(draft.configText || '{}')
		};
		if (draft.secretsText.trim()) {
			payload.secrets = parseJSON(draft.secretsText);
		}
		return payload;
	}

	function parseJSON(raw: string) {
		try {
			return JSON.parse(raw);
		} catch {
			throw new Error('JSON 格式不正确');
		}
	}

	function normalizeRuntimeValue(spec: RuntimeSpec, value: unknown) {
		if (spec.kind === 'bool') return Boolean(value);
		if (spec.kind === 'int' || spec.kind === 'duration_ms') return Number(value);
		return String(value ?? '');
	}

	function groupedSpecs(group: string) {
		return data?.runtime_specs.filter((spec) => spec.group === group) ?? [];
	}

	function setProviderTemplate(provider: string) {
		newIntegration.provider = provider;
		if (provider === 'zpay') {
			newIntegration.display_name = '7pay / ZPay';
			newIntegration.configText = '{\n  "pid": "",\n  "cid": ""\n}';
			newIntegration.secretsText = '{\n  "key": ""\n}';
		} else if (provider === 'smtp') {
			newIntegration.display_name = 'SMTP';
			newIntegration.configText = '{\n  "addr": "",\n  "username": "",\n  "from": ""\n}';
			newIntegration.secretsText = '{\n  "password": ""\n}';
		} else if (provider === 'resend') {
			newIntegration.display_name = 'Resend';
			newIntegration.configText = '{\n  "from": ""\n}';
			newIntegration.secretsText = '{\n  "api_key": ""\n}';
		} else if (provider === 'turnstile') {
			newIntegration.display_name = 'Cloudflare Turnstile';
			newIntegration.configText = '{\n  "site_key": ""\n}';
			newIntegration.secretsText = '{\n  "secret_key": ""\n}';
		}
	}
</script>

<div class="page-header">
	<div>
		<div class="eyebrow">配置中心</div>
		<h1>系统设置</h1>
		<p class="lede">管理运行期配置、计费参数、支付渠道和邮件渠道。密钥保存后只显示指纹，不可读回。</p>
	</div>
	<div class="page-actions">
		<button class="secondary-btn fit" onclick={loadSettings}>刷新</button>
		<button class="primary-btn fit" onclick={saveSettings} disabled={saving === 'settings'}>{saving === 'settings' ? '保存中...' : '保存设置'}</button>
	</div>
</div>

{#if error}
	<p class="error-msg">{error}</p>
{/if}
{#if message}
	<p class="success-msg">{message}</p>
{/if}

{#if loading || !data}
	<p class="loading">正在加载设置...</p>
{:else}
	<h2>常规设置</h2>
	<div class="settings-grid">
		{#each groupedSpecs('general') as spec}
			<label class="field-card">
				<span>{spec.label}</span>
				<input bind:value={runtimeDraft[spec.key]} />
				<small>{spec.help || spec.key}</small>
			</label>
		{/each}
	</div>

	<h2>计费参数</h2>
	<div class="settings-grid">
		{#each Object.entries(billingDraft) as [key, value]}
			<label class="field-card">
				<span>{billingLabels[key] || key}</span>
				<input bind:value={billingDraft[key]} />
				<small>{key}</small>
			</label>
		{/each}
	</div>

	<h2>支付与邮件集成</h2>
	<div class="stack">
		{#each data.integrations as integration}
			{@const draft = integrationDrafts[integration.id]}
			<section class="panel">
				<div class="panel-head">
					<div>
						<h3>{integration.display_name}</h3>
						<p class="sub">{integration.kind} / {integration.provider} · priority {integration.priority}</p>
					</div>
					<div class="page-actions">
						<span class:ok={integration.enabled} class="status-pill">{integration.enabled ? '启用' : '停用'}</span>
						{#if integration.secret_configured}
							<span class="mono muted">secret {integration.secret_fingerprint}</span>
						{/if}
					</div>
				</div>
				<div class="settings-grid">
					<label class="field-card">
						<span>名称</span>
						<input bind:value={draft.display_name} />
					</label>
					<label class="field-card">
						<span>优先级</span>
						<input type="number" bind:value={draft.priority} />
					</label>
					<label class="field-card check">
						<input type="checkbox" bind:checked={draft.enabled} />
						<span>启用</span>
					</label>
				</div>
				<div class="settings-grid wide">
					<label class="field-card">
						<span>Config JSON</span>
						<textarea bind:value={draft.configText}></textarea>
					</label>
					<label class="field-card">
						<span>Secrets JSON</span>
						<textarea bind:value={draft.secretsText} placeholder="留空表示不修改；填写 JSON 后会覆盖密钥"></textarea>
					</label>
				</div>
				<div class="page-actions">
					<button class="primary-btn fit" onclick={() => saveIntegration(integration.id)} disabled={saving === integration.id}>
						{saving === integration.id ? '保存中...' : '保存集成'}
					</button>
					{#if integration.kind === 'email'}
						<input class="inline-input" placeholder="测试收件邮箱" bind:value={testEmailTo} />
					{/if}
					<button class="secondary-btn fit" onclick={() => testIntegration(integration)} disabled={saving === `test:${integration.id}`}>
						{saving === `test:${integration.id}` ? '测试中...' : '测试'}
					</button>
				</div>
			</section>
		{/each}
	</div>

	<section class="panel">
		<div class="panel-head">
			<div>
				<h3>新增集成</h3>
				<p class="sub">支持 zpay、smtp、resend、turnstile。Stripe 需要接入 adapter 后再启用。</p>
			</div>
		</div>
		<div class="settings-grid">
			<label class="field-card">
				<span>类型</span>
				<select bind:value={newIntegration.kind}>
					<option value="payment">payment</option>
					<option value="email">email</option>
					<option value="security">security</option>
				</select>
			</label>
			<label class="field-card">
				<span>Provider</span>
				<select bind:value={newIntegration.provider} onchange={(e) => setProviderTemplate((e.target as HTMLSelectElement).value)}>
					<option value="zpay">zpay</option>
					<option value="smtp">smtp</option>
					<option value="resend">resend</option>
					<option value="turnstile">turnstile</option>
					<option value="stripe">stripe</option>
				</select>
			</label>
			<label class="field-card">
				<span>名称</span>
				<input bind:value={newIntegration.display_name} />
			</label>
			<label class="field-card">
				<span>优先级</span>
				<input type="number" bind:value={newIntegration.priority} />
			</label>
			<label class="field-card check">
				<input type="checkbox" bind:checked={newIntegration.enabled} />
				<span>启用</span>
			</label>
		</div>
		<div class="settings-grid wide">
			<label class="field-card">
				<span>Config JSON</span>
				<textarea bind:value={newIntegration.configText}></textarea>
			</label>
			<label class="field-card">
				<span>Secrets JSON</span>
				<textarea bind:value={newIntegration.secretsText}></textarea>
			</label>
		</div>
		<button class="primary-btn fit" onclick={createIntegration} disabled={saving === 'new'}>{saving === 'new' ? '创建中...' : '创建集成'}</button>
	</section>

	<h2>高级运行参数</h2>
	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					<th>配置</th>
					<th>值</th>
					<th>说明</th>
				</tr>
			</thead>
			<tbody>
				{#each groupedSpecs('advanced') as spec}
					<tr>
						<td>
							<div>{spec.label}</div>
							<div class="mono muted">{spec.key}</div>
						</td>
						<td>
							{#if spec.kind === 'bool'}
								<input type="checkbox" bind:checked={runtimeDraft[spec.key]} />
							{:else if spec.kind.startsWith('enum:')}
								<select bind:value={runtimeDraft[spec.key]}>
									{#each spec.kind.replace('enum:', '').split(',') as item}
										<option value={item}>{item}</option>
									{/each}
								</select>
							{:else}
								<input class="wide-input" type={spec.kind === 'string' || spec.kind === 'url' ? 'text' : 'number'} bind:value={runtimeDraft[spec.key]} />
							{/if}
						</td>
						<td>{spec.restart_required ? '保存后下次进程启动生效' : '保存后立即读取'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<style>
	.settings-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		gap: 14px;
		margin: 14px 0 24px;
	}
	.settings-grid.wide {
		grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
	}
	.field-card {
		display: flex;
		flex-direction: column;
		gap: 8px;
		border: 1px solid var(--border);
		background: rgba(255,255,255,0.035);
		border-radius: 8px;
		padding: 14px;
	}
	.field-card span {
		font-weight: 600;
	}
	.field-card small {
		color: var(--faint);
	}
	.field-card.check {
		flex-direction: row;
		align-items: center;
	}
	input, select, textarea {
		min-height: 38px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: rgba(0,0,0,0.28);
		color: var(--text);
		padding: 8px 10px;
	}
	textarea {
		min-height: 128px;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
		font-size: 12px;
	}
	.panel {
		border: 1px solid var(--border);
		border-radius: 8px;
		background: rgba(255,255,255,0.025);
		padding: 18px;
	}
	.panel + .panel {
		margin-top: 14px;
	}
	.panel-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
		margin-bottom: 14px;
	}
	.panel-head h3 {
		margin: 0;
	}
	.stack {
		margin-bottom: 24px;
	}
	.status-pill {
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 4px 9px;
		color: var(--faint);
	}
	.status-pill.ok {
		border-color: color-mix(in srgb, var(--accent) 45%, transparent);
		color: var(--accent);
	}
	.inline-input {
		width: min(260px, 100%);
	}
	.wide-input {
		width: min(360px, 100%);
	}
	.success-msg {
		border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
		background: color-mix(in srgb, var(--accent) 10%, transparent);
		border-radius: 8px;
		padding: 12px 14px;
		color: var(--accent);
	}
</style>
