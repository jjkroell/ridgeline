<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { roleLabel } from '$lib/format';
	import { theme } from '$lib/theme.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { basemapStyleUrl, collapseAttribution } from '$lib/map-basemap';
	import { ensureHillshade } from '$lib/map-hillshade';
	import { ROLE_HEX, FAV_COLOR, locatedNodes } from '$lib/map-util';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let nodes = $state<Node[]>([]);
	let didFit = false;
	let basemapLight = false;

	function features(): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: nodes.map((n) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [n.longitude!, n.latitude!] },
				properties: {
					color: ROLE_HEX[n.role] ?? '#8394a1',
					name: n.name || n.publicKey.slice(0, 10),
					roleLabel: roleLabel(n.role),
					pubkey: n.publicKey,
					fav: favorites.has(n.publicKey)
				}
			}))
		};
	}
	function updateSource() {
		(map?.getSource('nodes') as maplibregl.GeoJSONSource | undefined)?.setData(features());
		fit();
	}
	function fit() {
		if (!map || didFit || nodes.length === 0) return;
		const b = new maplibregl.LngLatBounds();
		for (const n of nodes) b.extend([n.longitude!, n.latitude!]);
		map.fitBounds(b, { padding: 56, maxZoom: 11, duration: 0 });
		didFit = true;
	}

	function addLayers() {
		if (!map || map.getSource('nodes')) return;
		map.addSource('nodes', { type: 'geojson', data: features() });
		map.addLayer({
			id: 'fav-halo',
			type: 'circle',
			source: 'nodes',
			filter: ['==', ['get', 'fav'], true],
			paint: {
				'circle-radius': ['interpolate', ['linear'], ['zoom'], 4, 6, 11, 11],
				'circle-color': 'transparent',
				'circle-stroke-color': FAV_COLOR,
				'circle-stroke-width': 2
			}
		});
		map.addLayer({
			id: 'node-dots',
			type: 'circle',
			source: 'nodes',
			paint: {
				'circle-radius': ['interpolate', ['linear'], ['zoom'], 4, 3.5, 11, 7],
				'circle-color': ['get', 'color'],
				'circle-stroke-color': 'rgba(0,0,0,0.4)',
				'circle-stroke-width': 1
			}
		});
		map.on('click', 'node-dots', (e) => {
			const f = e.features?.[0];
			const pk = f?.properties?.pubkey as string | undefined;
			if (pk) goto('/m/nodes/' + pk);
		});
		map.on('mouseenter', 'node-dots', () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
	}

	function ensureOverlays() {
		if (!map || !map.isStyleLoaded()) return;
		ensureHillshade(map, basemapLight);
		if (!map.getSource('nodes')) { addLayers(); updateSource(); }
	}

	// Theme swap → restyle basemap, re-add overlays on idle.
	$effect(() => {
		void theme.mode;
		if (!map) return;
		const light = theme.mode === 'light';
		if (light === basemapLight) return;
		basemapLight = light;
		map.setStyle(basemapStyleUrl(light));
		map.once('idle', ensureOverlays);
	});

	async function refresh() {
		try {
			nodes = locatedNodes(await api.nodes());
			updateSource();
		} catch {
			/* keep */
		}
	}

	onMount(() => {
		basemapLight = theme.mode === 'light';
		map = new maplibregl.Map({
			container: mapEl,
			style: basemapStyleUrl(basemapLight),
			center: [-123.65, 49.25],
			zoom: 7,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
		map.on('load', () => {
			if (!map) return;
			map.resize();
			ensureHillshade(map, basemapLight);
			addLayers();
			updateSource();
			collapseAttribution(map);
		});
		refresh();
		const t = setInterval(refresh, 15000);
		return () => { clearInterval(t); map?.remove(); map = null; };
	});
</script>

<div class="relative h-full w-full">
	<div bind:this={mapEl} class="h-full w-full"></div>
	<div class="border-line/60 bg-ink-2/80 pointer-events-none absolute top-3 left-3 z-10 rounded-full border px-3 py-1.5 backdrop-blur-md">
		<span class="text-fg-dim font-mono text-[0.62rem] tnum">{nodes.length} located nodes</span>
	</div>
</div>

<style>
	:global(.maplibregl-ctrl-attrib) {
		font-size: 9px;
	}
</style>
