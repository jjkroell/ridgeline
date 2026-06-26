<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Map from 'ol/Map';
	import View from 'ol/View';
	import { fromLonLat, toLonLat } from 'ol/proj';
	import { boundingExtent } from 'ol/extent';
	import { Attribution } from 'ol/control';
	import { Translate } from 'ol/interaction';
	import type { FeatureLike } from 'ol/Feature';
	import { api, type Node } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { basemap } from '$lib/basemap.svelte';
	import { ROLE_HEX, isLight, locatedNodes } from '$lib/map-util';
	import { applyBasemap, createCoverageLayer, setCoverage } from '$lib/ol/basemap';
	import { createNodeLayer, createPinLayer } from '$lib/ol/nodes';
	import { computeCoverage, covered, distKm, type CoverageResult } from '$lib/coverage';
	import BasemapSelector from '$lib/components/BasemapSelector.svelte';

	let mapEl: HTMLDivElement;
	let map: Map | null = null;
	let nodes = $state<Node[]>([]);
	let didFit = false;

	let nodeLayer: ReturnType<typeof createNodeLayer> | null = null;
	let coverageLayer: ReturnType<typeof createCoverageLayer> | null = null;
	let pinLayer: ReturnType<typeof createPinLayer> | null = null;

	// RF coverage prediction
	let coverageMode = $state(false);
	let pin = $state<{ lat: number; lon: number } | null>(null);
	let txHeight = $state(6);
	let rangeKm = $state(15);
	let computing = $state(false);
	let coverage = $state<CoverageResult | null>(null);
	let showNodes = $state(false);

	const nodesInCoverage = $derived(
		coverage && pin
			? nodes
					.filter((n) => covered(coverage!, n.longitude!, n.latitude!))
					.map((n) => ({ n, d: distKm(pin!.lat, pin!.lon, n.latitude!, n.longitude!) }))
					.sort((a, b) => a.d - b.d)
			: []
	);

	function placePin(lat: number, lon: number) {
		pin = { lat, lon };
		pinLayer?.setPin(lon, lat);
	}
	function drawCoverage() {
		if (coverageLayer) setCoverage(coverageLayer, coverage);
	}
	async function runCoverage() {
		if (!pin) return;
		computing = true;
		try {
			coverage = await computeCoverage({ lat: pin.lat, lon: pin.lon, txHeightM: txHeight, rxHeightM: 2, maxRangeKm: rangeKm });
			drawCoverage();
		} finally {
			computing = false;
		}
	}
	function clearCoverage() {
		coverage = null;
		pin = null;
		pinLayer?.clear();
		drawCoverage();
	}
	function toggleCoverage() {
		coverageMode = !coverageMode;
		if (!coverageMode) clearCoverage();
	}

	// Swap basemap on theme/selector change; re-render dots for the new ink stroke.
	$effect(() => {
		const id = basemap.id;
		void theme.mode;
		if (!map) return;
		applyBasemap(map, id, isLight());
		nodeLayer?.layer.changed();
	});

	// Re-render nodes when favorites change.
	$effect(() => {
		void favorites.keys;
		if (map && nodeLayer) nodeLayer.setNodes(nodes, (pk) => favorites.has(pk));
	});

	function fit() {
		if (!map || didFit || nodes.length === 0) return;
		const ext = boundingExtent(nodes.map((n) => fromLonLat([n.longitude!, n.latitude!])));
		map.getView().fit(ext, { padding: [56, 56, 56, 56], maxZoom: 11, duration: 0 });
		didFit = true;
	}

	async function refresh() {
		try {
			nodes = locatedNodes(await api.nodes());
			nodeLayer?.setNodes(nodes, (pk) => favorites.has(pk));
			fit();
		} catch {
			/* keep */
		}
	}

	onMount(() => {
		basemap.init();
		nodeLayer = createNodeLayer({ cluster: false });
		coverageLayer = createCoverageLayer();
		pinLayer = createPinLayer();

		map = new Map({
			target: mapEl,
			layers: [coverageLayer, nodeLayer.layer, pinLayer.layer],
			view: new View({ center: fromLonLat([-123.65, 49.25]), zoom: 7 }),
			controls: [new Attribution({ collapsible: true, collapsed: true })]
		});
		applyBasemap(map, basemap.id, isLight());

		map.on('click', (e) => {
			if (coverageMode) {
				const ll = toLonLat(e.coordinate);
				placePin(ll[1], ll[0]);
				runCoverage();
				return;
			}
			map!.forEachFeatureAtPixel(
				e.pixel,
				(feat) => {
					const pk = (feat as FeatureLike).get('pubkey') as string | undefined;
					if (pk) {
						goto('/m/nodes/' + pk);
						return true;
					}
				},
				{ layerFilter: (l) => l === nodeLayer!.layer, hitTolerance: 6 }
			);
		});

		const translate = new Translate({ layers: [pinLayer.layer] });
		translate.on('translateend', (e) => {
			const f = e.features.item(0);
			if (!f) return;
			const ll = toLonLat((f.getGeometry() as import('ol/geom/Point').default).getCoordinates());
			pin = { lat: ll[1], lon: ll[0] };
			runCoverage();
		});
		map.addInteraction(translate);

		setTimeout(() => map?.updateSize(), 80);
		refresh();
		const t = setInterval(refresh, 15000);
		return () => {
			clearInterval(t);
			map?.setTarget(undefined);
			map?.dispose();
			map = null;
		};
	});
