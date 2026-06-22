/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />
import { build, files, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;

const CACHE = `ridgeline-${version}`;
// App shell: the built JS/CSS plus static assets (icons, contours, manifest).
const SHELL = [...build, ...files];

sw.addEventListener('install', (event) => {
	event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => sw.skipWaiting()));
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		(async () => {
			for (const key of await caches.keys()) {
				if (key !== CACHE) await caches.delete(key);
			}
			await sw.clients.claim();
		})()
	);
});

sw.addEventListener('fetch', (event) => {
	const req = event.request;
	if (req.method !== 'GET') return;
	const url = new URL(req.url);
	if (url.origin !== location.origin) return;

	// App shell + static assets → cache-first (instant, offline-capable).
	if (SHELL.includes(url.pathname)) {
		event.respondWith(caches.match(req).then((hit) => hit ?? fetch(req)));
		return;
	}

	// API reads → network-first, fall back to the last cached response so the
	// app still shows the most recent data it saw when offline. (WebSocket is
	// not a fetch, so the live feed simply pauses offline — expected.)
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

	// SPA navigations → network-first, fall back to the cached app shell so the
	// PWA cold-starts offline (client router then handles the route).
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
