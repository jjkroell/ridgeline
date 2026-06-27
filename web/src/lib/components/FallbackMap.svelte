<!--
  WebGL-free map shown when MapLibre can't run (WebGL disabled). Uses Leaflet — a
  DOM/canvas slippy map — with theme-aware CARTO raster tiles and clickable node
  markers. Two optional modes give parity with the WebGL maps without WebGL:
    • cluster — group nodes with Leaflet.markercluster (the static /map view).
    • live    — animate propagation pulses onto a 2-D canvas overlay driven by the
                shared PulseEngine (the /live-map view).
  Leaflet + the cluster plugin (and their CSS) are dynamically imported so they
  only load for the minority of visitors with WebGL disabled. A dismissible banner
  points them at enabling WebGL for the full MapLibre maps.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import type { Node } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import { isLight, inkColor, ROLE_HEX, FAV_COLOR, locatedNodes } from '$lib/map-util';
	import { favorites } from '$lib/favorites.svelte';
	import { roleLabel } from '$lib/format';
	import { live } from '$lib/live.svelte';
	import { PulseEngine } from '$lib/live-pulse';

	interface Props {
		nodes: Node[];
		/** Initial centre as [lon, lat] (matches the MapLibre maps' convention). */
		center?: [number, number];
		zoom?: number;
		onselect?: (pubkey: string) => void;
		/** Banner copy; defaults to the generic "enable WebGL" notice. */
		notice?: string;
		/** Group nodes into clusters (static map). */
		cluster?: boolean;
		/** Animate propagation pulses from the live feed (live map). */
		live?: boolean;
	}
	let {
		nodes,
		center = [-123.65, 49.25],
		zoom = 9,
		onselect,
		notice = 'Basic map — WebGL is disabled in your browser. Enable it for the full interactive map with terrain, clustering, coverage and live propagation.',
		cluster = false,
		live: liveMode = false
	}: Props = $props();

	let el: HTMLDivElement;
	// Leaflet is loaded lazily; keep the namespace + instances as `any` rather than
	// pulling the types into the eager bundle.
	/* eslint-disable @typescript-eslint/no-explicit-any */
	let L: any = null;
	let map: any = null;
	let tiles: any = null;
	let markers: any = null;
	let canvasRenderer: any = null;
	/* eslint-enable @typescript-eslint/no-explicit-any */
	let didFit = false;
	let curLight = false;
	let bannerOpen = $state(true);

	const located = $derived(locatedNodes(nodes));

	function tileUrl(light: boolean): string {
		// CARTO's key-less raster basemaps mirror the app's dark/light themes.
		return `https://{s}.basemaps.cartocdn.com/${light ? 'light_all' : 'dark_all'}/{z}/{x}/{y}{r}.png`;
	}

	// Teal cluster bubble matching the MapLibre cluster style.
	function clusterIcon(count: number) {
		const d = count >= 30 ? 48 : count >= 10 ? 36 : 26;
		return L.divIcon({
			html:
				`<div style="width:${d}px;height:${d}px;display:flex;align-items:center;justify-content:center;` +
				`border-radius:50%;background:rgba(21,158,139,.85);border:1.5px solid #34e3c4;` +
				`color:#04140f;font:600 12px/1 var(--font-mono,monospace)">${count}</div>`,
			className: 'rl-cluster',
			iconSize: [d, d]
		});
	}

	// A node rendered as a real marker with a divIcon. Leaflet.markercluster only
	// shows/hides leaf layers through their `_icon` DOM element, which circleMarkers
	// (canvas OR svg) don't have — so in cluster mode each node must be a divIcon
	// marker or it never appears once clustered.
	function nodeIcon(n: Node, stroke: string) {
		const repeater = n.role === 'Repeater';
		const d = repeater ? 13 : 10;
		const color = ROLE_HEX[n.role] ?? '#8394a1';
		const ring = favorites.has(n.publicKey) ? `box-shadow:0 0 0 2px ${FAV_COLOR};` : '';
		return L.divIcon({
			html:
				`<div style="width:${d}px;height:${d}px;border-radius:50%;` +
				`background:${color};border:1px solid ${stroke};${ring}box-sizing:border-box"></div>`,
			className: 'rl-node',
			iconSize: [d, d],
			iconAnchor: [d / 2, d / 2]
		});
	}

	// Canvas circleMarkers are sized in screen pixels, so a fixed radius looks the
	// same at every zoom — tiny dots eat the whole region when zoomed out. Scale the
	// radius with zoom (~22% per level, anchored at z9) so dots stay proportionate,
	// clamped so they never vanish or balloon.
	function nodeRadius(repeater: boolean, zoom: number): number {
		const base = repeater ? 4.2 : 3;
		const r = base * Math.pow(1.22, zoom - 9);
		return Math.max(1.5, Math.min(repeater ? 14 : 11, r));
	}

	function renderMarkers() {
		if (!map || !L) return;
		markers.clearLayers();
		const stroke = inkColor();
		const zoom = map.getZoom();
		for (const n of located) {
			const lat = n.latitude!;
			const lon = n.longitude!;
			// Cluster mode: divIcon markers so markercluster can cluster/decluster them.
			if (cluster) {
				const m = L.marker([lat, lon], { icon: nodeIcon(n, stroke) });
				m.bindTooltip(`${n.name || n.publicKey.slice(0, 10)} · ${roleLabel(n.role)}`, {
					direction: 'top'
				});
				m.on('click', () => onselect?.(n.publicKey));
				m.addTo(markers);
				continue;
			}
			// Non-cluster (live map): canvas circleMarkers — cheap to draw in bulk.
			const color = ROLE_HEX[n.role] ?? '#8394a1';
			const repeater = n.role === 'Repeater';
			const r = nodeRadius(repeater, zoom);
			if (favorites.has(n.publicKey)) {
				L.circleMarker([lat, lon], {
					renderer: canvasRenderer,
					radius: r + 2.5,
					color: FAV_COLOR,
					weight: 1.75,
					opacity: 0.95,
					fill: false,
					interactive: false
				}).addTo(markers);
			}
			const m = L.circleMarker([lat, lon], {
				renderer: canvasRenderer,
				radius: r,
				color: stroke,
				weight: 1,
				fillColor: color,
				fillOpacity: 0.9
			});
			m.bindTooltip(`${n.name || n.publicKey.slice(0, 10)} · ${roleLabel(n.role)}`, {
				direction: 'top'
			});
			m.on('click', () => onselect?.(n.publicKey));
			m.addTo(markers);
		}
	}

	function fit() {
		if (!map || !L || didFit || located.length === 0) return;
		const b = L.latLngBounds(located.map((n) => [n.latitude!, n.longitude!]));
		map.fitBounds(b, { padding: [40, 40], maxZoom: 12 });
		didFit = true;
	}

	// ── live propagation overlay (2-D canvas, no WebGL) ────────────────────
	let engine: PulseEngine | null = null;
	let pulseCanvas: HTMLCanvasElement | null = null;
	let raf = 0;
	const PAYLOAD_COLOR: Record<string, string> = {
		Advert: '#34e3c4',
		TextMessage: '#5b9dff',
		GroupText: '#a78bfa',
		Trace: '#e8b454',
		Ack: '#9aa7b0',
		Request: '#6ee7a8',
		Response: '#6ee7a8'
	};
	const payloadColor = (t: string) => PAYLOAD_COLOR[t] ?? '#34e3c4';

	function sizeCanvas() {
		if (!pulseCanvas || !map) return;
		const size = map.getSize();
		const dpr = window.devicePixelRatio || 1;
		pulseCanvas.width = size.x * dpr;
		pulseCanvas.height = size.y * dpr;
		pulseCanvas.style.width = size.x + 'px';
		pulseCanvas.style.height = size.y + 'px';
		const ctx = pulseCanvas.getContext('2d');
		if (ctx) ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
	}

	function drawPulses() {
		raf = requestAnimationFrame(drawPulses);
		if (!engine || !pulseCanvas || !map) return;
		const ctx = pulseCanvas.getContext('2d');
		if (!ctx) return;
		const size = map.getSize();
		ctx.clearRect(0, 0, size.x, size.y);
		const out = engine.frame(performance.now());
		// Project an engine [lon,lat] to a container pixel.
		const px = (c: [number, number]) => map.latLngToContainerPoint([c[1], c[0]]);

		// Comet trails (line-width 2, soft glow).
		ctx.lineCap = 'round';
		ctx.lineJoin = 'round';
		ctx.lineWidth = 2;
		for (const l of out.lines) {
			if (l.coords.length < 2) continue;
			ctx.globalAlpha = l.opacity;
			ctx.strokeStyle = l.color;
			ctx.shadowColor = l.color;
			ctx.shadowBlur = 4;
			ctx.beginPath();
			const p0 = px(l.coords[0]);
			ctx.moveTo(p0.x, p0.y);
			for (let i = 1; i < l.coords.length; i++) {
				const p = px(l.coords[i]);
				ctx.lineTo(p.x, p.y);
			}
			ctx.stroke();
		}

		// Node ripples (expanding stroked rings).
		ctx.shadowBlur = 0;
		ctx.lineWidth = 1.5;
		for (const r of out.rings) {
			const p = px(r.at);
			ctx.globalAlpha = r.o;
			ctx.strokeStyle = r.color;
			ctx.beginPath();
			ctx.arc(p.x, p.y, r.r, 0, Math.PI * 2);
			ctx.stroke();
		}

		// Travelling head: a glow halo under a white dot.
		for (const d of out.dots) {
			const p = px(d.at);
			ctx.globalAlpha = 0.5;
			ctx.fillStyle = d.color;
			ctx.shadowColor = d.color;
			ctx.shadowBlur = 9;
			ctx.beginPath();
			ctx.arc(p.x, p.y, 7, 0, Math.PI * 2);
			ctx.fill();
			ctx.shadowBlur = 0;
			ctx.globalAlpha = 1;
			ctx.fillStyle = '#ffffff';
			ctx.strokeStyle = d.color;
			ctx.lineWidth = 2;
			ctx.beginPath();
			ctx.arc(p.x, p.y, 4.5, 0, Math.PI * 2);
			ctx.fill();
			ctx.stroke();
		}
		ctx.globalAlpha = 1;
	}

	onMount(() => {
		let destroyed = false;
		(async () => {
			// CSS first so controls/tiles are styled the moment the map appears.
			await import('leaflet/dist/leaflet.css');
			L = (await import('leaflet')).default ?? (await import('leaflet'));
			if (cluster) {
				await import('leaflet.markercluster/dist/MarkerCluster.css');
				await import('leaflet.markercluster');
			}
			if (destroyed || !el) return;
			curLight = isLight();
			canvasRenderer = L.canvas({ padding: 0.5 });
			map = L.map(el, {
				center: [center[1], center[0]],
				zoom,
				zoomControl: false,
				preferCanvas: true,
				attributionControl: true
			});
			L.control.zoom({ position: 'bottomright' }).addTo(map);
			tiles = L.tileLayer(tileUrl(curLight), {
				subdomains: 'abcd',
				maxZoom: 19,
				attribution:
					'© <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> © <a href="https://carto.com/attributions" target="_blank" rel="noopener">CARTO</a>'
			}).addTo(map);
			markers =
				cluster && L.markerClusterGroup
					? L.markerClusterGroup({
							maxClusterRadius: 46,
							showCoverageOnHover: false,
							disableClusteringAtZoom: 12,
							iconCreateFunction: (c: { getChildCount(): number }) => clusterIcon(c.getChildCount())
						})
					: L.layerGroup();
			markers.addTo(map);
			renderMarkers();
			fit();

			// Re-plot dots at the new zoom-scaled radius. Cluster mode keeps fixed-size
			// divIcons (markercluster redraws those itself), so only the live/non-cluster
			// circleMarkers need this.
			if (!cluster) map.on('zoomend', renderMarkers);

			if (liveMode) {
				engine = new PulseEngine(payloadColor);
				pulseCanvas = document.createElement('canvas');
				pulseCanvas.style.cssText =
					'position:absolute;inset:0;pointer-events:none;z-index:400';
				map.getContainer().appendChild(pulseCanvas);
				sizeCanvas();
				map.on('resize', sizeCanvas);
				raf = requestAnimationFrame(drawPulses);
			}
		})();
		return () => {
			destroyed = true;
			cancelAnimationFrame(raf);
			map?.remove();
			map = null;
			pulseCanvas = null;
			engine = null;
		};
	});

	// Re-plot markers as nodes load / favourites change.
	$effect(() => {
		void located;
		void favorites.keys;
		if (map) {
			renderMarkers();
			fit();
		}
	});

	// Feed the pulse engine from the live event stream.
	$effect(() => {
		if (!liveMode) return;
		void live.events.length;
		engine?.ingest(live.events, located);
	});

	// Swap tile theme when the UI theme toggles.
	$effect(() => {
		void theme.mode;
		const light = isLight();
		if (!map || !tiles || light === curLight) return;
		curLight = light;
		tiles.setUrl(tileUrl(light));
		renderMarkers(); // marker stroke follows the theme ink colour
	});
