import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');

const requiredCustomerRoutes = [
	'src/routes/app/dashboard/+page.svelte',
	'src/routes/app/keys/+page.svelte',
	'src/routes/app/key-test/+page.svelte',
	'src/routes/app/images/+page.svelte',
	'src/routes/app/usage/+page.svelte',
	'src/routes/app/billing/+page.svelte',
	'src/routes/app/orders/+page.svelte',
	'src/routes/app/subscriptions/+page.svelte',
	'src/routes/app/balance-history/+page.svelte',
	'src/routes/app/redeem/+page.svelte',
	'src/routes/app/referrals/+page.svelte',
	'src/routes/app/referrals/earnings/+page.svelte',
	'src/routes/app/settings/+page.svelte'
];

for (const route of requiredCustomerRoutes) {
	assert.ok(existsSync(resolve(root, route)), `missing target-aligned customer route: ${route}`);
}

const requiredPublicRoutes = [
	'src/routes/pricing/+page.svelte',
	'src/routes/models/+page.svelte',
	'src/routes/blog/+page.svelte',
	'src/routes/blog/claude-code-setup-guide/+page.svelte',
	'src/routes/blog/api-pricing-comparison/+page.svelte',
	'src/routes/blog/best-ai-coding-tools-2026/+page.svelte',
	'src/routes/partner/+page.svelte',
	'src/routes/contact/+page.svelte',
	'src/routes/privacy-policy/+page.svelte',
	'src/routes/terms-of-service/+page.svelte',
	'src/routes/acceptable-use-policy/+page.svelte'
];

for (const route of requiredPublicRoutes) {
	assert.ok(existsSync(resolve(root, route)), `missing target-aligned public route: ${route}`);
}

const requiredDocsRoutes = [
	'src/routes/docs/+page.svelte',
	'src/routes/docs/getting-started/+page.svelte',
	'src/routes/docs/pricing/+page.svelte',
	'src/routes/docs/install/+page.svelte',
	'src/routes/docs/usage/+page.svelte',
	'src/routes/docs/troubleshooting/+page.svelte',
	'src/routes/docs/faq/+page.svelte'
];

for (const route of requiredDocsRoutes) {
	assert.ok(existsSync(resolve(root, route)), `missing target-aligned docs route: ${route}`);
}
