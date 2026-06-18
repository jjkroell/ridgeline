<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { roleLabel } from '$lib/format';
	import { theme } from '$lib/theme.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { basemapStyleUrl } from '$lib/map-basemap';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import MapRoleFilter from '$lib/components/MapRoleFilter.svelte';
	import NodeModal from '$lib/components/NodeModal.svelte';

	const FAV_COLOR = '#e8b454'; // amber ring on favorited nodes

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let ready = false;
	let didFit = false;
	let allLocated = $state<Node[]>([]);
	let selectedRoles = $state(new Set(['Repeater', 'RoomServer', 'ChatNode', 'Sensor']));
	let nodeKey = $state<string | null>(null);

	const visible = $derived(allLocated.filter((n) => selectedRoles.has(n.role)));

	const ROLE_HEX: Record<string, string> = {
		Repeater: '#ff6b6b',
		ChatNode: '#5b9dff',
		RoomServer: '#6ee7a8',
		Sensor: '#e8b454',
		Observer: '#a78bfa'
	};

	const isLight = () => document.documentElement.classList.contains('theme-light');
	const inkColor = () =>
		getComputedStyle(document.documentElement).getPropertyValue('--color-ink').trim() || '#070a0e';

	function nodeFeatures(): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: visible.map((n) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [n.longitude!, n.latitude!] },
				properties: {
					role: n.role,
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
		(map?.getSource('nodes') as maplibregl.GeoJSONSource | undefined)?.setData(nodeFeatures());
	}

	// Re-add overlays after a basemap style swap (theme change) removes them.
	let basemapLight = false;
	function ensureOverlays() {
		if (!map || !map.isStyleLoaded() || map.getSource('nodes')) return;
		addLayers();
		updateSource();
	}

	$effect(() => {
		void theme.mode;
		const light = isLight();
		if (!map || light === basemapLight) return;
		basemapLight = light;
		map.setStyle(basemapStyleUrl(light));
	});

	// re-filter / re-style when the role selection or favorites change
	$effect(() => {
		void selectedRoles;
		void favorites.keys;
		if (ready) updateSource();
	});

	function addLayers() {
		if (!map) return;
		map.addSource('nodes', {
			type: 'geojson',
			data: nodeFeatures(),
			cluster: true,
			clusterRadius: 46,
			clusterMaxZoom: 11
		});
		map.addLayer({
			id: 'clusters',
			type: 'circle',
			source: 'nodes',
			filter: ['has', 'point_count'],
			paint: {
				'circle-color': '#159e8b',
				'circle-opacity': 0.85,
				'circle-radius': ['step', ['get', 'point_count'], 13, 10, 18, 30, 24],
				'circle-stroke-width': 1.5,
				'circle-stroke-color': '#34e3c4'
			}
		});
		map.addLayer({
			id: 'cluster-count',
			type: 'symbol',
			source: 'nodes',
			filter: ['has', 'point_count'],
			layout: {
				'text-field': ['get', 'point_count_abbreviated'],
				'text-font': ['Noto Sans Regular'],
				'text-size': 12
			},
			paint: { 'text-color': '#04140f' }
		});
		// Amber ring around favorited individual nodes, drawn beneath the dots.
		map.addLayer({
			id: 'fav-halo',
			type: 'circle',
			source: 'nodes',
			filter: ['all', ['!', ['has', 'point_count']], ['==', ['get', 'fav'], true]],
			paint: {
				// Ring hugging the dot: ~2px outside the node radius at each zoom.
				'circle-radius': [
					'interpolate', ['linear'], ['zoom'],
					6, ['match', ['get', 'role'], 'Repeater', 4.5, 3.8],
					11, ['match', ['get', 'role'], 'Repeater', 7, 5.2],
					15, ['match', ['get', 'role'], 'Repeater', 10, 7.5]
				],
				'circle-color': 'rgba(0,0,0,0)',
				'circle-stroke-color': FAV_COLOR,
				'circle-stroke-width': 1.75,
				'circle-stroke-opacity': 0.95
			}
		});
		map.addLayer({
			id: 'unclustered',
			type: 'circle',
			source: 'nodes',
			filter: ['!', ['has', 'point_count']],
			paint: {
				'circle-color': ['get', 'color'],
				// Smaller when zoomed out, scaling up as you zoom in.
				'circle-radius': [
					'interpolate', ['linear'], ['zoom'],
					6, ['match', ['get', 'role'], 'Repeater', 2.5, 1.8],
					11, ['match', ['get', 'role'], 'Repeater', 5, 3.2],
					15, ['match', ['get', 'role'], 'Repeater', 8, 5.5]
				],
				'circle-opacity': 0.9,
				'circle-stroke-width': 1,
				'circle-stroke-color': inkColor()
			}
		});
	}

	// Interaction handlers — bound once. MapLibre keeps layer-id listeners across
	// removeLayer/addLayer, so they survive a basemap style swap.
	function bindEvents() {
		if (!map) return;
		// Cluster click → zoom to expand.
		map.on('click', 'clusters', async (e) => {
			const f = map!.queryRenderedFeatures(e.point, { layers: ['clusters'] })[0];
			const id = f.properties!.cluster_id;
			const src = map!.getSource('nodes') as maplibregl.GeoJSONSource;
			const zoom = await src.getClusterExpansionZoom(id);
			map!.easeTo({ center: (f.geometry as GeoJSON.Point).coordinates as [number, number], zoom });
		});
		// Node click → full node-detail modal.
		map.on('click', 'unclustered', (e) => {
			const p = e.features![0].properties as { pubkey?: string };
			if (p?.pubkey) nodeKey = p.pubkey;
		});
		for (const layer of ['clusters', 'unclustered']) {
			map.on('mouseenter', layer, () => (map!.getCanvas().style.cursor = 'pointer'));
			map.on('mouseleave', layer, () => (map!.getCanvas().style.cursor = ''));
		}
	}

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
		const b = new maplibregl.LngLatBounds();
		pts.forEach((n) => b.extend([n.longitude!, n.latitude!]));
		map.fitBounds(b, { padding: 80, maxZoom: 12, duration: 600 });
	}

	async function plot() {
		if (!map) return;
		const nodes = await api.nodes();
		allLocated = nodes.filter(
			(n) =>
				n.hasLocation &&
				!n.gpsSuspect &&
				n.latitude != null &&
				n.longitude != null &&
				Math.abs(n.latitude) <= 90 &&
				Math.abs(n.longitude) <= 180
		);
		if (ready) updateSource();
		if (!didFit && allLocated.length > 0) {
			fitToNodes();
			didFit = true;
		}
	}

	onMount(() => {
		basemapLight = isLight();
		map = new maplibregl.Map({
			container: mapEl,
			style: basemapStyleUrl(basemapLight),
			center: [-123.65, 49.25],
			zoom: 9,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			addLayers();
			bindEvents();
			ready = true;
			plot();
		});
		// Re-add overlays after a basemap (theme) style swap drops them.
		map.on('styledata', ensureOverlays);
		const t = setInterval(plot, 10000);
		return () => {
			clearInterval(t);
			map?.remove();
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
	</div>
</div>

<NodeModal pubkey={nodeKey} onclose={() => (nodeKey = null)} />

<style>
	:global(.maplibregl-popup-content) {
		background: var(--color-panel);
		border: 1px solid var(--color-line-bright);
		border-radius: 2px;
		padding: 8px 10px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
	}
	:global(.maplibregl-popup-tip) {
		border-top-color: var(--color-line-bright) !important;
		border-bottom-color: var(--color-line-bright) !important;
	}
	:global(.maplibregl-ctrl-group) {
		background: var(--color-panel);
		border: 1px solid var(--color-line);
	}
	:global(.maplibregl-ctrl-group button + button) {
		border-top: 1px solid var(--color-line);
	}
</style>