</script>

<div class="relative h-full w-full">
	<div bind:this={el} class="h-full w-full"></div>

	{#if bannerOpen}
		<div
			class="border-line bg-ink-2/90 absolute top-3 right-3 left-3 z-[1000] flex items-start gap-2 rounded-[var(--radius)] border px-3 py-2 shadow-lg backdrop-blur-md"
		>
			<svg
				viewBox="0 0 24 24"
				class="text-signal mt-0.5 h-4 w-4 shrink-0"
				fill="none"
				stroke="currentColor"
				stroke-width="1.7"
				stroke-linecap="round"
				stroke-linejoin="round"
			>
				<circle cx="12" cy="12" r="10" /><path d="M12 16v-4M12 8h.01" />
			</svg>
			<p class="text-fg-dim flex-1 text-xs leading-snug">{notice}</p>
			<button
				onclick={() => (bannerOpen = false)}
				aria-label="Dismiss"
				class="text-fg-faint hover:text-fg -mt-0.5 shrink-0 text-lg leading-none">×</button
			>
		</div>
	{/if}
</div>

<style>
	/* Leaflet renders its own controls/tiles; keep its attribution unobtrusive and
	   on-theme rather than the default white box. */
	:global(.leaflet-container) {
		background: var(--color-ink-2);
		font-family: inherit;
	}
	:global(.leaflet-control-attribution) {
		background: color-mix(in srgb, var(--color-ink-2) 80%, transparent);
		color: var(--color-fg-faint);
		font-size: 9px;
	}
	:global(.leaflet-control-attribution a) {
		color: var(--color-fg-dim);
	}
	:global(.leaflet-bar a) {
		background: var(--color-panel);
		color: var(--color-fg-dim);
		border-color: var(--color-line);
	}
	:global(.leaflet-bar a:hover) {
		background: var(--color-panel-2);
		color: var(--color-fg);
	}
	:global(.leaflet-tooltip) {
		background: var(--color-panel);
		border: 1px solid var(--color-line-bright);
		color: var(--color-fg);
		box-shadow: 0 4px 14px rgba(0, 0, 0, 0.4);
	}
	:global(.leaflet-tooltip-top::before) {
		border-top-color: var(--color-line-bright);
	}
	:global(.rl-cluster) {
		background: transparent;
		border: 0;
	}
	:global(.rl-node) {
		background: transparent;
		border: 0;
	}
</style>
