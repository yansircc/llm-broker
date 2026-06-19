<script lang="ts">
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';
</script>

<section class="mx-auto max-w-4xl px-5 py-16">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">acceptable use policy</div>
	<h1 class="mt-3 text-4xl font-bold tracking-tight">Acceptable Use Policy</h1>
	<p class="mt-3 font-mono text-xs text-faint">Last updated: June 19, 2026</p>

	<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
		<section>
			<h2 class="text-2xl font-bold text-slate-100">1. Prohibited Uses</h2>
			<p>
				You may not use {BRAND_NAME} for illegal activity, credential theft, malware, phishing, spam, harassment, payment fraud, platform abuse, unauthorized security bypass, or activity that violates third-party rights or provider policies.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">2. API Abuse</h2>
			<p>
				You must not automate account creation, evade rate limits, resell access without approval, rotate identities to avoid enforcement, overload shared capacity, or submit requests intended to degrade service for other users.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">3. Enforcement</h2>
			<p>
				We may limit requests, disable API keys, suspend accounts, freeze suspicious balances, preserve audit records, or refuse service when activity appears unsafe, abusive, fraudulent, or incompatible with upstream requirements.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">4. Reporting</h2>
			<p>
				To report abuse or request review of an enforcement decision, contact <span class="font-mono text-brand">{BRAND_SUPPORT_EMAIL}</span> with relevant account identifiers, timestamps, and request IDs.
			</p>
		</section>
	</div>
</section>
