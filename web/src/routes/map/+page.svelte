<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import { api, type Node } from '$lib/api';
	import { roleColor } from '$lib/format';
	import { theme } from '$lib/theme.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let markers: maplibregl.Marker[] = [];
	let located = $state(0);

	// The applied theme is authoritative via the <html> class (set by an
	// inline script before paint), which is ready earlier than the theme
	// store. Derive map colors from it so the basemap is correct at creation.
	const isLight = () => document.documentElement.classList.contains('theme-light');

	const basemap = (light: boolean) =>
		(['a', 'b', 'c'] as const).map(
			(s) => `https://${s}.basemaps.cartocdn.com/${light ? 'light_all' : 'dark_all'}/{z}/{x}/{y}.png`
		);

	const inkColor = () =>
		getComputedStyle(document.documentElement).getPropertyValue('--color-ink').trim() || '#070a0e';

	const style = (): maplibregl.StyleSpecification => ({
		version: 8,
		sources: {
			carto: {
				type: 'raster',
				tiles: basemap(isLight()),
				tileSize: 256,
				attribution: '© OpenStreetMap © CARTO'
			}
		},
		layers: [
			{ id: 'bg', type: 'background', paint: { 'background-color': inkColor() } },
			{ id: 'carto', type: 'raster', source: 'carto', paint: { 'raster-opacity': 0.7 } }
		]
	});

	// Re-apply the current theme's basemap. No-op until the style has loaded
	// (MapLibre throws if mutated earlier).
	function applyTheme() {
		if (!map || !map.isStyleLoaded()) return;
		(map.getSource('carto') as maplibregl.RasterTileSource | undefined)?.setTiles(basemap(isLight()));
		map.setPaintProperty('bg', 'background-color', inkColor());
	}

	// React to live theme toggles (theme.mode change triggers the effect).
	$effect(() => {
		void theme.mode;
		applyTheme();
	});

	function markerEl(node: Node): HTMLElement {
		const c = roleColor(node.role);
		const el = document.createElement('div');
		el.style.cssText = `width:12px;height:12px;border-radius:999px;background:${c};box-shadow:0 0 0 2px ${inkColor()},0 0 12px ${c};cursor:pointer`;
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
					<div style="color:var(--color-fg);font-weight:700">${n.name || n.publicKey.slice(0, 10)}</div>
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
			style: style(),
			center: [-123.65, 49.25],
			zoom: 9,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			applyTheme();
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
		<span class="text-signal tnum">{located}</span> <span class="text-fg-faint">located nodes</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	<div class="panel overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<!-- Full-size block child: MapLibre's CSS forces position:relative on
		     this element, so it must carry its own height rather than rely on
		     absolute insets. -->
		<div bind:this={mapEl} class="h-full w-full"></div>
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
