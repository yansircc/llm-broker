<script lang="ts">
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';
</script>

<section class="mx-auto max-w-4xl px-5 py-16">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">privacy policy</div>
	<h1 class="mt-3 text-4xl font-bold tracking-tight">Privacy Policy</h1>
	<p class="mt-3 font-mono text-xs text-faint">Last updated: June 19, 2026</p>

	<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
		<section>
			<h2 class="text-2xl font-bold text-slate-100">1. Scope</h2>
			<p>
				This Privacy Policy explains how {BRAND_NAME} handles information required to provide account access, API relay service, prepaid billing, abuse prevention, support, and partner communication.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">2. Information We Process</h2>
			<p>
				We may process account identifiers, email addresses, API key metadata, payment and balance records, request usage records, model identifiers, timestamps, IP-derived security signals, support messages, and affiliate or enterprise inquiry details.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">3. How We Use Information</h2>
			<p>
				We use information to authenticate users, meter usage, calculate balances, detect abuse, troubleshoot service issues, communicate service notices, and operate partner or enterprise workflows. Secrets such as passwords and API keys should not be logged or displayed in plaintext.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">4. Retention and Security</h2>
			<p>
				Operational records are retained as needed for billing, audit, fraud prevention, debugging, and legal compliance. We use access controls and redaction practices intended to limit exposure of credentials and sensitive request data.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">5. Contact</h2>
			<p>
				For privacy questions or data requests, contact <span class="font-mono text-brand">{BRAND_SUPPORT_EMAIL}</span>. We may need to verify account ownership before acting on a request.
			</p>
		</section>
	</div>
</section>
