<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { goto } from '$app/navigation';
	import maplibregl from 'maplibre-gl';
	import { sosSpots, SosRings } from '$lib/sos-map';
	import { sos } from '$lib/sos.svelte';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { live, hashColor } from '$lib/live.svelte';
	import { theme } from '$lib/theme.svelte';
	import { basemapStyle, basemapHasHillshade, basemapHasLocalTerrain, collapseAttribution, ensureLocalTerrain, basemapHasTint, ensureTint } from '$lib/map-basemap';
	import { basemap } from '$lib/basemap.svelte';
	import { ensureHillshade } from '$lib/map-hillshade';
	import { ROLE_HEX, SEGMENT_COLOR, locatedNodes } from '$lib/map-util';
	import BasemapSelector from '$lib/components/BasemapSelector.svelte';
	import CopyMapLink from '$lib/components/CopyMapLink.svelte';
	import { addressBarUrl } from '$lib/address-bar';
	import { parseMapView, hasMapView, shareUrl, withSavedCamera, saveCamera } from '$lib/map-share';
	import FallbackMap from '$lib/components/FallbackMap.svelte';
	import { hasWebGL } from '$lib/webgl';
	import NodeSheet from '$lib/mobile/NodeSheet.svelte';
	import LivePill from '$lib/components/LivePill.svelte';
	import { seasonal } from '$lib/seasonal.svelte';
	import { liveSpeed, speedLabel } from '$lib/live-speed.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	// Pulsing red rings over the nodes of recent #sos alert senders.
	let sosRings = $state<SosRings | null>(null);
	$effect(() => sosRings?.update(sosSpots(nodes, sos.ringAlerts)));
	let webglOk = $state(true);
	let fbBanner = $state(true); // WebGL-free banner open? (offsets the basemap selector below it)
	let nodes = $state<Node[]>([]);
	// A tapped node opens a bottom sheet over the map (the map stays in view);
	// its Details button goes on to the full node page.
	let sheetNode = $state<Node | null>(null);
	function openNode(pk: string) {
		sheetNode = nodes.find((n) => n.publicKey === pk) ?? null;
		if (!sheetNode) goto('/m/nodes/' + pk);
	}
	let basemapLight = false;
	let didFit = false;

	// Shareable view state. A link frames the map (suppressing the auto-fit)
	// and previews its basemap without persisting it over the viewer's own.
	const incoming = withSavedCamera(parseMapView(addressBarUrl(page.url)), 'm-live-map');
	const fromLink = hasMapView(incoming);
	function currentView() {
		const c = map?.getCenter();
		return { lat: c?.lat, lon: c?.lng, zoom: map?.getZoom(), basemap: basemap.id };
	}
	function linkUrl() {
		return shareUrl(addressBarUrl(page.url), currentView());
	}
	function syncUrl() {
		if (!map) return;
		replaceState(shareUrl(addressBarUrl(page.url), currentView()), {});
		saveCamera('m-live-map', currentView());
	}
	let pulseCount = $state(0);

	// resolve a path-hop prefix to a located node's coordinates (unique prefix)
	function resolveHop(hop: string): [number, number] | null {
		const h = hop.toUpperCase();
		const hit = nodes.filter((n) => n.publicKey.toUpperCase().startsWith(h));
		return hit.length === 1 ? [hit[0].longitude!, hit[0].latitude!] : null;
	}

	function nodeFeatures(): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: nodes.map((n) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [n.longitude!, n.latitude!] },
				properties: { color: ROLE_HEX[n.role] ?? '#8394a1', pubkey: n.publicKey, offSegment: !!n.viaBridge }
			}))
		};
	}

	// active pulses: a node coord + colour + start time
	// dur: PULSE_MS / TRAIL_MS scaled by the speed setting when the packet arrived.
	type Pulse = { lng: number; lat: number; color: string; kind: string; start: number; dur: number };
	let pulses: Pulse[] = [];
	const PULSE_MS = 1600;

	// active trails: the polyline through a packet's resolved path hops, fading out.
	type Trail = { coords: [number, number][]; color: string; start: number; dur: number };
	let trails: Trail[] = [];
	const TRAIL_MS = 2600;

	function trailFeatures(now: number): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: trails.map((t) => {
				const age = (now - t.start) / t.dur; // 0..1
				return {
					type: 'Feature',
					geometry: { type: 'LineString', coordinates: t.coords },
					properties: { color: t.color, o: Math.max(0, 1 - age) * 0.85 }
				};
			})
		};
	}

	function pulseFeatures(now: number): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: pulses.map((p) => {
				const age = (now - p.start) / p.dur; // 0..1
				return {
					type: 'Feature',
					geometry: { type: 'Point', coordinates: [p.lng, p.lat] },
					// icon/s/rot: the season's icon that pops at each hop, growing as it fades.
					properties: {
						color: p.color,
						r: 4 + age * 22,
						o: Math.max(0, 1 - age),
						icon: seasonal.imageId(p.kind),
						s: 0.55 + Math.min(1, age * 3) * 0.6,
						rot: Math.sin(now / 160 + p.lng * 40) * 14
					}
				};
			})
		};
	}

	function spawn(ev: { path?: string[]; payloadType: string; messageHash: string }) {
		if (!ev.path?.length) return;
		// Colour per message hash, matching the desktop comets (and the Recent
		// Packets flag there); keeps the propagation colours consistent app-wide.
		const color = hashColor(ev.messageHash);
		const now = performance.now();
		const speed = liveSpeed.value || 1;
		const coords: [number, number][] = [];
		ev.path.forEach((hop, i) => {
			const c = resolveHop(hop);
			if (c) {
				coords.push(c);
				pulses.push({ lng: c[0], lat: c[1], color, kind: ev.payloadType, start: now + (i * 180) / speed, dur: PULSE_MS / speed });
			}
		});
		if (coords.length >= 2) trails.push({ coords, color, start: now, dur: TRAIL_MS / speed });
	}

	let raf = 0;
	function frame() {
		const now = performance.now();
		pulses = pulses.filter((p) => now - p.start < p.dur);
		trails = trails.filter((t) => now - t.start < t.dur);
		pulseCount = pulses.length;
		(map?.getSource('pulses') as maplibregl.GeoJSONSource | undefined)?.setData(pulseFeatures(now));
		(map?.getSource('trails') as maplibregl.GeoJSONSource | undefined)?.setData(trailFeatures(now));
		raf = requestAnimationFrame(frame);
	}

	// watch the live store for new events
	let lastSeen = 0;
	$effect(() => {
		const evs = live.events;
		void evs.length;
		if (!map) return;
		const now = Date.now();
		for (const ev of evs) {
			const t = +new Date(ev.receivedAt);
			if (t > lastSeen && now - t < 20000) spawn(ev);
		}
		lastSeen = Math.max(lastSeen, ...evs.map((e) => +new Date(e.receivedAt)), lastSeen);
	});

	function addLayers() {
		if (!map || map.getSource('nodes')) return;
		map.addSource('nodes', { type: 'geojson', data: nodeFeatures() });
		map.addSource('pulses', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
		map.addSource('trails', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
		map.addLayer({
			id: 'pulse-lines', type: 'line', source: 'trails',
			layout: { 'line-cap': 'round', 'line-join': 'round' },
			paint: { 'line-color': ['get', 'color'], 'line-width': 2, 'line-opacity': ['get', 'o'], 'line-blur': 0.5 }
		});
		map.addLayer({
			id: 'pulse-rings', type: 'circle', source: 'pulses',
			paint: { 'circle-radius': ['get', 'r'], 'circle-color': 'transparent', 'circle-stroke-color': ['get', 'color'], 'circle-stroke-width': 2, 'circle-stroke-opacity': ['get', 'o'] }
		});
		map.addLayer({
			id: 'node-dots', type: 'circle', source: 'nodes',
			// Role stays the fill; a node beyond a bridge gets a violet ring. These dots
			// have no stroke by default, so the ring is added only where it means something.
			paint: { 'circle-radius': ['interpolate', ['linear'], ['zoom'], 4, 2.5, 11, 5], 'circle-color': ['get', 'color'], 'circle-opacity': 0.85, 'circle-stroke-color': SEGMENT_COLOR, 'circle-stroke-width': ['case', ['get', 'offSegment'], 1.5, 0] }
		});
		// Seasonal theme: the season's icon (pumpkin, bat…) pops up at each node as the packet reaches it.
		if (seasonal.on) seasonal.ensureImages(map);
		map.addLayer({
			id: 'pulse-season', type: 'symbol', source: 'pulses',
			layout: {
				'icon-image': ['get', 'icon'], 'icon-size': ['get', 's'], 'icon-rotate': ['get', 'rot'],
				'icon-rotation-alignment': 'viewport', 'icon-allow-overlap': true, 'icon-ignore-placement': true,
				visibility: seasonal.on ? 'visible' : 'none'
			},
			paint: { 'icon-opacity': ['get', 'o'] }
		});
		// Transparent, larger hit target so the small dots are tappable on a phone.
		map.addLayer({
			id: 'node-hit', type: 'circle', source: 'nodes',
			paint: { 'circle-radius': 13, 'circle-color': 'transparent' }
		});
		map.on('click', 'node-hit', (e) => {
			const pk = e.features?.[0]?.properties?.pubkey as string | undefined;
			if (pk) openNode(pk);
		});
		// Tapping empty map dismisses the node sheet.
		map.on('click', (e) => {
			if (sheetNode && !map?.queryRenderedFeatures(e.point, { layers: ['node-hit'] }).length) sheetNode = null;
		});
		map.on('mouseenter', 'node-hit', () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
	}
	let currentBasemap = basemap.id;
	function ensureOverlays() {
		if (!map || !map.isStyleLoaded()) return;
		if (basemapHasHillshade(currentBasemap)) ensureHillshade(map, basemapLight);
		if (basemapHasLocalTerrain(currentBasemap)) ensureLocalTerrain(map, basemapLight);
		if (basemapHasTint(currentBasemap)) ensureTint(map, basemapLight);
		if (!map.getSource('nodes')) addLayers();
	}
	function fit() {
		if (!map || didFit || fromLink || nodes.length === 0) return;
		const b = new maplibregl.LngLatBounds();
		for (const n of nodes) b.extend([n.longitude!, n.latitude!]);
		map.fitBounds(b, { padding: 56, maxZoom: 11, duration: 0 });
		didFit = true;
	}

	$effect(() => {
		void theme.id;
		if (!map) return;
		const light = theme.isLight;
		if (light === basemapLight) return;
		basemapLight = light;
		map.setStyle(basemapStyle(currentBasemap, light));
		map.once('idle', ensureOverlays);
	});

	// Swap the basemap when the user picks a different one.
	$effect(() => {
		const id = basemap.id;
		if (!map || id === currentBasemap) return;
		currentBasemap = id;
		basemapLight = theme.isLight;
		map.setStyle(basemapStyle(id, basemapLight));
		map.once('idle', ensureOverlays);
	});

	async function refresh() {
		try {
			nodes = locatedNodes(await api.nodes());
			(map?.getSource('nodes') as maplibregl.GeoJSONSource | undefined)?.setData(nodeFeatures());
			fit();
		} catch {
			/* keep */
		}
	}

	$effect(() => {
		const on = seasonal.on;
		void seasonal.season;
		if (!map?.getLayer('pulse-season')) return;
		if (on) seasonal.ensureImages(map);
		map.setLayoutProperty('pulse-season', 'visibility', on ? 'visible' : 'none');
	});

	onMount(() => {
		seasonal.load();
		liveSpeed.load();
		basemap.init();
		if (incoming.basemap) basemap.preview(incoming.basemap);
		webglOk = hasWebGL();
		if (!webglOk) {
			refresh();
			const t = setInterval(refresh, 30000);
			return () => clearInterval(t);
		}
		currentBasemap = basemap.id;
		basemapLight = theme.isLight;
		map = new maplibregl.Map({ container: mapEl, style: basemapStyle(currentBasemap, basemapLight), center: [incoming.lon ?? -123.65, incoming.lat ?? 49.25], zoom: incoming.zoom ?? 7, attributionControl: { compact: true } });
		map.on('moveend', syncUrl);
		map.addControl(new maplibregl.NavigationControl({ showCompass: true, visualizePitch: true }), 'bottom-right');
		sosRings = new SosRings(map);
		map.on('load', () => {
			if (!map) return;
			map.resize();
			if (basemapHasHillshade(currentBasemap)) ensureHillshade(map, basemapLight);
			if (basemapHasLocalTerrain(currentBasemap)) ensureLocalTerrain(map, basemapLight);
			if (basemapHasTint(currentBasemap)) ensureTint(map, basemapLight);
			addLayers();
			collapseAttribution(map);
			refresh();
			raf = requestAnimationFrame(frame);
		});
		const t = setInterval(refresh, 15000);
		return () => { clearInterval(t); cancelAnimationFrame(raf); map?.remove(); map = null; };
	});
</script>

<Seo title="Live Map" description="Packets on the map as they arrive." path="/m/live-map" />

<div class="relative h-full w-full">
	{#if !webglOk}
		<FallbackMap
			nodes={nodes}
			center={[-123.65, 49.25]}
			zoom={7}
			live
			bind:bannerOpen={fbBanner}
			onselect={(k) => openNode(k)}
			notice="WebGL is disabled — showing the basic live map. Enable WebGL for terrain and the full-fidelity animation."
		/>
		<BasemapSelector compact posClass={fbBanner ? 'top-16 left-3' : 'top-3 left-3'} />
	{:else}
	<div bind:this={mapEl} data-map-canvas class="h-full w-full"></div>
	<div class="absolute top-3 left-3 z-20"><CopyMapLink url={linkUrl} compact /></div>
	<div class="pointer-events-none absolute top-3 right-3 left-[7.5rem] z-10 flex justify-center">
		<LivePill detail="{nodes.length} nodes on the map · {pulseCount} animating now" onnode={openNode} />
	</div>
	<BasemapSelector compact posClass="top-14 left-3" />
	{/if}
	<!-- Bottom-left: packet speed (tap to step ¼× … 3×) and the seasonal theme toggle (only in season). -->
	<div class="absolute bottom-3 left-3 z-20 flex gap-2">
	<button
		onclick={() => liveSpeed.cycle()}
		aria-label="Packet speed {speedLabel(liveSpeed.value)}, tap to change"
		class="border-line/60 bg-ink-2/85 flex h-10 items-center gap-1.5 rounded-full border px-3 text-xs font-medium shadow-lg backdrop-blur-md {liveSpeed.value === 1
			? 'text-fg-dim'
			: 'text-signal'}"
	>
		<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 13l4-4" /><path d="M3.5 17a9 9 0 1 1 17 0" /></svg>
		<span class="tnum">{speedLabel(liveSpeed.value)}</span>
	</button>
	{#if seasonal.season}
		{@const ss = seasonal.season}
		<button
			onclick={() => seasonal.toggle()}
			aria-pressed={seasonal.on}
			aria-label={seasonal.on ? `Turn off ${ss.label.toLowerCase()}` : `Turn on ${ss.label.toLowerCase()}`}
			class="border-line/60 bg-ink-2/85 flex h-10 items-center gap-1.5 rounded-full border px-3 text-xs font-medium shadow-lg backdrop-blur-md transition-colors {seasonal.on
				? ''
				: 'text-fg-dim'}"
			style={seasonal.on ? `border-color:color-mix(in srgb, ${ss.accent} 60%, transparent);color:${ss.accent}` : ''}
		>
			<span class="text-base leading-none {seasonal.on ? '' : 'opacity-60 grayscale'}" aria-hidden="true">{ss.emoji}</span>
			{seasonal.on ? `${ss.short} on` : ss.short}
		</button>
	{/if}
	</div>
	{#if sheetNode}
		<NodeSheet node={sheetNode} onclose={() => (sheetNode = null)} />
	{/if}
</div>
