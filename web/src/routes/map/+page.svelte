<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { roleLabel } from '$lib/format';
	import { theme } from '$lib/theme.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import MapRoleFilter from '$lib/components/MapRoleFilter.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let ready = false;
	let didFit = false;
	let allLocated = $state<Node[]>([]);
	let selectedRoles = $state(new Set(['Repeater', 'RoomServer', 'ChatNode', 'Sensor']));

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
	const basemap = (light: boolean) =>
		(['a', 'b', 'c'] as const).map(
			(s) => `https://${s}.basemaps.cartocdn.com/${light ? 'light_all' : 'dark_all'}/{z}/{x}/{y}.png`
		);

	const style = (): maplibregl.StyleSpecification => ({
		version: 8,
		glyphs: 'https://fonts.openmaptiles.org/{fontstack}/{range}.pbf',
		sources: {
			carto: { type: 'raster', tiles: basemap(isLight()), tileSize: 256, attribution: '© OpenStreetMap © CARTO' }
		},
		layers: [
			{ id: 'bg', type: 'background', paint: { 'background-color': inkColor() } },
			{ id: 'carto', type: 'raster', source: 'carto', paint: { 'raster-opacity': 0.7 } }
		]
	});

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
					pubkey: n.publicKey
				}
			}))
		};
	}

	function updateSource() {
		(map?.getSource('nodes') as maplibregl.GeoJSONSource | undefined)?.setData(nodeFeatures());
	}

	function applyTheme() {
		if (!map || !map.isStyleLoaded()) return;
		(map.getSource('carto') as maplibregl.RasterTileSource | undefined)?.setTiles(basemap(isLight()));
		map.setPaintProperty('bg', 'background-color', inkColor());
		if (map.getLayer('unclustered')) map.setPaintProperty('unclustered', 'circle-stroke-color', inkColor());
	}

	$effect(() => {
		void theme.mode;
		applyTheme();
	});

	// re-filter when role selection changes
	$effect(() => {
		void selectedRoles;
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
		map.addLayer({
			id: 'unclustered',
			type: 'circle',
			source: 'nodes',
			filter: ['!', ['has', 'point_count']],
			paint: {
				'circle-color': ['get', 'color'],
				'circle-radius': ['match', ['get', 'role'], 'Repeater', 6, 4],
				'circle-opacity': 0.9,
				'circle-stroke-width': 1.5,
				'circle-stroke-color': inkColor()
			}
		});

		// Cluster click → zoom to expand.
		map.on('click', 'clusters', async (e) => {
			const f = map!.queryRenderedFeatures(e.point, { layers: ['clusters'] })[0];
			const id = f.properties!.cluster_id;
			const src = map!.getSource('nodes') as maplibregl.GeoJSONSource;
			const zoom = await src.getClusterExpansionZoom(id);
			map!.easeTo({ center: (f.geometry as GeoJSON.Point).coordinates as [number, number], zoom });
		});
		// Node click → popup.
		map.on('click', 'unclustered', (e) => {
			const f = e.features![0];
			const p = f.properties as { name: string; roleLabel: string; color: string };
			new maplibregl.Popup({ offset: 12, closeButton: false })
				.setLngLat((f.geometry as GeoJSON.Point).coordinates as [number, number])
				.setHTML(
					`<div style="font-family:'Space Mono',monospace;font-size:12px">
						<div style="color:var(--color-fg);font-weight:700">${p.name}</div>
						<div style="color:${p.color};margin-top:2px">${p.roleLabel}</div>
					</div>`
				)
				.addTo(map!);
		});
		for (const layer of ['clusters', 'unclustered']) {
			map.on('mouseenter', layer, () => (map!.getCanvas().style.cursor = 'pointer'));
			map.on('mouseleave', layer, () => (map!.getCanvas().style.cursor = ''));
		}
	}

	async function plot() {
		if (!map) return;
		const nodes = await api.nodes();
		allLocated = nodes.filter(
			(n) =>
				n.hasLocation &&
				n.latitude != null &&
				n.longitude != null &&
				Math.abs(n.latitude) <= 90 &&
				Math.abs(n.longitude) <= 180
		);
		if (ready) updateSource();
		if (!didFit && allLocated.length > 0) {
			const b = new maplibregl.LngLatBounds();
			allLocated.forEach((n) => b.extend([n.longitude!, n.latitude!]));
			map.fitBounds(b, { padding: 80, maxZoom: 12, duration: 600 });
			didFit = true;
		}
	}

	onMount(() => {
		map = new maplibregl.Map({
			container: mapEl,
			style: style(),
			center: [-123.65, 49.25],
			zoom: 9,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			addLayers();
			applyTheme();
			ready = true;
			plot();
		});
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
	<div class="panel overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<div bind:this={mapEl} class="h-full w-full"></div>
		<MapRoleFilter bind:selected={selectedRoles} />
	</div>
</div>

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
