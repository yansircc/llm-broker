<script lang="ts">
	import { browser } from '$app/environment';
	import { base } from '$app/paths';
	import Logo from '$lib/components/Logo.svelte';
	import MetricCard from '$lib/components/MetricCard.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';

	let origin = $state('https://your-domain.example');

	$effect(() => {
		if (browser) {
			origin = window.location.origin;
		}
	});

	const terminalLines = $derived([
		`$ curl ${origin}/v1/responses`,
		'> Authorization: Bearer cdx_live_***',
		'> model: gpt-5',
		'< routed: openai/responses',
		'< charged: input/output tokens',
		'< balance: positive'
	]);
</script>

<div class="site-shell">
	<header class="top-glass">
		<div class="nav-inner">
			<Logo href={`${base}/`} />
			<nav class="nav-links" aria-label="Site navigation">
				<a href="#relay">relay</a>
				<a href="#billing">billing</a>
				<a href="{base}/app/login">login</a>
				<a class="active" href="{base}/app/register">start</a>
			</nav>
		</div>
	</header>

	<main class="shell-main">
		<section class="hero">
			<div class="hero-grid">
				<div>
					<div class="eyebrow">openai relay control plane</div>
					<h1>CDX Codex API relay for external users.</h1>
					<p class="lede">
						OpenAI Responses and compatible API forwarding with user accounts, API keys, prepaid balance,
						token-based charging, recharge orders, and referral credit tracking.
					</p>
					<div class="hero-actions">
						<a class="cta-primary" href="{base}/app/register">Create account</a>
						<a class="cta-secondary" href="{base}/app/login">Open console</a>
						<a class="cta-secondary" href="{base}/login">Admin</a>
					</div>
				</div>
				<TerminalPanel title="cdx edge" lines={terminalLines} />
			</div>
		</section>

		<section class="metric-grid" aria-label="Product signals">
			<MetricCard label="surface" value="Responses" sub="native OpenAI relay" />
			<MetricCard label="billing" value="Token" sub="USD display, RMB recharge" />
			<MetricCard label="access" value="API Keys" sub="per-user key management" />
			<MetricCard label="growth" value="Referral" sub="signup and paid incentives" />
		</section>

		<section id="relay" class="feature-grid">
			<div class="feature-card">
				<div class="feature-icon">01</div>
				<h3>OpenAI-first forwarding</h3>
				<p class="muted">The public product surface is centered on OpenAI Responses with a compatibility layer kept for client migration.</p>
			</div>
			<div class="feature-card">
				<div class="feature-icon">02</div>
				<h3>Account-bound access</h3>
				<p class="muted">Customers create keys in the console. Relay requests are admitted by account status and positive balance.</p>
			</div>
			<div class="feature-card">
				<div class="feature-icon">03</div>
				<h3>Operational visibility</h3>
				<p class="muted">Admin pages keep account pools, users, request logs, billing orders, and failed relay evidence in one control surface.</p>
			</div>
		</section>

		<section id="billing" class="panel section-band">
			<div>
				<div class="eyebrow">prepaid metering</div>
				<h2>Charge in RMB. Display and settle usage in USD credits.</h2>
			</div>
			<p class="lede">
				The exchange ratio is configurable. The current business default can present 1 RMB as 1 USD credit while the ledger still records token usage as USD debits.
			</p>
		</section>
	</main>

	<footer class="footer">
		<div class="footer-inner">
			<span>CDX relay</span>
			<span>openai/responses · billing · referrals · payments</span>
		</div>
	</footer>
</div>
