<script module lang="ts">
	// A prerendered page's head tags are not adopted on hydration: Svelte adds its
	// own set beside them, so a crawler's rendered page had two canonicals, and the
	// first page's stayed behind on every later one. Note what the HTML arrived
	// with, before any component runs (hydration can start over, so a component's
	// own setup is too late); once a page's own tags are in, these go.
	let arrived: Element[] =
		typeof document === 'undefined'
			? []
			: [
					...document.head.querySelectorAll(
						'link[rel="canonical"], meta[name="description"], meta[property^="og:"], meta[name^="twitter:"]'
					)
				];
</script>

<script lang="ts">
	// Per-page SEO. Go renders the initial public share metadata; this component
	// takes over once the SPA mounts and updates it during client navigation.
	const SITE = 'https://ridgeline.ve7kod.ca';
	import { onMount } from 'svelte';

	let {
		title,
		description,
		path = ''
	}: { title: string; description: string; path?: string } = $props();

	// Suffix the brand unless the title already carries it.
	const fullTitle = $derived(
		title.includes('Ridgeline') ? title : `${title} · Ridgeline`
	);

	let canonicalEl: HTMLLinkElement | undefined;

	// The Go server owns the initial no-JS head. Once this component mounts,
	// Svelte owns navigation metadata, so a previous node's image cannot linger.
	onMount(() => {
		// Tags with no counterpart of ours (og:locale) stay.
		const key = (e: Element) => e.getAttribute('property') ?? e.getAttribute('name') ?? e.getAttribute('rel');
		if (canonicalEl && !arrived.includes(canonicalEl)) {
			const ours = new Set(
				[...document.head.querySelectorAll('meta, link[rel="canonical"]')]
					.filter((e) => !arrived.includes(e))
					.map(key)
			);
			for (const tag of arrived) if (ours.has(key(tag))) tag.remove();
		}
		arrived = [];
		for (const tag of document.head.querySelectorAll('[data-ridgeline-share]')) {
			// Svelte updates document.title in place; keep that title element.
			if (tag.tagName === 'TITLE') tag.removeAttribute('data-ridgeline-share');
			else tag.remove();
		}
	});
	const sharePath = $derived(path === '/m' ? '/' : path.replace(/^\/m\//, '/').replace(/\/$/, '') || '/');
	// A /m page is the phone layout of the same page: point search at the
	// desktop URL, as the server's initial head does (share_preview.go).
	const canonical = $derived(`${SITE}${sharePath}`);
	const hasCard = $derived(
		['/', '/nodes', '/live', '/channels', '/map', '/live-map', '/analytics', '/topology', '/identity', '/observers', '/contact'].includes(sharePath)
		|| /^\/nodes\/[a-fA-F0-9]{64}$/.test(sharePath)
	);
	const image = $derived(hasCard
		? `${SITE}/share/card.png?path=${encodeURIComponent(sharePath)}`
		: `${SITE}/icons/icon-512.png`);
</script>

<svelte:head>
	<title>{fullTitle}</title>
	<meta name="description" content={description} />
	<link rel="canonical" href={canonical} bind:this={canonicalEl} />
	<meta property="og:title" content={fullTitle} />
	<meta property="og:description" content={description} />
	<meta property="og:url" content={canonical} />
	<meta name="twitter:title" content={fullTitle} />
	<meta name="twitter:description" content={description} />
	<meta property="og:type" content="website" />
	<meta property="og:site_name" content="Ridgeline" />
	<meta property="og:image" content={image} />
	<meta property="og:image:alt" content={`${title} — ${description}`} />
	<meta name="twitter:card" content={hasCard ? 'summary_large_image' : 'summary'} />
	<meta name="twitter:image" content={image} />
	<meta name="twitter:image:alt" content={`${title} — ${description}`} />
</svelte:head>
