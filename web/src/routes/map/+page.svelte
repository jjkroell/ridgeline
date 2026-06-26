<script lang="ts">
	import { onMount } from 'svelte';
	import Map from 'ol/Map';
	import View from 'ol/View';
	import { fromLonLat, toLonLat } from 'ol/proj';
	import { boundingExtent } from 'ol/extent';
	import { Attribution, Zoom } from 'ol/control';
	import { Translate } from 'ol/interaction';
	import type { FeatureLike } from 'ol/Feature';
	import { api, type Node } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { basemap } from '$lib/basemap.svelte';
	import { isLight, locatedNodes } from '$lib/map-util';
	import { applyBasemap, createCoverageLayer, setCoverage } from '$lib/ol/basemap';
	import { createNodeLayer, createPinLayer } from '$lib/ol/nodes';
	import { computeCoverage, covered, distKm, type CoverageResult } from '$lib/coverage';
	import { ROLE_HEX } from '$lib/map-util';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import MapRoleFilter from '$lib/components/MapRoleFilter.svelte';
	import BasemapSelector from '$lib/components/BasemapSelector.svelte';
	import NodeModal from '$lib/components/NodeModal.svelte';

	let mapEl: HTMLDivElement;
	let map: Map | null = null;
	let ready = false;
	let didFit = false;
	let allLocated = $state<Node[]>([]);
	let selectedRoles = $state(new Set(['Repeater', 'RoomServer', 'ChatNode', 'Sensor']));
	let nodeKey = $state<string | null>(null);

	let nodeLayer: ReturnType<typeof createNodeLayer> | null = null;
	let coverageLayer: ReturnType<typeof createCoverageLayer> | null = null;
	let pinLayer: ReturnType<typeof createPinLayer> | null = null;

	const visible = $derived(allLocated.filter((n) => selectedRoles.has(n.role)));

	// ── RF coverage prediction (terrain line-of-sight from a planned repeater) ──
	let coverageMode = $state(false);
	let pin = $state<{ lat: number; lon: number } | null>(null);
	let txHeight = $state(6);
	let rxHeight = $state(2);
	let rangeKm = $state(15);
	let computing = $state(false);
	let coverage = $state<CoverageResult | null>(null);

	// Known located nodes that fall inside the computed coverage, nearest first.
	const nodesInCoverage = $derived(
		coverage && pin
			? allLocated
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
			coverage = await computeCoverage({
				lat: pin.lat,
				lon: pin.lon,
				txHeightM: txHeight,
				rxHeightM: rxHeight,
				maxRangeKm: rangeKm
			});
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

	// Swap the basemap on theme change or selector change; re-render node dots so
	// their theme-derived ink stroke updates too.
	$effect(() => {
		const id = basemap.id;
		void theme.mode;
		if (!map) return;
		applyBasemap(map, id, isLight());
		nodeLayer?.layer.changed();
	});

	// Re-filter / re-style when the role selection or favorites change.
	$effect(() => {
		void selectedRoles;
		void favorites.keys;
		if (ready && nodeLayer) nodeLayer.setNodes(visible, (pk) => favorites.has(pk));
	});

	// Fit to the bulk of nodes, rejecting geographic outliers (bad GPS or far
	// regions) via the 1.5×IQR rule so the view focuses where the mesh is.
	function fitToNodes() {
		if (!map || allLocated.length === 0) return;
		const q = (arr: number[], p: number) =>
			arr[Math.min(arr.length - 1, Math.max(0, Math.round((arr.length - 1) * p)))];
		const whisker = (vals: number[]): [number, number] => {
			const s = [...vals].sort((a, b) => a - b);
			const q1 = q(s, 0.25),
				q3 = q(s, 0.75),
				iqr = q3 - q1;
			return [q1 - 1.5 * iqr, q3 + 1.5 * iqr];
		};
		const [latLo, latHi] = whisker(allLocated.map((n) => n.latitude!));
		const [lonLo, lonHi] = whisker(allLocated.map((n) => n.longitude!));
		const inliers = allLocated.filter(
			(n) =>
				n.latitude! >= latLo && n.latitude! <= latHi && n.longitude! >= lonLo && n.longitude! <= lonHi
		);
		const pts = inliers.length ? inliers : allLocated;
		const ext = boundingExtent(pts.map((n) => fromLonLat([n.longitude!, n.latitude!])));
		map.getView().fit(ext, { padding: [80, 80, 80, 80], maxZoom: 12, duration: 600 });
	}

	async function plot() {
		if (!map) return;
		const nodes = await api.nodes();
		allLocated = locatedNodes(nodes);
		if (ready && nodeLayer) nodeLayer.setNodes(visible, (pk) => favorites.has(pk));
		if (!didFit && allLocated.length > 0) {
			fitToNodes();
			didFit = true;
		}
	}

	function zoomToCluster(members: FeatureLike[]) {
		if (!map) return;
		const ext = boundingExtent(
			members.map((f) => (f.getGeometry() as import('ol/geom/Point').default).getCoordinates())
		);
		map.getView().fit(ext, { padding: [100, 100, 100, 100], maxZoom: 14, duration: 400 });
	}

	onMount(() => {
		basemap.init();
		nodeLayer = createNodeLayer();
		coverageLayer = createCoverageLayer();
		pinLayer = createPinLayer();

		map = new Map({
			target: mapEl,
			// Overlays bottom→top; applyBasemap then inserts the base below them.
			layers: [coverageLayer, nodeLayer.layer, pinLayer.layer],
			view: new View({ center: fromLonLat([-123.65, 49.25]), zoom: 9 }),
			controls: [new Attribution({ collapsible: true, collapsed: true }), new Zoom()]
		});
		applyBasemap(map, basemap.id, isLight());

		// Click: drop/keep the coverage pin, else open a node or expand a cluster.
		map.on('click', (e) => {
			if (coverageMode) {
				const ll = toLonLat(e.coordinate);
				placePin(ll[1], ll[0]);
				runCoverage();
				return;
			}
			let handled = false;
			map!.forEachFeatureAtPixel(
				e.pixel,
				(feat) => {
					if (handled) return;
					const members = feat.get('features') as FeatureLike[] | undefined;
					if (!members) return;
					if (members.length > 1) zoomToCluster(members);
					else {
						const pk = members[0].get('pubkey') as string | undefined;
						if (pk) nodeKey = pk;
					}
					handled = true;
				},
				{ layerFilter: (l) => l === nodeLayer!.layer, hitTolerance: 4 }
			);
		});

		// Pointer cursor over clusters/nodes.
		map.on('pointermove', (e) => {
			if (e.dragging) return;
			const hit = map!.hasFeatureAtPixel(e.pixel, { layerFilter: (l) => l === nodeLayer!.layer });
			map!.getTargetElement().style.cursor = hit ? 'pointer' : '';
		});

		// Coverage pin is draggable; recompute when it's dropped.
		const translate = new Translate({ layers: [pinLayer.layer] });
		translate.on('translateend', (e) => {
			const f = e.features.item(0);
			if (!f) return;
			const ll = toLonLat((f.getGeometry() as import('ol/geom/Point').default).getCoordinates());
			pin = { lat: ll[1], lon: ll[0] };
			runCoverage();
		});
		map.addInteraction(translate);

		ready = true;
		setTimeout(() => map?.updateSize(), 80);
		plot();
		const t = setInterval(plot, 10000);
		return () => {
			clearInterval(t);
			map?.setTarget(undefined);
			map?.dispose();
			map = null;
		};
	});
</script>

<PageHeader eyebrow="Terrain & Topology" title="Network Map">
	<div class="font-mono text-fg-dim text-xs">
		<span class="text-signal tnum">{visible.length}</span> <span class="text-fg-faint">shown</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	<div class="panel relative overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<div bind:this={mapEl} class="h-full w-full"></div>
		<MapRoleFilter bind:selected={selectedRoles} />
		<BasemapSelector />

		<!-- Coverage prediction control (bottom-left; panel expands upward) -->
		<div class="absolute bottom-3 left-3 z-10 flex w-64 max-w-[80vw] flex-col-reverse gap-2">
			<button
				onclick={toggleCoverage}
				class="panel flex w-full items-center gap-2 px-3 py-2 text-sm font-600 transition-colors {coverageMode ? 'border-signal/50 text-signal' : 'text-fg-dim hover:text-fg'}"
			>
				<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4.9 4.9a10 10 0 0 0 0 14.2M19.1 4.9a10 10 0 0 1 0 14.2M8 8a5 5 0 0 0 0 8M16 8a5 5 0 0 1 0 8M12 11.2a1 1 0 1 0 0 1.6 1 1 0 0 0 0-1.6z" /></svg>
				Coverage prediction
				<span class="ml-auto text-xs">{coverageMode ? '×' : '+'}</span>
			</button>

			{#if coverageMode}
				<div class="panel rise px-4 py-3">
					<p class="text-fg-faint mb-3 text-xs">
						{pin ? 'Drag the pin or tap to move it.' : 'Tap the map to drop a planned repeater.'}
					</p>
					<div class="flex gap-2">
						<label class="flex-1">
							<span class="label">Antenna m</span>
							<input type="number" min="0" bind:value={txHeight} onchange={runCoverage} class="border-line bg-ink-2 text-fg focus:border-signal mt-1 w-full rounded-[var(--radius)] border px-2 py-1 font-mono text-sm outline-none" />
						</label>
						<label class="flex-1">
							<span class="label">Range km</span>
							<input type="number" min="1" max="60" bind:value={rangeKm} onchange={runCoverage} class="border-line bg-ink-2 text-fg focus:border-signal mt-1 w-full rounded-[var(--radius)] border px-2 py-1 font-mono text-sm outline-none" />
						</label>
					</div>

					<div class="mt-3 flex items-center gap-2">
						<button onclick={runCoverage} disabled={!pin || computing} class="border-signal/40 bg-signal/15 text-signal flex-1 rounded-[var(--radius)] border px-3 py-1.5 text-xs font-600 disabled:opacity-50">
							{computing ? 'Computing…' : 'Recompute'}
						</button>
						{#if pin}<button onclick={clearCoverage} class="border-line text-fg-dim hover:text-coral rounded-[var(--radius)] border px-3 py-1.5 text-xs font-600">Clear</button>{/if}
					</div>

					{#if coverage}
						<div class="border-line/60 text-fg-faint mt-3 border-t pt-2 font-mono text-[0.62rem]">
							ground {Number.isFinite(coverage.groundElevM) ? coverage.groundElevM.toFixed(0) + ' m' : '—'} · reaches up to {coverage.maxReachKm.toFixed(1)} km
						</div>

						<!-- Nodes reachable inside the coverage -->
						<div class="border-line/60 mt-2 border-t pt-2">
							<div class="label mb-1.5 flex items-center justify-between">
								<span>Nodes in coverage</span>
								<span class="text-signal tnum">{nodesInCoverage.length}</span>
							</div>
							{#if nodesInCoverage.length === 0}
								<div class="text-fg-faint text-xs">No known nodes inside.</div>
							{:else}
								<div class="-mr-1 max-h-48 space-y-0.5 overflow-y-auto pr-1">
									{#each nodesInCoverage as { n, d } (n.publicKey)}
										<button onclick={() => (nodeKey = n.publicKey)} class="hover:bg-panel-2/50 flex w-full items-center gap-2 rounded-[var(--radius)] px-1.5 py-1 text-left">
											<span class="h-2 w-2 shrink-0 rounded-full" style="background:{ROLE_HEX[n.role] ?? '#8394a1'}"></span>
											<span class="text-fg-dim min-w-0 flex-1 truncate text-xs">{n.name || n.publicKey.slice(0, 10)}</span>
											<span class="text-fg-faint font-mono text-[0.62rem] tnum">{d.toFixed(1)} km</span>
										</button>
									{/each}
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</div>

<NodeModal pubkey={nodeKey} onclose={() => (nodeKey = null)} />
