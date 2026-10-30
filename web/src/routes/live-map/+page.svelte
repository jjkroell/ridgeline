<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import Seo from '$lib/components/Seo.svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { Feature, FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import LivePill from '$lib/components/LivePill.svelte';
	import { live, groupLive, hashColor, type LiveGroup } from '$lib/live.svelte';
	import { theme } from '$lib/theme.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { basemapStyle, basemapHasHillshade, basemapHasLocalTerrain, collapseAttribution, ensureLocalTerrain, basemapHasTint, ensureTint } from '$lib/map-basemap';
	import { basemap } from '$lib/basemap.svelte';
	import { ensureHillshade } from '$lib/map-hillshade';
	import { isLight, inkColor, ROLE_HEX, FAV_COLOR, SEGMENT_COLOR, locatedNodes } from '$lib/map-util';
	import { attachHoverLabel } from '$lib/map-hover';
	import { ago, shortKey, fmtSnr, snrColor } from '$lib/format';
	import { PulseEngine } from '$lib/live-pulse';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import LiveGroupModal from '$lib/components/LiveGroupModal.svelte';
	import MapRoleFilter from '$lib/components/MapRoleFilter.svelte';
	import BasemapSelector from '$lib/components/BasemapSelector.svelte';
	import CopyMapLink from '$lib/components/CopyMapLink.svelte';
	import { addressBarUrl } from '$lib/address-bar';
	import { parseMapView, shareUrl, withSavedCamera, saveCamera } from '$lib/map-share';
	import NodeInspector from '$lib/components/NodeInspector.svelte';
	import FallbackMap from '$lib/components/FallbackMap.svelte';
	import ChimeControls from '$lib/components/ChimeControls.svelte';
	import { chime } from '$lib/live-audio.svelte';
	import { hasWebGL } from '$lib/webgl';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let webglOk = $state(true);
	let fbBanner = $state(true); // WebGL-free banner open? (offsets the Map Control panel below it)
	let ready = false;
	let nodes: Node[] = [];
	let located: Node[] = $state([]); // all located nodes (pulse resolution uses these)
	let animCount = $state(0);
	let selectedRoles = $state(new Set(['Repeater', 'RoomServer', 'ChatNode', 'Sensor']));

	// ── Shareable view state ──────────────────────────────────────────────────
	// A link frames the map and applies its basemap/role filter for this visit
	// only; basemap.preview() deliberately does not persist, so the recipient's
	// own saved choice is untouched.
	const incoming = withSavedCamera(parseMapView(addressBarUrl(page.url)), 'live-map');

	function currentView() {
		const c = map?.getCenter();
		return {
			lat: c?.lat,
			lon: c?.lng,
			zoom: map?.getZoom(),
			basemap: basemap.id,
			roles: [...selectedRoles]
		};
	}
	function linkUrl() {
		return shareUrl(addressBarUrl(page.url), currentView());
	}
	function syncUrl() {
		if (!map) return;
		replaceState(shareUrl(addressBarUrl(page.url), currentView()), {});
		saveCamera('live-map', currentView());
	}

	// Overlay collapse state — both default minimized, persisted across visits.
	const MAPCTRL_KEY = 'ridgeline-livemap-mapcontrol';
	const PANEL_KEY = 'ridgeline-livemap-recent';
	let mapCtrlOpen = $state(false);
	let overlaysReady = $state(false); // gate persistence until localStorage is loaded

	// Recent packets overlay (grouped, max 15) with minimize/maximize.
	let panelOpen = $state(false);
	let selected = $state<LiveGroup | null>(null);
	// ?node=<pubkey> opens that node's panel, as on /map.
	let nodeKey = $state<string | null>(page.url.searchParams.get('node'));
	const recent = $derived(groupLive(live.events).slice(0, 15));

	// ── propagation pulses (shared, renderer-agnostic engine) ──────────────
	// The pulse geometry (hop resolution, comet animation, node ripples) lives in
	// $lib/live-pulse so the WebGL-free Leaflet fallback draws the exact same
	// animation onto a 2-D canvas; here we just feed its output into MapLibre.
	// Comets are coloured per message hash so each pulse matches the colour flag
	// on its Recent Packets row below.
	const engine = new PulseEngine((ev) => hashColor(ev.messageHash));

	// Pulse each newly-seen header path as it arrives.
	$effect(() => {
		void live.events.length;
		engine.ingest(live.events, located);
	});

	// ── per-frame render into GeoJSON sources ─────────────────────────────
	function frame() {
		if (!map || !ready) {
			requestAnimationFrame(frame);
			return;
		}
		const out = engine.frame(performance.now());

		(map.getSource('pulse-lines') as maplibregl.GeoJSONSource)?.setData({
			type: 'FeatureCollection',
			features: out.lines.map((l) => ({
				type: 'Feature',
				geometry: { type: 'LineString', coordinates: l.coords },
				properties: { color: l.color, opacity: l.opacity }
			})) as Feature[]
		});
		(map.getSource('pulse-dots') as maplibregl.GeoJSONSource)?.setData({
			type: 'FeatureCollection',
			features: out.dots.map((d) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: d.at },
				properties: { color: d.color }
			})) as Feature[]
		});
		(map.getSource('pulse-rings') as maplibregl.GeoJSONSource)?.setData({
			type: 'FeatureCollection',
			features: out.rings.map((r) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: r.at },
				properties: { color: r.color, r: r.r, o: r.o }
			})) as Feature[]
		});

		// Chime once as the dot reaches each node (shared engine: $lib/live-audio).
		if (chime.on) for (const at of out.arrivals) chime.play(Math.round((at[0] + at[1]) * 100));

		animCount = out.count;
		requestAnimationFrame(frame);
	}

	// react to theme changes → swap the basemap style
	let basemapLight = false;
	let currentBasemap = basemap.id;
	$effect(() => {
		void theme.id;
		const light = isLight();
		if (!map || light === basemapLight) return;
		basemapLight = light;
		map.setStyle(basemapStyle(currentBasemap, light));
		// styledata fires mid-load with isStyleLoaded()===false, so ensureOverlays
		// bails; `idle` is guaranteed once the new style has fully settled.
		map.once('idle', ensureOverlays);
	});

	// Swap the basemap when the user picks a different one.
	$effect(() => {
		const id = basemap.id;
		if (!map || id === currentBasemap) return;
		currentBasemap = id;
		basemapLight = isLight();
		map.setStyle(basemapStyle(id, basemapLight));
		map.once('idle', ensureOverlays);
	});

	// Re-add overlays after a basemap style swap (theme or basemap change) drops them.
	function ensureOverlays() {
		if (!map || !map.isStyleLoaded()) return;
		if (basemapHasHillshade(currentBasemap)) ensureHillshade(map, basemapLight);
		if (basemapHasLocalTerrain(currentBasemap)) ensureLocalTerrain(map, basemapLight);
		if (basemapHasTint(currentBasemap)) ensureTint(map, basemapLight);
		if (map.getSource('nodes')) return;
		addLayers();
		(map.getSource('nodes') as maplibregl.GeoJSONSource | undefined)?.setData(nodeFeatures());
	}

	function nodeFeatures(): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: located
				.filter((n) => selectedRoles.has(n.role))
				.map((n) => ({
					type: 'Feature',
					geometry: { type: 'Point', coordinates: [n.longitude!, n.latitude!] },
					properties: {
						role: n.role,
						color: ROLE_HEX[n.role] ?? '#8394a1',
						name: n.name,
						pubkey: n.publicKey,
						// Marks a node beyond a sanctioned bridge so the dot can carry a ring.
						offSegment: !!n.viaBridge,
						fav: favorites.has(n.publicKey)
					}
				}))
		};
	}

	// re-filter node display when the role selection or favorites change
	$effect(() => {
		void selectedRoles;
		void favorites.keys;
		if (map && ready) (map.getSource('nodes') as maplibregl.GeoJSONSource)?.setData(nodeFeatures());
	});

	async function loadNodes() {
		nodes = await api.nodes();
		located = locatedNodes(nodes);
		if (map && ready) (map.getSource('nodes') as maplibregl.GeoJSONSource)?.setData(nodeFeatures());
	}

	function addLayers() {
		if (!map) return;
		map.addSource('nodes', { type: 'geojson', data: nodeFeatures() });
		map.addSource('pulse-lines', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
		map.addSource('pulse-dots', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
		map.addSource('pulse-rings', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });

		map.addLayer({
			id: 'pulse-lines',
			type: 'line',
			source: 'pulse-lines',
			layout: { 'line-cap': 'round', 'line-join': 'round' },
			paint: {
				'line-color': ['get', 'color'],
				'line-width': 2,
				'line-opacity': ['get', 'opacity'],
				'line-blur': 1.5
			}
		});
		// Amber ring around favorited nodes, beneath the node dots.
		map.addLayer({
			id: 'fav-halo',
			type: 'circle',
			source: 'nodes',
			filter: ['==', ['get', 'fav'], true],
			paint: {
				// Ring hugging the dot tightly — sits right against the node radius
				// (≈ node radius + 0.5px) so it reads as the dot's own outline.
				'circle-radius': [
					'interpolate', ['linear'], ['zoom'],
					6, ['match', ['get', 'role'], 'Repeater', 2.7, 2.1],
					11, ['match', ['get', 'role'], 'Repeater', 5, 3.3],
					15, ['match', ['get', 'role'], 'Repeater', 7.5, 5.3]
				],
				'circle-color': 'rgba(0,0,0,0)',
				'circle-stroke-color': FAV_COLOR,
				'circle-stroke-width': 1.75,
				'circle-stroke-opacity': 0.95
			}
		});
		map.addLayer({
			id: 'nodes',
			type: 'circle',
			source: 'nodes',
			paint: {
				// Smaller when zoomed out, scaling up as you zoom in.
				'circle-radius': [
					'interpolate', ['linear'], ['zoom'],
					6, ['match', ['get', 'role'], 'Repeater', 2.2, 1.6],
					11, ['match', ['get', 'role'], 'Repeater', 4.5, 2.8],
					15, ['match', ['get', 'role'], 'Repeater', 7, 4.8]
				],
				'circle-color': ['get', 'color'],
				'circle-opacity': 0.85,
				'circle-stroke-width': ['case', ['get', 'offSegment'], 2, 1],
				'circle-stroke-color': ['case', ['get', 'offSegment'], SEGMENT_COLOR, inkColor()]
			}
		});
		map.addLayer({
			id: 'pulse-rings',
			type: 'circle',
			source: 'pulse-rings',
			paint: {
				'circle-radius': ['get', 'r'],
				'circle-color': 'rgba(0,0,0,0)',
				'circle-stroke-color': ['get', 'color'],
				'circle-stroke-width': 1.5,
				'circle-stroke-opacity': ['get', 'o']
			}
		});
		map.addLayer({
			id: 'pulse-glow',
			type: 'circle',
			source: 'pulse-dots',
			paint: { 'circle-radius': 11, 'circle-color': ['get', 'color'], 'circle-blur': 1, 'circle-opacity': 0.5 }
		});
		map.addLayer({
			id: 'pulse-dots',
			type: 'circle',
			source: 'pulse-dots',
			paint: {
				'circle-radius': 4.5,
				'circle-color': '#ffffff',
				'circle-stroke-width': 2,
				'circle-stroke-color': ['get', 'color']
			}
		});

	}

	// Interaction handlers — bound once. MapLibre keeps layer-id listeners across
	// removeLayer/addLayer, so they survive a basemap style swap.
	function bindEvents() {
		if (!map) return;
		// Node click → full node-detail modal.
		map.on('click', 'nodes', (e) => {
			const p = e.features![0].properties as { pubkey?: string };
			if (p?.pubkey) nodeKey = p.pubkey;
		});
		map.on('mouseenter', 'nodes', () => map && (map.getCanvas().style.cursor = 'pointer'));
		map.on('mouseleave', 'nodes', () => map && (map.getCanvas().style.cursor = ''));
		// Name-only label while hovering a node dot.
		attachHoverLabel(map, 'nodes');
	}

	// Persist overlay collapse state once loaded (default minimized on first visit).
	$effect(() => {
		const mc = mapCtrlOpen;
		const rp = panelOpen;
		if (!overlaysReady) return;
		try {
			localStorage.setItem(MAPCTRL_KEY, mc ? '1' : '0');
			localStorage.setItem(PANEL_KEY, rp ? '1' : '0');
		} catch {
			/* storage unavailable */
		}
	});

	onMount(() => {
		try {
			mapCtrlOpen = localStorage.getItem(MAPCTRL_KEY) === '1';
			panelOpen = localStorage.getItem(PANEL_KEY) === '1';
		} catch {
			/* storage unavailable */
		}
		overlaysReady = true;
		// Restores sound on/off + tuning; if sound was left on it arms the
		// gesture unlock the browser requires before audio can start.
		chime.load();

		basemap.init();
		if (incoming.basemap) basemap.preview(incoming.basemap);
		if (incoming.roles) selectedRoles = new Set(incoming.roles);
		webglOk = hasWebGL();
		if (!webglOk) {
			// No WebGL → no live propagation (it needs MapLibre); show a static
			// node map via Leaflet, still refreshing the located-node set.
			loadNodes();
			const t = setInterval(loadNodes, 30000);
			return () => clearInterval(t);
		}
		currentBasemap = basemap.id;
		basemapLight = isLight();
		map = new maplibregl.Map({
			container: mapEl,
			style: basemapStyle(currentBasemap, basemapLight),
			center: [incoming.lon ?? -123.9, incoming.lat ?? 49.2],
			zoom: incoming.zoom ?? 8.4,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: true, visualizePitch: true }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			if (basemapHasHillshade(currentBasemap)) ensureHillshade(map!, basemapLight);
			if (basemapHasLocalTerrain(currentBasemap)) ensureLocalTerrain(map!, basemapLight);
			if (basemapHasTint(currentBasemap)) ensureTint(map!, basemapLight);
			collapseAttribution(map!);
			addLayers();
			bindEvents();
			ready = true;
			loadNodes();
		});
		// Re-add overlays after a basemap (theme) style swap drops them.
		map.on('styledata', ensureOverlays);
		// Mirror the camera into the URL on moveend (replaceState, so panning
		// never fills the back button).
		map.on('moveend', syncUrl);
		const t = setInterval(loadNodes, 30000);
		requestAnimationFrame(frame);
		return () => {
			clearInterval(t);
			map?.remove();
			map = null;
		};
	});