</script>

<div class="relative h-full w-full">
	<div bind:this={mapEl} class="h-full w-full"></div>

	<div class="border-line/60 bg-ink-2/80 pointer-events-none absolute top-3 left-3 z-10 rounded-full border px-3 py-1.5 backdrop-blur-md">
		<span class="text-fg-dim font-mono text-[0.62rem] tnum">{nodes.length} located</span>
	</div>

	<BasemapSelector compact posClass="top-14 left-3" />

	<!-- Coverage toggle -->
	<button
		onclick={toggleCoverage}
		class="border-line/60 bg-ink-2/85 absolute top-3 right-14 z-10 flex items-center gap-1.5 rounded-full border px-3 py-1.5 backdrop-blur-md {coverageMode ? 'text-signal border-signal/50' : 'text-fg-dim'}"
	>
		<svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4.9 4.9a10 10 0 0 0 0 14.2M19.1 4.9a10 10 0 0 1 0 14.2M8 8a5 5 0 0 0 0 8M16 8a5 5 0 0 1 0 8M12 11.2a1 1 0 1 0 0 1.6 1 1 0 0 0 0-1.6z" /></svg>
		<span class="text-[0.62rem] font-600">Coverage</span>
	</button>

	<!-- Coverage panel -->
	{#if coverageMode}
		<div class="border-line/60 bg-ink-2/95 absolute inset-x-2 bottom-2 z-10 rounded-2xl border p-3 backdrop-blur-md">
			{#if !pin}
				<p class="text-fg-faint text-center text-xs">Tap the map to drop a planned repeater.</p>
			{:else}
				<div class="flex items-end gap-2">
					<label class="flex-1">
						<span class="label !text-[0.55rem]">Antenna m</span>
						<input type="number" min="0" bind:value={txHeight} onchange={runCoverage} class="border-line bg-ink text-fg mt-1 w-full rounded-lg border px-2 py-1.5 font-mono text-sm outline-none" />
					</label>
					<label class="flex-1">
						<span class="label !text-[0.55rem]">Range km</span>
						<input type="number" min="1" max="60" bind:value={rangeKm} onchange={runCoverage} class="border-line bg-ink text-fg mt-1 w-full rounded-lg border px-2 py-1.5 font-mono text-sm outline-none" />
					</label>
					<button onclick={runCoverage} disabled={computing} class="border-signal/40 bg-signal/15 text-signal shrink-0 rounded-lg border px-3 py-1.5 text-xs font-600 disabled:opacity-50">{computing ? '…' : 'Go'}</button>
					<button onclick={clearCoverage} class="border-line text-fg-faint shrink-0 rounded-lg border px-2.5 py-1.5 text-xs font-600">×</button>
				</div>
				{#if coverage}
					<button onclick={() => (showNodes = !showNodes)} class="text-fg-faint mt-2 flex w-full items-center justify-between text-[0.62rem]">
						<span>reaches {coverage.maxReachKm.toFixed(1)} km · ground {Number.isFinite(coverage.groundElevM) ? coverage.groundElevM.toFixed(0) + 'm' : '—'}</span>
						<span class="text-signal">{nodesInCoverage.length} nodes {showNodes ? '▾' : '▸'}</span>
					</button>
					{#if showNodes && nodesInCoverage.length}
						<div class="mt-1 max-h-40 space-y-0.5 overflow-y-auto">
							{#each nodesInCoverage as { n, d } (n.publicKey)}
								<a href="/m/nodes/{n.publicKey}" class="active:bg-line/40 flex items-center gap-2 rounded-lg px-1.5 py-1">
									<span class="h-2 w-2 shrink-0 rounded-full" style="background:{ROLE_HEX[n.role] ?? '#8394a1'}"></span>
									<span class="text-fg-dim min-w-0 flex-1 truncate text-xs">{n.name || n.publicKey.slice(0, 10)}</span>
									<span class="text-fg-faint font-mono text-[0.6rem] tnum">{d.toFixed(1)}km</span>
								</a>
							{/each}
						</div>
					{/if}
				{/if}
			{/if}
		</div>
	{/if}
</div>
