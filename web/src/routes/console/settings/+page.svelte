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
		zpayPid: string;
		zpayCid: string;
		zpayKey: string;
		smtpAddr: string;
		smtpUsername: string;
		smtpFrom: string;
		smtpPassword: string;
		resendFrom: string;
		resendApiKey: string;
		turnstileSiteKey: string;
		turnstileSecretKey: string;
	};

	const billingLabels: Record<string, string> = {
		cny_to_usd_rate_micros: '人民币兑换 USD 比例',
		referral_new_user_bonus_micros: '受邀注册奖励',
		referral_inviter_bonus_micros: '邀请人付费后奖励',
		low_balance_alert_threshold_micros: '低余额提醒阈值'
	};
	const dayMs = 24 * 60 * 60 * 1000;

	let data = $state<SettingsResponse | null>(null);
	let runtimeDraft = $state<Record<string, unknown>>({});
	let billingDraft = $state<Record<string, string>>({});
	let integrationDrafts = $state<Record<string, IntegrationDraft>>({});
	let newZPay = $state(blankDraft('payment', 'zpay', '7pay / ZPay'));
	let newSMTP = $state(blankDraft('email', 'smtp', 'SMTP'));
	let newResend = $state(blankDraft('email', 'resend', 'Resend'));
	let newTurnstile = $state(blankDraft('security', 'turnstile', 'Cloudflare Turnstile'));
	let testEmailTo = $state('');
	let error = $state('');
	let message = $state('');
	let loading = $state(false);
	let saving = $state('');

	$effect(() => {
		loadSettings();
	});

	function blankDraft(kind: string, provider: string, displayName: string): IntegrationDraft {
		return {
			kind,
			provider,
			display_name: displayName,
			enabled: false,
			priority: 100,
			zpayPid: '',
			zpayCid: '',
			zpayKey: '',
			smtpAddr: '',
			smtpUsername: '',
			smtpFrom: '',
			smtpPassword: '',
			resendFrom: '',
			resendApiKey: '',
			turnstileSiteKey: '',
			turnstileSecretKey: ''
		};
	}

	async function loadSettings() {
		loading = true;
		error = '';
		try {
			data = await api<SettingsResponse>('/settings');
			runtimeDraft = {};
			for (const spec of data.runtime_specs) {
				runtimeDraft[spec.key] = runtimeDraftValue(spec, data.runtime_settings[spec.key]?.value ?? spec.default);
			}
			billingDraft = { ...data.billing_settings };
			integrationDrafts = {};
			for (const integration of data.integrations) {
				integrationDrafts[integration.id] = draftFromIntegration(integration);
			}
		} catch (e: any) {
			error = e.message || '加载设置失败';
		} finally {
			loading = false;
		}
	}

	function draftFromIntegration(integration: IntegrationView): IntegrationDraft {
		const draft = blankDraft(integration.kind, integration.provider, integration.display_name);
		draft.enabled = integration.enabled;
		draft.priority = integration.priority;
		draft.zpayPid = stringConfig(integration, 'pid');
		draft.zpayCid = stringConfig(integration, 'cid');
		draft.smtpAddr = stringConfig(integration, 'addr');
		draft.smtpUsername = stringConfig(integration, 'username');
		draft.smtpFrom = stringConfig(integration, 'from');
		draft.resendFrom = stringConfig(integration, 'from');
		draft.turnstileSiteKey = stringConfig(integration, 'site_key');
		return draft;
	}

	function stringConfig(integration: IntegrationView, key: string): string {
		const value = integration.config?.[key];
		return value == null ? '' : String(value);
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
			clearSecrets(draft);
			await loadSettings();
		} catch (e: any) {
			error = e.message || '保存集成失败';
		} finally {
			saving = '';
		}
	}

	async function createIntegration(draft: IntegrationDraft, key: string) {
		saving = key;
		error = '';
		message = '';
		try {
			await api('/integrations', {
				method: 'POST',
				body: JSON.stringify(integrationPayload(draft))
			});
			message = '集成已创建';
			resetNewDraft(draft);
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
			config: providerConfig(draft)
		};
		const secrets = providerSecrets(draft);
		if (Object.keys(secrets).length > 0) {
			payload.secrets = secrets;
		}
		return payload;
	}

	function providerConfig(draft: IntegrationDraft): Record<string, string> {
		if (draft.provider === 'zpay') return cleanObject({ pid: draft.zpayPid, cid: draft.zpayCid });
		if (draft.provider === 'smtp') return cleanObject({ addr: draft.smtpAddr, username: draft.smtpUsername, from: draft.smtpFrom });
		if (draft.provider === 'resend') return cleanObject({ from: draft.resendFrom });
		if (draft.provider === 'turnstile') return cleanObject({ site_key: draft.turnstileSiteKey });
		return {};
	}

	function providerSecrets(draft: IntegrationDraft): Record<string, string> {
		if (draft.provider === 'zpay') return cleanObject({ key: draft.zpayKey });
		if (draft.provider === 'smtp') return cleanObject({ password: draft.smtpPassword });
		if (draft.provider === 'resend') return cleanObject({ api_key: draft.resendApiKey });
		if (draft.provider === 'turnstile') return cleanObject({ secret_key: draft.turnstileSecretKey });
		return {};
	}

	function cleanObject(input: Record<string, string>): Record<string, string> {
		const out: Record<string, string> = {};
		for (const [key, value] of Object.entries(input)) {
			const v = String(value ?? '').trim();
			if (v !== '') out[key] = v;
		}
		return out;
	}

	function clearSecrets(draft: IntegrationDraft) {
		draft.zpayKey = '';
		draft.smtpPassword = '';
		draft.resendApiKey = '';
		draft.turnstileSecretKey = '';
	}

	function resetNewDraft(draft: IntegrationDraft) {
		const next = blankDraft(draft.kind, draft.provider, draft.display_name);
		Object.assign(draft, next);
	}

	function normalizeRuntimeValue(spec: RuntimeSpec, value: unknown) {
		if (spec.kind === 'bool') return Boolean(value);
		if (isDayDuration(spec)) return Number(value) * dayMs;
		if (spec.kind === 'int' || spec.kind === 'duration_ms') return Number(value);
		return String(value ?? '');
	}

	function runtimeDraftValue(spec: RuntimeSpec, value: unknown) {
		if (!isDayDuration(spec)) return value;
		const ms = Number(value);
		if (!Number.isFinite(ms)) return '';
		const days = ms / dayMs;
		return Number.isInteger(days) ? days : Number(days.toFixed(4));
	}

	function isDayDuration(spec: RuntimeSpec) {
		return spec.key === 'customer_session_ttl_ms';
	}

	function groupedSpecs(group: string) {
		return data?.runtime_specs.filter((spec) => spec.group === group) ?? [];
	}

	function integrationsBy(kind: string) {
		return data?.integrations.filter((integration) => integration.kind === kind) ?? [];
	}

	function providerLabel(provider: string) {
		if (provider === 'zpay') return '7pay / ZPay';
		if (provider === 'smtp') return 'SMTP';
		if (provider === 'resend') return 'Resend';
		if (provider === 'turnstile') return 'Cloudflare Turnstile';
		return provider;
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
				{#if isDayDuration(spec)}
					<div class="input-with-unit">
						<input type="number" min="1" step="1" bind:value={runtimeDraft[spec.key]} />
						<span>天</span>
					</div>
					<small>{spec.key}，保存时转换为毫秒</small>
				{:else}
					<input bind:value={runtimeDraft[spec.key]} />
					<small>{spec.help || spec.key}</small>
				{/if}
			</label>
		{/each}
	</div>

	<h2>计费参数</h2>
	<div class="settings-grid">
		{#each Object.entries(billingDraft) as [key]}
			<label class="field-card">
				<span>{billingLabels[key] || key}</span>
				<input bind:value={billingDraft[key]} />
				<small>{key}</small>
			</label>
		{/each}
	</div>

	<h2>支付渠道</h2>
	<div class="stack">
		{#each integrationsBy('payment') as integration}
			{@const draft = integrationDrafts[integration.id]}
			{#if draft}
				<section class="panel">
					<div class="panel-head">
						<div>
							<h3>{integration.display_name || providerLabel(integration.provider)}</h3>
							<p class="sub">{providerLabel(integration.provider)} · priority {integration.priority}</p>
						</div>
						{@render IntegrationStatus(integration)}
					</div>
					{@render CommonIntegrationFields(draft)}
					{#if integration.provider === 'zpay'}
						{@render ZPayFields(draft, true, integration.secret_configured)}
					{:else}
						<p class="muted">该支付 provider 暂未提供可视化配置表单。</p>
					{/if}
					<div class="page-actions">
						<button class="primary-btn fit" onclick={() => saveIntegration(integration.id)} disabled={saving === integration.id}>{saving === integration.id ? '保存中...' : '保存支付渠道'}</button>
						<button class="secondary-btn fit" onclick={() => testIntegration(integration)} disabled={saving === `test:${integration.id}`}>{saving === `test:${integration.id}` ? '测试中...' : '测试配置'}</button>
					</div>
				</section>
			{/if}
		{/each}

		<section class="panel">
			<div class="panel-head">
				<div>
					<h3>新增 7pay / ZPay</h3>
					<p class="sub">人民币扫码支付，成功后按当前兑换比例入账 USD 额度。</p>
				</div>
			</div>
			{@render CommonIntegrationFields(newZPay)}
			{@render ZPayFields(newZPay, false, false)}
			<button class="primary-btn fit" onclick={() => createIntegration(newZPay, 'new:zpay')} disabled={saving === 'new:zpay'}>{saving === 'new:zpay' ? '创建中...' : '新增 7pay'}</button>
		</section>

		<section class="panel disabled-panel">
			<div class="panel-head">
				<div>
					<h3>Stripe</h3>
					<p class="sub">Stripe adapter 接入后会在这里配置 secret key 和 webhook secret。</p>
				</div>
				<span class="status-pill">待接入</span>
			</div>
		</section>
	</div>

	<h2>邮件渠道</h2>
	<div class="stack">
		{#each integrationsBy('email') as integration}
			{@const draft = integrationDrafts[integration.id]}
			{#if draft}
				<section class="panel">
					<div class="panel-head">
						<div>
							<h3>{integration.display_name || providerLabel(integration.provider)}</h3>
							<p class="sub">{providerLabel(integration.provider)} · priority {integration.priority}</p>
						</div>
						{@render IntegrationStatus(integration)}
					</div>
					{@render CommonIntegrationFields(draft)}
					{#if integration.provider === 'smtp'}
						{@render SMTPFields(draft, true, integration.secret_configured)}
					{:else if integration.provider === 'resend'}
						{@render ResendFields(draft, true, integration.secret_configured)}
					{:else}
						<p class="muted">该邮件 provider 暂未提供可视化配置表单。</p>
					{/if}
					<div class="page-actions">
						<button class="primary-btn fit" onclick={() => saveIntegration(integration.id)} disabled={saving === integration.id}>{saving === integration.id ? '保存中...' : '保存邮件渠道'}</button>
						<input class="inline-input" placeholder="测试收件邮箱" bind:value={testEmailTo} />
						<button class="secondary-btn fit" onclick={() => testIntegration(integration)} disabled={saving === `test:${integration.id}`}>{saving === `test:${integration.id}` ? '测试中...' : '发送测试'}</button>
					</div>
				</section>
			{/if}
		{/each}

		<section class="panel">
			<div class="panel-head">
				<div>
					<h3>新增 SMTP</h3>
					<p class="sub">通用 SMTP 发信配置。</p>
				</div>
			</div>
			{@render CommonIntegrationFields(newSMTP)}
			{@render SMTPFields(newSMTP, false, false)}
			<button class="primary-btn fit" onclick={() => createIntegration(newSMTP, 'new:smtp')} disabled={saving === 'new:smtp'}>{saving === 'new:smtp' ? '创建中...' : '新增 SMTP'}</button>
		</section>

		<section class="panel">
			<div class="panel-head">
				<div>
					<h3>新增 Resend</h3>
					<p class="sub">通过 Resend API 发信。</p>
				</div>
			</div>
			{@render CommonIntegrationFields(newResend)}
			{@render ResendFields(newResend, false, false)}
			<button class="primary-btn fit" onclick={() => createIntegration(newResend, 'new:resend')} disabled={saving === 'new:resend'}>{saving === 'new:resend' ? '创建中...' : '新增 Resend'}</button>
		</section>
	</div>

	<h2>安全验证</h2>
	<div class="stack">
		{#each integrationsBy('security') as integration}
			{@const draft = integrationDrafts[integration.id]}
			{#if draft}
				<section class="panel">
					<div class="panel-head">
						<div>
							<h3>{integration.display_name || providerLabel(integration.provider)}</h3>
							<p class="sub">{providerLabel(integration.provider)} · priority {integration.priority}</p>
						</div>
						{@render IntegrationStatus(integration)}
					</div>
					{@render CommonIntegrationFields(draft)}
					{#if integration.provider === 'turnstile'}
						{@render TurnstileFields(draft, true, integration.secret_configured)}
					{:else}
						<p class="muted">该安全 provider 暂未提供可视化配置表单。</p>
					{/if}
					<button class="primary-btn fit" onclick={() => saveIntegration(integration.id)} disabled={saving === integration.id}>{saving === integration.id ? '保存中...' : '保存安全配置'}</button>
				</section>
			{/if}
		{/each}

		<section class="panel">
			<div class="panel-head">
				<div>
					<h3>新增 Turnstile</h3>
					<p class="sub">Cloudflare Turnstile 人机验证。</p>
				</div>
			</div>
			{@render CommonIntegrationFields(newTurnstile)}
			{@render TurnstileFields(newTurnstile, false, false)}
			<button class="primary-btn fit" onclick={() => createIntegration(newTurnstile, 'new:turnstile')} disabled={saving === 'new:turnstile'}>{saving === 'new:turnstile' ? '创建中...' : '新增 Turnstile'}</button>
		</section>
	</div>

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

{#snippet CommonIntegrationFields(draft: IntegrationDraft)}
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
{/snippet}

{#snippet IntegrationStatus(integration: IntegrationView)}
	<div class="page-actions">
		<span class:ok={integration.enabled} class="status-pill">{integration.enabled ? '启用' : '停用'}</span>
		{#if integration.secret_configured}
			<span class="mono muted">secret {integration.secret_fingerprint}</span>
		{/if}
	</div>
{/snippet}

{#snippet ZPayFields(draft: IntegrationDraft, existing: boolean, secretConfigured: boolean)}
	<div class="settings-grid">
		<label class="field-card">
			<span>PID</span>
			<input bind:value={draft.zpayPid} autocomplete="off" />
		</label>
		<label class="field-card">
			<span>CID</span>
			<input bind:value={draft.zpayCid} autocomplete="off" />
		</label>
		<label class="field-card">
			<span>KEY</span>
			<input type="password" bind:value={draft.zpayKey} autocomplete="new-password" placeholder={existing && secretConfigured ? '留空表示不修改' : ''} />
		</label>
	</div>
{/snippet}

{#snippet SMTPFields(draft: IntegrationDraft, existing: boolean, secretConfigured: boolean)}
	<div class="settings-grid">
		<label class="field-card">
			<span>SMTP 地址</span>
			<input bind:value={draft.smtpAddr} placeholder="smtp.example.com:587" autocomplete="off" />
		</label>
		<label class="field-card">
			<span>用户名</span>
			<input bind:value={draft.smtpUsername} autocomplete="off" />
		</label>
		<label class="field-card">
			<span>发件人</span>
			<input bind:value={draft.smtpFrom} placeholder="CDX <noreply@example.com>" autocomplete="off" />
		</label>
		<label class="field-card">
			<span>密码</span>
			<input type="password" bind:value={draft.smtpPassword} autocomplete="new-password" placeholder={existing && secretConfigured ? '留空表示不修改' : ''} />
		</label>
	</div>
{/snippet}

{#snippet ResendFields(draft: IntegrationDraft, existing: boolean, secretConfigured: boolean)}
	<div class="settings-grid">
		<label class="field-card">
			<span>发件人</span>
			<input bind:value={draft.resendFrom} placeholder="CDX <noreply@example.com>" autocomplete="off" />
		</label>
		<label class="field-card">
			<span>API Key</span>
			<input type="password" bind:value={draft.resendApiKey} autocomplete="new-password" placeholder={existing && secretConfigured ? '留空表示不修改' : ''} />
		</label>
	</div>
{/snippet}

{#snippet TurnstileFields(draft: IntegrationDraft, existing: boolean, secretConfigured: boolean)}
	<div class="settings-grid">
		<label class="field-card">
			<span>Site Key</span>
			<input bind:value={draft.turnstileSiteKey} autocomplete="off" />
		</label>
		<label class="field-card">
			<span>Secret Key</span>
			<input type="password" bind:value={draft.turnstileSecretKey} autocomplete="new-password" placeholder={existing && secretConfigured ? '留空表示不修改' : ''} />
		</label>
	</div>
{/snippet}

<style>
	.settings-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		gap: 14px;
		margin: 14px 0 18px;
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
	input, select {
		min-height: 38px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: rgba(0,0,0,0.28);
		color: var(--text);
		padding: 8px 10px;
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
		margin-bottom: 28px;
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
	.input-with-unit {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.input-with-unit input {
		width: min(180px, 100%);
	}
	.input-with-unit span {
		color: var(--muted);
		font-size: 13px;
	}
	.disabled-panel {
		opacity: 0.72;
	}
	.success-msg {
		border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
		background: color-mix(in srgb, var(--accent) 10%, transparent);
		border-radius: 8px;
		padding: 12px 14px;
		color: var(--accent);
	}
</style>
