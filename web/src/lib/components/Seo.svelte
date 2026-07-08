<script lang="ts">
	// Per-page SEO. Overrides the crawl-time defaults baked into app.html once the
	// SPA renders (Googlebot picks these up on its JavaScript pass, and they set the
	// browser tab title). Keep titles under ~60 chars and descriptions ~150–160.
	const SITE = 'https://ridgeline.ve7kod.ca';

	let {
		title,
		description,
		path = ''
	}: { title: string; description: string; path?: string } = $props();

	// Suffix the brand unless the title already carries it.
	const fullTitle = $derived(
		title.includes('Ridgeline') ? title : `${title} · Ridgeline`
	);
	const canonical = $derived(`${SITE}${path}`);
</script>

<svelte:head>
	<title>{fullTitle}</title>
	<meta name="description" content={description} />
	<link rel="canonical" href={canonical} />
	<meta property="og:title" content={fullTitle} />
	<meta property="og:description" content={description} />
	<meta property="og:url" content={canonical} />
	<meta name="twitter:title" content={fullTitle} />
	<meta name="twitter:description" content={description} />
</svelte:head>