</script>

<Seo
	title="Live MeshCore Signal Map"
	description="Real-time signal map of the MeshCore LoRa mesh — watch packets and links light up across Vancouver Island and the Lower Mainland of British Columbia."
	path="/live-map"
/>

<PageHeader eyebrow="Real-time Propagation" title="Live Map">
	<div class="font-mono text-fg-dim flex items-center gap-4 text-xs">
		<span><span class="text-signal tnum">{located.length}</span> <span class="text-fg-faint">located</span></span>
		<span class="flex items-center gap-1.5">
			{#if live.connected}<span class="live-dot"></span>{/if}
			<span class="text-fg-faint">{animCount} active</span>
		</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	<div class="panel relative overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<NodeInspector pubkey={nodeKey} onclose={() => (nodeKey = null)} />
		{#if !webglOk}
			<FallbackMap
				nodes={located}
				roleFilter={selectedRoles}
				center={[-123.9, 49.2]}
				zoom={8}
				live
				audio
				bind:bannerOpen={fbBanner}
				onselect={(k) => (nodeKey = k)}
				notice="WebGL is disabled — showing the basic live map. Enable WebGL for terrain and the full-fidelity animation."
			/>
			<MapRoleFilter
				bind:selected={selectedRoles}
				title="Map Control"
				bind:open={mapCtrlOpen}
				pos={fbBanner ? 'top-16' : 'top-3'}
			>
				<ChimeControls />
			</MapRoleFilter>
			<BasemapSelector posClass={fbBanner ? 'top-16 right-3' : 'top-3 right-3'} />
		{:else}
		<div bind:this={mapEl} class="h-full w-full"></div>
		<BasemapSelector />
		<!-- Top-centre: the left (map control) and right (basemap) panels both
		     expand downward when opened, so stacking under either collides. -->
		<div data-map-center class="pointer-events-none absolute top-3 left-1/2 z-10 flex -translate-x-1/2 flex-col items-center gap-2">
			<div class="pointer-events-auto"><CopyMapLink url={linkUrl} /></div>
			<LivePill detail="{located.length} located nodes" onnode={(k) => (nodeKey = k)} />
		</div>

		<MapRoleFilter bind:selected={selectedRoles} title="Map Control" bind:open={mapCtrlOpen}>
			<ChimeControls />
		</MapRoleFilter>
		{/if}

		<!-- Recent packets overlay (bottom-left) — shared by the MapLibre and
		     WebGL-free live maps. -->
		<div
			class="border-line bg-ink-2/85 absolute bottom-3 left-3 z-10 w-[280px] overflow-hidden rounded-[var(--radius)] border shadow-lg backdrop-blur-md"
		>
			<button
				onclick={() => (panelOpen = !panelOpen)}
				class="border-line/70 hover:bg-panel-2/60 flex w-full items-center gap-2 px-3 py-2 text-left transition-colors {panelOpen
					? 'border-b'
					: ''}"
			>
				{#if live.connected}<span class="live-dot"></span>{/if}
				<span data-caps class="font-display text-fg text-xs font-700 tracking-wide uppercase">Recent packets</span>
				<span class="label ml-auto tnum">{recent.length}</span>
				<svg
					viewBox="0 0 24 24"
					class="text-fg-faint h-3.5 w-3.5 transition-transform {panelOpen ? '' : 'rotate-180'}"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg
				>
			</button>

			{#if panelOpen}
				<div class="max-h-[300px] overflow-y-auto">
					{#if recent.length === 0}
						<div class="text-fg-faint px-3 py-6 text-center text-xs">Waiting for packets…</div>
					{:else}
						<div class="divide-line/40 divide-y">
							{#each recent as g (g.key)}
								<button
									onclick={() => (selected = g)}
									class="panel-hover flex w-full items-center gap-2 px-3 py-1.5 text-left"
								>
									<Tooltip text="Matches this packet's comet on the map" class="shrink-0">
										<span
											class="h-4 w-1 rounded-full"
											style="background:{hashColor(g.messageHash)}"
										></span>
									</Tooltip>
									<PayloadTag type={g.payloadType} />
									<span class="min-w-0 flex-1 truncate text-xs">
										{#if g.node}
											<span class="text-fg">{g.node.name || shortKey(g.node.publicKey)}</span>
										{:else}
											<span class="font-mono text-fg-faint">{g.messageHash}</span>
										{/if}
									</span>
									{#if g.count > 1}
										<span class="font-mono text-signal text-[0.6rem] tnum">×{g.count}</span>
									{/if}
									<span class="font-mono w-9 text-right text-[0.62rem] tnum" style="color:{snrColor(g.bestSnr)}"
										>{fmtSnr(g.bestSnr)}</span
									>
									<span class="font-mono text-fg-faint w-6 text-right text-[0.62rem] tnum">{ago(g.latest)}</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</div>

<LiveGroupModal group={selected} onclose={() => (selected = null)} />
