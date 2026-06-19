import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const sourceRoots = ['src', 'tests'];
const brandModule = 'src/lib/brand.ts';
const allowedHardcodedBrandFiles = new Set([brandModule, 'tests/brand-constant.test.mjs']);
const forbiddenNames = ['CCSub', 'CC Substitute', 'ccsub.net', 'www.ccsub.net', 'support@ccsub'];
const temporaryName = 'cdx';

function* walk(dir) {
	for (const entry of readdirSync(dir)) {
		const path = join(dir, entry);
		const stat = statSync(path);
		if (stat.isDirectory()) {
			yield* walk(path);
		} else if (/\.(svelte|ts|js|mjs)$/.test(path)) {
			yield path;
		}
	}
}

const brandSource = readFileSync(resolve(root, brandModule), 'utf8');
assert.match(brandSource, /export const BRAND_NAME = 'cdx'/, 'brand name must be centralized in src/lib/brand.ts');
assert.match(brandSource, /export const BRAND_TAGLINE = 'Claude Code 超强平替'/, 'brand tagline must be centralized in src/lib/brand.ts');

for (const sourceRoot of sourceRoots) {
	for (const file of walk(resolve(root, sourceRoot))) {
		const rel = relative(root, file);
		const content = readFileSync(file, 'utf8');
		if (!allowedHardcodedBrandFiles.has(rel)) {
			for (const name of forbiddenNames) {
				assert.ok(!content.includes(name), `${rel} must not hardcode copied target brand name ${name}`);
			}
			assert.ok(!content.includes(temporaryName), `${rel} must import the temporary brand name instead of hardcoding ${temporaryName}`);
		}
	}
}
