<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import { api, type Node } from '$lib/api';
	import { roleColor } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let markers: maplibregl.Marker[] = [];
	let located = $state(0);

	const style: maplibregl.StyleSpecification = {
		version: 8,
		sources: {
			carto: {
				type: 'raster',
				tiles: [
					'https://a.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png',
					'https://b.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png',
					'https://c.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png'
				],
				tileSize: 256,
				attribution: '© OpenStreetMap © CARTO'
			}
		},
		layers: [
			{ id: 'bg', type: 'background', paint: { 'background-color': '#070a0e' } },
			{ id: 'carto', type: 'raster', source: 'carto', paint: { 'raster-opacity': 0.65 } }
		]
	};

	function markerEl(node: Node): HTMLElement {
		const c = roleColor(node.role);
		const el = document.createElement('div');
		el.style.cssText = `width:12px;height:12px;border-radius:999px;background:${c};box-shadow:0 0 0 2px rgba(7,10,14,.9),0 0 12px ${c};cursor:pointer`;
		el.title = node.name || node.publicKey;
		return el;
	}

	async function plot() {
		if (!map) return;
		const nodes = await api.nodes();
		const withLoc = nodes.filter((n) => n.hasLocation && n.latitude != null && n.longitude != null);
		located = withLoc.length;

		markers.forEach((m) => m.remove());
		markers = withLoc.map((n) => {
			const popup = new maplibregl.Popup({ offset: 14, closeButton: false }).setHTML(
				`<div style="font-family:'Space Mono',monospace;font-size:12px">
					<div style="color:#dde7ed;font-weight:700">${n.name || n.publicKey.slice(0, 10)}</div>
					<div style="color:${roleColor(n.role)};margin-top:2px">${n.role}</div>
				</div>`
			);
			return new maplibregl.Marker({ element: markerEl(n) })
				.setLngLat([n.longitude!, n.latitude!])
				.setPopup(popup)
				.addTo(map!);
		});

		if (withLoc.length > 0) {
			const b = new maplibregl.LngLatBounds();
			withLoc.forEach((n) => b.extend([n.longitude!, n.latitude!]));
			map.fitBounds(b, { padding: 80, maxZoom: 12, duration: 600 });
		}
	}

	onMount(() => {
		map = new maplibregl.Map({
			container: mapEl,
			style,
			center: [-123.65, 49.25],
			zoom: 9,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', plot);
		const t = setInterval(plot, 10000);
		return () => {
			clearInterval(t);
			map?.remove();
		};
	});
</script>

<PageHeader eyebrow="Terrain & Topology" title="Network Map">
	<div class="font-mono text-fg-dim text-xs">
		<span class="text-signal tnum">{located}</span> <span class="text-fg-faint">located nodes</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	<div class="panel relative overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<div bind:this={mapEl} class="absolute inset-0"></div>
	</div>
</div>

<style>
	:global(.maplibregl-popup-content) {
		background: #0e141b;
		border: 1px solid #2b3b48;
		border-radius: 2px;
		padding: 8px 10px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
	}
	:global(.maplibregl-popup-tip) {
		border-top-color: #2b3b48 !important;
		border-bottom-color: #2b3b48 !important;
	}
	:global(.maplibregl-ctrl-group) {
		background: #0e141b;
		border: 1px solid #1c2730;
	}
	:global(.maplibregl-ctrl-group button + button) {
		border-top: 1px solid #1c2730;
	}
</style>
