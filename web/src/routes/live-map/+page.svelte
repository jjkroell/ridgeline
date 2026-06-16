<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { Feature, FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { live, groupLive, type LiveGroup } from '$lib/live.svelte';
	import { theme } from '$lib/theme.svelte';
	import { ago, shortKey, fmtSnr, snrColor } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';
	import LiveGroupModal from '$lib/components/LiveGroupModal.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let ready = false;
	let nodes: Node[] = [];
	let located: Node[] = $state([]);
	let animCount = $state(0);

	// Recent packets overlay (grouped, max 15) with minimize/maximize.
	let panelOpen = $state(true);
	let selected = $state<LiveGroup | null>(null);
	const recent = $derived(groupLive(live.events).slice(0, 15));

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
	const FADE = 700;

	// Fire-on-arrival: pulse each observer's reported header path as it comes
	// in, deduped by messageHash + path. Observers report the same transmission
	// over several seconds (mean ~5s here), so firing on arrival animates the
	// flood spreading in real time rather than dumping every branch at once.
	const fired = new Map<string, number>(); // "hash:path" -> time

	// A small expanding ring fired at each node as the pulse passes through it.
	interface Ripple {
		at: [number, number];
		born: number; // when the dot reaches this node (may be in the future)
		color: string;
	}
	let ripples: Ripple[] = [];
	const RIPPLE_DUR = 650;

	function addAnim(pts: [number, number][], color: string, uncertain: boolean) {
		const seglen = [0];
		for (let i = 1; i < pts.length; i++) {
			const dx = pts[i][0] - pts[i - 1][0],
				dy = pts[i][1] - pts[i - 1][1];
			seglen.push(seglen[i - 1] + Math.hypot(dx, dy));
		}
		const total = seglen[seglen.length - 1];
		const born = performance.now();
		const dur = 900 + pts.length * 280;
		anims.push({ pts, seglen, total, color, born, dur, uncertain });
		if (anims.length > 60) anims.shift();

		// Schedule a ripple at each node, timed to when the dot arrives there.
		for (let i = 0; i < pts.length; i++) {
			ripples.push({ at: pts[i], born: born + (total > 0 ? seglen[i] / total : 0) * dur, color });
		}
		if (ripples.length > 500) ripples.splice(0, ripples.length - 500);
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

		// Node ripples: an expanding ring as the pulse reaches each node.
		const rings: Feature[] = [];
		ripples = ripples.filter((r) => now - r.born < RIPPLE_DUR);
		for (const r of ripples) {
			const age = now - r.born;
			if (age < 0) continue; // scheduled but not yet reached
			const t = age / RIPPLE_DUR;
			rings.push({
				type: 'Feature',
				geometry: { type: 'Point', coordinates: r.at },
				properties: { color: r.color, r: 3 + t * 11, o: 0.85 * (1 - t) }
			});
		}
		(map.getSource('pulse-rings') as maplibregl.GeoJSONSource)?.setData({
			type: 'FeatureCollection',
			features: rings
		});

		animCount = anims.length;
		requestAnimationFrame(frame);
	}

	// react to new live events → pulse each newly-seen header path on arrival
	$effect(() => {
		void live.events.length;
		const now = performance.now();
		for (const ev of live.events.slice(0, 60)) {
			if (!ev.path || ev.path.length < 1) continue;
			const key = ev.messageHash + ':' + ev.path.join(',');
			if (fired.has(key)) continue;
			fired.set(key, now);
			const { pts, uncertain } = resolvePath(ev.path);
			if (pts.length >= 2) addAnim(pts, payloadColor(ev.payloadType), uncertain);
		}
		// prune dedupe map
		const cutoff = now - 90000;
		for (const [k, t] of fired) if (t < cutoff) fired.delete(k);
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
	<div class="panel relative overflow-hidden" style="height:calc(100vh - 220px);min-height:420px">
		<div bind:this={mapEl} class="h-full w-full"></div>

		<!-- Recent packets overlay (bottom-left) -->
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
				<span class="font-display text-fg text-xs font-700 tracking-wide">RECENT PACKETS</span>
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
	<p class="label mt-3">
		Pulses trace each packet along the repeaters that relayed it · color = payload type · gaps
		are hops whose repeater hasn't advertised a location yet
	</p>
</div>

<LiveGroupModal group={selected} onclose={() => (selected = null)} />

<style>
	:global(.maplibregl-ctrl-group) {
		background: var(--color-panel);
		border: 1px solid var(--color-line);
	}
	:global(.maplibregl-ctrl-group button + button) {
		border-top: 1px solid var(--color-line);
	}
</style>
