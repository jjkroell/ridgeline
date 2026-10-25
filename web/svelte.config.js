import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';

// app.html's inline theme script runs before any module loads, so SvelteKit
// can't hash it for us. Hash it here so editing the script can't silently
// break the CSP.
const appHtml = readFileSync(new URL('./src/app.html', import.meta.url), 'utf8');
const inlineScripts = [...appHtml.matchAll(/<script>([\s\S]*?)<\/script>/g)].map(
	(m) => `sha256-${createHash('sha256').update(m[1]).digest('base64')}`
);

// Map tiles, styles and glyphs are fetched by MapLibre (connect-src) and drawn
// by Leaflet (img-src); keep this list in step with map-basemap.ts,
// leaflet-basemap.ts, map-hillshade.ts and coverage.ts.
const TILE_HOSTS = [
	'https://tiles.openfreemap.org',
	'https://maps.ve7kod.ca',
	'https://server.arcgisonline.com',
	'https://*.tile.opentopomap.org',
	'https://s3.amazonaws.com'
];
const UMAMI = 'https://umami.ve7kod.ca';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	compilerOptions: { runes: true },
	kit: {
		// Single-page app: the Go daemon serves the built static files and
		// falls back to index.html so client routing handles every path.
		adapter: adapter({ fallback: 'index.html' }),
		// The /about page is prerendered (see its +page.ts). Its crawl follows links
		// in the shared app.html head/nav — don't fail the whole build on the
		// pre-existing /favicon.svg reference or on links to SPA-only (non-prerendered)
		// routes; those are served fine at runtime by the Go daemon's index.html fallback.
		prerender: {
			handleHttpError: 'warn',
			handleMissingId: 'warn'
		},
		// Node names and channel text come straight off the air, so script-src
		// stays 'self' + hashes (no 'unsafe-inline') as a backstop against XSS.
		// Delivered as a <meta> tag, which can't carry frame-ancestors — that and
		// the anti-framing header live in deploy/Caddyfile.
		csp: {
			mode: 'hash',
			directives: {
				'default-src': ['self'],
				'script-src': ['self', ...inlineScripts, UMAMI],
				'style-src': ['self', 'unsafe-inline', 'https://fonts.googleapis.com'],
				'font-src': ['self', 'data:', 'https://fonts.gstatic.com'],
				'img-src': ['self', 'data:', 'blob:', ...TILE_HOSTS],
				// data:: MapLibre fetch()es inline placeholder images (e.g. a 1x1
				// PNG on /map). Reading a data: URL never leaves the browser.
				'connect-src': ['self', 'data:', ...TILE_HOSTS, UMAMI],
				'worker-src': ['self', 'blob:'],
				'child-src': ['self', 'blob:'],
				'manifest-src': ['self'],
				'object-src': ['none'],
				'base-uri': ['self'],
				'form-action': ['self']
			}
		},
		alias: {
			$lib: 'src/lib'
		}
	}
};

export default config;
