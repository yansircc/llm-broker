<script lang="ts">
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL } from '$lib/brand';
</script>

<section class="mx-auto max-w-4xl px-5 py-16">
	<div class="font-mono text-xs uppercase tracking-wider text-brand">terms of service</div>
	<h1 class="mt-3 text-4xl font-bold tracking-tight">Terms of Service</h1>
	<p class="mt-3 font-mono text-xs text-faint">Last updated: June 19, 2026</p>

	<div class="mt-10 space-y-8 text-sm leading-7 text-muted">
		<section>
			<h2 class="text-2xl font-bold text-slate-100">1. Service</h2>
			<p>
				{BRAND_NAME} provides a prepaid API relay service for supported model endpoints. Current production availability is bounded by the runtime model catalog, account pool capacity, user balance, API key state, and upstream provider behavior.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">2. Accounts and Access</h2>
			<p>
				You are responsible for maintaining control of your account, API keys, credentials, and client configuration. You must not share access in a way that enables abuse, resale outside approved partner terms, or unauthorized use.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">3. Billing</h2>
			<p>
				Prepaid balances are consumed according to request usage, model pricing, cache behavior, and service-side billing rules. Public pricing pages are explanatory; the authoritative billing record is the server-side ledger.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">4. Availability</h2>
			<p>
				The service may be affected by upstream model availability, account limits, network conditions, maintenance, abuse controls, or payment-provider constraints. We do not guarantee that every displayed model, reserved plan, or future integration is available at all times.
			</p>
		</section>

		<section>
			<h2 class="text-2xl font-bold text-slate-100">5. Contact</h2>
			<p>
				For support, enterprise procurement, or partner questions, contact <span class="font-mono text-brand">{BRAND_SUPPORT_EMAIL}</span>.
			</p>
		</section>
	</div>
</section>
