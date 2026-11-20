<script lang="ts">
	// Geographic layout of the relay backbone: every located relay at its
	// advertised position, hand-off links drawn as straight lines between them.
	// Same rules as the force graph — links filtered by minWeight (plus each
	// node's strongest), colour = forwarding volume, focus shows all links.
	import { onMount, untrack } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { FeatureCollection } from 'geojson';
	import type { TopologyNode, TopologyEdge } from '$lib/api';
	import { basemapStyle, collapseAttribution } from '$lib/map-basemap';
	import { basemap } from '$lib/basemap.svelte';
	import { isLight } from '$lib/map-util';
	import { attachHoverLabel } from '$lib/map-hover';
	import { visibleEdges, relayNorm, relayRanks, topoPalette, ROLE_SHAPE } from '$lib/topology';

	let {
		nodes,
		edges,
		minWeight = 1,
		selected = null,
		onselect
	}: {
		nodes: TopologyNode[];
		edges: TopologyEdge[];
		minWeight?: number;
		selected?: string | null;
		onselect?: (key: string | null) => void;
	} = $props();

	let el: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let ready = $state(false);

	const located = $derived(nodes.filter((n) => n.lat != null && n.lon != null));
	const pos = $derived(new Map(located.map((n) => [n.publicKey, [n.lon!, n.lat!] as [number, number]])));
	const maxRelayed = $derived(Math.max(1, ...nodes.map((n) => n.relayed)));
	const ranks = $derived(relayRanks(nodes));
	const maxW = $derived(Math.max(1, ...edges.map((e) => e.weight)));

	function linkFC(list: TopologyEdge[]): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: list
				.filter((e) => pos.has(e.a) && pos.has(e.b))
				.sort((x, y) => x.weight - y.weight)
				.map((e) => ({
					type: 'Feature',
					properties: { t: Math.sqrt(e.weight / maxW), inf: e.inferred ? 1 : 0 },
					geometry: { type: 'LineString', coordinates: [pos.get(e.a)!, pos.get(e.b)!] }
				}))
		};
	}
	function nodeFC(): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: located.map((n) => ({
				type: 'Feature',
				properties: {
					pubkey: n.publicKey,
					name: n.name,
					t: relayNorm(n, maxRelayed),
					c: ranks.get(n.publicKey) ?? 0,
					odd: (ROLE_SHAPE[n.role] ?? 'circle') !== 'circle' ? 1 : 0,
					sel: n.publicKey === selected ? 1 : 0
				},
				geometry: { type: 'Point', coordinates: pos.get(n.publicKey)! }
			}))
		};
	}

	function setData() {
		if (!map || !ready) return;
		const vis = visibleEdges(edges, minWeight);
		const focus = selected ? edges.filter((e) => e.a === selected || e.b === selected) : [];
		(map.getSource('topo-links') as maplibregl.GeoJSONSource)?.setData(
			linkFC(selected ? vis.filter((e) => !focus.includes(e)) : vis)
		);
		(map.getSource('topo-focus') as maplibregl.GeoJSONSource)?.setData(linkFC(focus));
		(map.getSource('topo-nodes') as maplibregl.GeoJSONSource)?.setData(nodeFC());
		map.setPaintProperty('topo-links', 'line-opacity', selected ? 0.12 : ['+', 0.15, ['*', 0.6, ['get', 't']]]);
		map.setPaintProperty('topo-links-inf', 'line-opacity', selected ? 0.12 : ['+', 0.15, ['*', 0.6, ['get', 't']]]);
	}

	$effect(() => {
		void minWeight;
		void selected;
		void edges;
		untrack(setData);
	});

	function addLayers() {
		if (!map) return;
		const pal = topoPalette();
		const link = `rgb(${pal.link.join(',')})`;
		const empty: FeatureCollection = { type: 'FeatureCollection', features: [] };
		map.addSource('topo-links', { type: 'geojson', data: empty });
		map.addSource('topo-focus', { type: 'geojson', data: empty });
		map.addSource('topo-nodes', { type: 'geojson', data: empty });
		const width: maplibregl.ExpressionSpecification = ['+', 0.6, ['*', 3.6, ['get', 't']]];
		map.addLayer({
			id: 'topo-links',
			type: 'line',
			source: 'topo-links',
			filter: ['==', ['get', 'inf'], 0],
			layout: { 'line-cap': 'round' },
			paint: { 'line-color': link, 'line-width': width, 'line-opacity': 0.5 }
		});
		map.addLayer({
			id: 'topo-links-inf',
			type: 'line',
			source: 'topo-links',
			filter: ['==', ['get', 'inf'], 1],
			paint: { 'line-color': link, 'line-width': width, 'line-opacity': 0.5, 'line-dasharray': [2, 2] }
		});
		map.addLayer({
			id: 'topo-focus',
			type: 'line',
			source: 'topo-focus',
			layout: { 'line-cap': 'round' },
			paint: { 'line-color': pal.focus, 'line-width': ['+', 1, ['*', 3.6, ['get', 't']]], 'line-opacity': 0.95 }
		});
		map.addLayer({
			id: 'topo-nodes',
			type: 'circle',
			source: 'topo-nodes',
			paint: {
				'circle-radius': ['+', 3.5, ['*', 8, ['get', 't']]],
				'circle-color': ['interpolate', ['linear'], ['get', 'c'], 0, pal.ramp[0], 0.5, pal.ramp[1], 1, pal.ramp[2]],
				// rooms / companions / sensors get a bright ring so they stand out
				'circle-stroke-color': ['case', ['==', ['get', 'sel'], 1], pal.label, ['==', ['get', 'odd'], 1], pal.label, pal.ink],
				'circle-stroke-width': ['case', ['==', ['get', 'sel'], 1], 3, ['==', ['get', 'odd'], 1], 2, 1.2]
			}
		});
		// Persistent hub labels need glyphs; the raster basemaps don't carry any.
		if (map.getStyle().glyphs) {
			map.addLayer({
				id: 'topo-labels',
				type: 'symbol',
				source: 'topo-nodes',
				layout: {
					'text-field': ['get', 'name'],
					'text-font': ['Noto Sans Regular'],
					'text-size': 11,
					'text-offset': [0.9, 0],
					'text-anchor': 'left',
					'symbol-sort-key': ['-', 1, ['get', 't']],
					// only the busier half earns a permanent label; collision does the rest
					'text-optional': true
				},
				filter: ['any', ['>=', ['get', 't'], 0.45], ['==', ['get', 'sel'], 1]],
				paint: { 'text-color': pal.label, 'text-halo-color': pal.ink, 'text-halo-width': 1.4 }
			});
		}
		attachHoverLabel(map, 'topo-nodes');
		// One click handler: a node under the pointer toggles it, empty map clears.
		map.on('click', (e) => {
			const hit = map!.queryRenderedFeatures(e.point, { layers: ['topo-nodes'] })[0];
			const k = (hit?.properties as { pubkey?: string } | undefined)?.pubkey ?? null;
			onselect?.(k && k !== selected ? k : null);
		});
		map.on('mouseenter', 'topo-nodes', () => (map!.getCanvas().style.cursor = 'pointer'));
		map.on('mouseleave', 'topo-nodes', () => (map!.getCanvas().style.cursor = ''));
	}

	function fitAll() {
		if (!map || located.length === 0) return;
		const b = new maplibregl.LngLatBounds();
		for (const n of located) b.extend([n.lon!, n.lat!]);
		map.fitBounds(b, { padding: { top: 60, bottom: 60, left: 60, right: selected ? 320 : 60 }, duration: 0, maxZoom: 11 });
	}

	onMount(() => {
		basemap.init();
		map = new maplibregl.Map({
			container: el,
			style: basemapStyle(basemap.id, isLight()),
			center: [-123.65, 49.25],
			zoom: 8,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			collapseAttribution(map!);
			addLayers();
			ready = true;
			setData();
			fitAll();
		});
		return () => {
			map?.remove();
			map = null;
		};
	});
</script>

<div class="relative h-full w-full overflow-hidden">
	<div bind:this={el} class="h-full w-full"></div>
	<button
		onclick={fitAll}
		class="border-line bg-panel/85 text-fg-dim hover:border-signal/50 hover:text-signal absolute top-3 right-3 rounded-[var(--radius)] border px-2.5 py-1 font-mono text-[0.68rem] font-600 backdrop-blur transition-colors"
		>⟲ Fit all</button
	>
	{#if located.length < nodes.length}
		<div
			class="border-line/70 bg-panel/85 text-fg-faint absolute top-3 left-3 rounded-[var(--radius)] border px-2.5 py-1 font-mono text-[0.62rem] backdrop-blur"
		>
			{nodes.length - located.length} relay{nodes.length - located.length === 1 ? '' : 's'} without a location not shown
		</div>
	{/if}
</div>
