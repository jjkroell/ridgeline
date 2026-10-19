// The URL actually in the address bar.
//
// Pages that mirror their view into the URL use SvelteKit's shallow
// `replaceState`, which rewrites the address bar but NOT `page.url` — and it
// stores that stale `page.url` in the history entry. After a back navigation
// SvelteKit hands the page the stale one, so anything read from `page.url`
// comes back as it was when the page first loaded: a map zoomed in, opened a
// node and went back reappeared at its starting zoom. Read view state from
// here instead. (The app is client-only — `ssr = false` — so `location` is
// always there; the fallback only matters to tests.)
export function addressBarUrl(fallback: URL): URL {
	return typeof location === 'undefined' ? new URL(fallback) : new URL(location.href);
}
