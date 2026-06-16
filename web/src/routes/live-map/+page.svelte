<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { Feature, FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { live } from '$lib/live.svelte';
	import { theme } from '$lib/theme.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let ready = false;
	let nodes: Node[] = [];
	let located: Node[] = $state([]);
	let animCount = $state(0);

	const isLight = () => document.documentElement.classList.contains('theme-light');
	const inkColor = () =>
		getComputedStyle(document.documentElement).getPropertyValue('--color-ink').trim() || '#070a0e';
	const basemap = (light: boolean) =>
		(['a', 'b', 'c'] as const).map(
			(s) => `https://${s}.basemaps.cartocdn.com/${light ? 'light_all' : 'dark_all'}/{z}/{x}/{y}.png`
		);

	const ROLE_COLOR: Record<string, string> = {
		Repeater: '#ff6b6b',
		ChatNode: '#5b9dff',
		RoomServer: '#6ee7a8',
		Sensor: '#e8b454',
		Observer: '#a78bfa'
	};
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

	// ── hop resolution ────────────────────────────────────────────────────
	// A packet's hops are all at the originating node's hash size, so the hop
	// length is the prefix length to match. Each hop may match several located
	// nodes (a short/1-byte hop is ambiguous); resolve by:
	//   1. preferring repeaters (relays are repeaters),
	//   2. anchoring on hops with a single candidate,
	//   3. for the rest, picking the candidate nearest a resolved neighbour.
	// Returns the located points in path order plus whether any hop was
	// ambiguous (so the path can be drawn with less confidence).
	const dist2 = (a: [number, number], b: [number, number]) =>
		(a[0] - b[0]) ** 2 + (a[1] - b[1]) ** 2;

	function resolvePath(path: string[]): { pts: [number, number][]; uncertain: boolean } {
		const cands: [number, number][][] = path.map((hop) => {
			let c = located.filter((n) => n.publicKey.startsWith(hop));
			const reps = c.filter((n) => n.role === 'Repeater');
			if (reps.length) c = reps;
			return c.map((n) => [n.longitude!, n.latitude!] as [number, number]);
		});

		const resolved: ([number, number] | null)[] = path.map(() => null);
		let uncertain = false;

		// anchor unambiguous hops
		for (let i = 0; i < cands.length; i++) if (cands[i].length === 1) resolved[i] = cands[i][0];

		// disambiguate the rest by nearest resolved neighbour
		for (let i = 0; i < cands.length; i++) {
			if (resolved[i] || cands[i].length === 0) continue;
			uncertain = true;
			let ref: [number, number] | null = null;
			for (let d = 1; d < cands.length && !ref; d++) {
				if (i - d >= 0 && resolved[i - d]) ref = resolved[i - d];
				else if (i + d < cands.length && resolved[i + d]) ref = resolved[i + d];
			}
			resolved[i] = ref
				? cands[i].reduce((best, p) => (dist2(p, ref!) < dist2(best, ref!) ? p : best))
				: cands[i][0];
		}

		return { pts: resolved.filter((p): p is [number, number] => p !== null), uncertain };
	}

	// ── animation state ───────────────────────────────────────────────────
	interface Anim {
		pts: [number, number][];
		seglen: number[]; // cumulative length at each vertex
		total: number;
		color: string;
		born: number;
		dur: number;
		uncertain: boolean;
	}
	let anims: Anim[] = [];
	const animated = new Map<string, number>(); // messageHash -> time, for dedupe
	const FADE = 700;

	function addAnim(pts: [number, number][], color: string, uncertain: boolean) {
		const seglen = [0];
		for (let i = 1; i < pts.length; i++) {
			const dx = pts[i][0] - pts[i - 1][0],
				dy = pts[i][1] - pts[i - 1][1];
			seglen.push(seglen[i - 1] + Math.hypot(dx, dy));
		}
		const total = seglen[seglen.length - 1];
		anims.push({
			pts,
			seglen,
			total,
			color,
			born: performance.now(),
			dur: 900 + pts.length * 280,
			uncertain
		});
		if (anims.length > 60) anims.shift();
	}

	// position along the polyline at fraction t (0..1)
	function along(a: Anim, t: number): [number, number] {
		const d = t * a.total;
		for (let i = 1; i < a.pts.length; i++) {
			if (d <= a.seglen[i]) {
				const segStart = a.seglen[i - 1];
				const f = (d - segStart) / (a.seglen[i] - segStart || 1);
				return [
					a.pts[i - 1][0] + (a.pts[i][0] - a.pts[i - 1][0]) * f,
					a.pts[i - 1][1] + (a.pts[i][1] - a.pts[i - 1][1]) * f
				];
			}
		}
		return a.pts[a.pts.length - 1];
	}

	// ── per-frame render into GeoJSON sources ─────────────────────────────
	function frame() {
		if (!map || !ready) {
			requestAnimationFrame(frame);
			return;
		}
		const now = performance.now();
		const lines: Feature[] = [];
		const dots: Feature[] = [];
		anims = anims.filter((a) => now - a.born < a.dur + FADE);
		for (const a of anims) {
			const age = now - a.born;
			const travel = Math.min(1, age / a.dur);
			// Ambiguous (disambiguated) paths are drawn fainter.
			const base = a.uncertain ? 0.45 : 0.85;
			const opacity = age < a.dur ? base : base * (1 - (age - a.dur) / FADE);
			lines.push({
				type: 'Feature',
				geometry: { type: 'LineString', coordinates: a.pts },
				properties: { color: a.color, opacity }
			});
			if (age < a.dur) {
				dots.push({
					type: 'Feature',
					geometry: { type: 'Point', coordinates: along(a, travel) },
					properties: { color: a.color }
				});
			}
		}
		(map.getSource('pulse-lines') as maplibregl.GeoJSONSource)?.setData({
			type: 'FeatureCollection',
			features: lines
		});
		(map.getSource('pulse-dots') as maplibregl.GeoJSONSource)?.setData({
			type: 'FeatureCollection',
			features: dots
		});
		animCount = anims.length;
		requestAnimationFrame(frame);
	}

	// react to new live events → enqueue animations
	$effect(() => {
		void live.events.length;
		for (const ev of live.events.slice(0, 40)) {
			if (!ev.path || ev.path.length < 1 || animated.has(ev.messageHash)) continue;
			const { pts, uncertain } = resolvePath(ev.path);
			if (pts.length < 2) continue;
			animated.set(ev.messageHash, performance.now());
			addAnim(pts, payloadColor(ev.payloadType), uncertain);
		}
		// prune dedupe map
		const cutoff = performance.now() - 60000;
		for (const [k, t] of animated) if (t < cutoff) animated.delete(k);
	});

	// react to theme changes
	$effect(() => {
		void theme.mode;
		if (!map || !map.isStyleLoaded()) return;
		(map.getSource('carto') as maplibregl.RasterTileSource | undefined)?.setTiles(basemap(isLight()));
		map.setPaintProperty('bg', 'background-color', inkColor());
	});

	function nodeFeatures(): FeatureCollection {
		return {
			type: 'FeatureCollection',
			features: located.map((n) => ({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: [n.longitude!, n.latitude!] },
				properties: { role: n.role, color: ROLE_COLOR[n.role] ?? '#8394a1', name: n.name }
			}))
		};
	}

	async function loadNodes() {
		nodes = await api.nodes();
		located = nodes.filter((n) => n.hasLocation && n.latitude != null && n.longitude != null);
		if (map && ready) (map.getSource('nodes') as maplibregl.GeoJSONSource)?.setData(nodeFeatures());
	}

	function addLayers() {
		if (!map) return;
		map.addSource('nodes', { type: 'geojson', data: nodeFeatures() });
		map.addSource('pulse-lines', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
		map.addSource('pulse-dots', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });

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
		map.addLayer({
			id: 'nodes',
			type: 'circle',
			source: 'nodes',
			paint: {
				'circle-radius': ['match', ['get', 'role'], 'Repeater', 5, 3],
				'circle-color': ['get', 'color'],
				'circle-opacity': 0.85,
				'circle-stroke-width': 1.5,
				'circle-stroke-color': inkColor()
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

	onMount(() => {
		map = new maplibregl.Map({
			container: mapEl,
			style: {
				version: 8,
				sources: {
					carto: { type: 'raster', tiles: basemap(isLight()), tileSize: 256, attribution: '© OpenStreetMap © CARTO' }
				},
				layers: [
					{ id: 'bg', type: 'background', paint: { 'background-color': inkColor() } },
					{ id: 'carto', type: 'raster', source: 'carto', paint: { 'raster-opacity': 0.65 } }
				]
			},
			center: [-123.9, 49.2],
			zoom: 8.4,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			addLayers();
			ready = true;
			loadNodes();
		});
		const t = setInterval(loadNodes, 30000);
		requestAnimationFrame(frame);
		return () => {
			clearInterval(t);
			map?.remove();
			map = null;
		};
	});
</script>

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
	<div class="panel overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<div bind:this={mapEl} class="h-full w-full"></div>
	</div>
	<p class="label mt-3">
		Pulses trace each packet along the repeaters that relayed it · color = payload type · gaps
		are hops whose repeater hasn't advertised a location yet
	</p>
</div>

<style>
	:global(.maplibregl-ctrl-group) {
		background: var(--color-panel);
		border: 1px solid var(--color-line);
	}
	:global(.maplibregl-ctrl-group button + button) {
		border-top: 1px solid var(--color-line);
	}
</style>
