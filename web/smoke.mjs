#!/usr/bin/env node
// Browser smoke test — catches JS runtime errors that curl can't detect.
// Usage: SITE=https://DEPLOY_HOST API_TOKEN=xxx node web/smoke.mjs

import { chromium } from 'playwright-core';

const SITE = (process.env.SITE || 'https://DEPLOY_HOST').replace(/\/$/, '');
const TOKEN = process.env.API_TOKEN;
if (!TOKEN) { console.error('API_TOKEN is required'); process.exit(1); }

const domain = new URL(SITE).hostname;

// Pages to visit — detail pages extracted dynamically from dashboard links
const staticPages = ['/ui/dashboard', '/ui/add-account'];

async function run() {
  const browser = await chromium.launch();
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  await context.addCookies([
    { name: 'cc_session', value: TOKEN, domain, path: '/' },
  ]);

  const results = []; // { path, errors[] }

  async function visit(path) {
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (err) => errors.push(err.message));
    page.on('console', (msg) => {
      if (msg.type() !== 'error') return;
      const text = msg.text();
      // Browser-generated network errors — already covered by curl smoke tests
      if (text.includes('Failed to load resource')) return;
      errors.push(`console.error: ${text}`);
    });

    try {
      await page.goto(`${SITE}${path}`, { waitUntil: 'networkidle', timeout: 15000 });
      // extra settle time for async API calls + reactive rendering
      await page.waitForTimeout(1000);
    } catch (e) {
      errors.push(`navigation: ${e.message}`);
    }

    results.push({ path, errors });
    return page;
  }

  // 1. Visit dashboard and extract detail page links
  const dashboard = await visit('/ui/dashboard');
  let detailPages = [];
  try {
    const links = await dashboard.$$eval('a[href]', (els) =>
      els.map((a) => a.getAttribute('href')).filter(Boolean)
    );
    const accountLink = links.find((h) => h.match(/\/ui\/accounts\/\d+/));
    const userLink = links.find((h) => h.match(/\/ui\/users\/\d+/));
    if (accountLink) detailPages.push(accountLink);
    if (userLink) detailPages.push(userLink);
  } catch { /* no links found — skip detail pages */ }
  await dashboard.close();

  // 2. Visit remaining static pages + extracted detail pages
  const remaining = [...staticPages.slice(1), ...detailPages];
  for (const path of remaining) {
    const p = await visit(path);
    await p.close();
  }

  await browser.close();

  // 3. Report
  let hasErrors = false;
  for (const { path, errors } of results) {
    if (errors.length === 0) {
      console.log(`    \u2713 ${path} (0 errors)`);
    } else {
      hasErrors = true;
      console.log(`    \u2717 ${path} (${errors.length} error${errors.length > 1 ? 's' : ''})`);
      for (const e of errors) console.log(`      ${e}`);
    }
  }

  process.exit(hasErrors ? 1 : 0);
}

run().catch((e) => { console.error(e); process.exit(1); });
