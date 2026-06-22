/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />
import { build, files, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;

// The public deployment currently serves the Vite DEV server. A caching SW
// against a dev server is a foot-gun: it caches the dev app shell, whose module
// URLs change on every restart, leaving stale references that 404. So in dev the
// SW is a self-destructing no-op (clears caches + unregisters); full PWA caching
// only runs against a production build.
const DEV = import.meta.env.DEV;

const CACHE = `ridgeline-${version}`;
const SHELL = [...build, ...files];

sw.addEventListener('install', (event) => {
	sw.skipWaiting();
	if (DEV) return;
	event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)));
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		(async () => {
			// Always drop stale caches; in dev drop everything and remove the SW so a
			// previously-cached bad shell can't keep serving 404s.
			for (const key of await caches.keys()) {
				if (DEV || key !== CACHE) await caches.delete(key);
			}
			await sw.clients.claim();
			if (DEV) await sw.registration.unregister();
		})()
	);
});

sw.addEventListener('fetch', (event) => {
	if (DEV) return; // pass through to the network in dev
	const req = event.request;
	if (req.method !== 'GET') return;
	const url = new URL(req.url);
	if (url.origin !== location.origin) return;

	// App shell + static assets → cache-first.
	if (SHELL.includes(url.pathname)) {
		event.respondWith(caches.match(req).then((hit) => hit ?? fetch(req)));
		return;
	}

	// API reads → network-first, fall back to the last cached response offline.
	if (url.pathname.startsWith('/api/')) {
		event.respondWith(
			(async () => {
				try {
					const res = await fetch(req);
					if (res.ok) (await caches.open(CACHE)).put(req, res.clone());
					return res;
				} catch {
					const cached = await caches.match(req);
					if (cached) return cached;
					return new Response(JSON.stringify({ error: 'offline' }), {
						status: 503,
						headers: { 'content-type': 'application/json' }
					});
				}
			})()
		);
		return;
	}

	// SPA navigations → network-first, fall back to the cached shell offline.
	if (req.mode === 'navigate') {
		event.respondWith(
			(async () => {
				try {
					return await fetch(req);
				} catch {
					return (
						(await caches.match('/index.html')) ??
						(await caches.match('/')) ??
						new Response('offline', { status: 503 })
					);
				}
			})()
		);
	}
});
