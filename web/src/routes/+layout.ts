// Pure client-side SPA: no server-side rendering or prerendering. The Go
// daemon serves the static bundle and falls back to index.html.
import { browser } from '$app/environment';
import { redirect } from '@sveltejs/kit';
import { isSearchCrawler } from '$lib/crawler';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = false;

// Desktop section roots that have a mobile equivalent under /m. Dynamic detail
// routes (/nodes/[k], /observers/[id]) are omitted until their /m screens exist.
const MOBILE_ROOTS = ['/', '/nodes', '/live', '/map', '/live-map', '/analytics', '/channels', '/observers', '/admin', '/contact'];

// Send phones to the mobile app. Not search engines' phone crawlers: the desktop
// URL is the canonical one, so they must be able to stay on it.
export const load: LayoutLoad = ({ url }) => {
	if (!browser || isSearchCrawler()) return;
	if (url.pathname === '/m' || url.pathname.startsWith('/m/')) return;
	const isPhone = window.matchMedia('(pointer: coarse)').matches && window.innerWidth <= 820;
	if (isPhone && MOBILE_ROOTS.includes(url.pathname)) {
		// Keep the query (/channels?channel=sos) and fragment (a private channel
		// link's key) across the hop. load can't read url.hash; on a page load the
		// address bar has it.
		const hash = location.pathname === url.pathname ? location.hash : '';
		redirect(307, (url.pathname === '/' ? '/m' : '/m' + url.pathname) + url.search + hash);
	}
};
