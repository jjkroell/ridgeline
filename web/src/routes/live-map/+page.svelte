<script lang="ts">
	import { onMount } from 'svelte';
	import maplibregl from 'maplibre-gl';
	import 'maplibre-gl/dist/maplibre-gl.css';
	import type { Feature, FeatureCollection } from 'geojson';
	import { api, type Node } from '$lib/api';
	import { live, groupLive, type LiveGroup } from '$lib/live.svelte';
	import { theme } from '$lib/theme.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { basemapStyle, basemapHasHillshade, collapseAttribution } from '$lib/map-basemap';
	import { basemap } from '$lib/basemap.svelte';
	import { ensureHillshade } from '$lib/map-hillshade';
	import { isLight, inkColor, ROLE_HEX, FAV_COLOR, locatedNodes } from '$lib/map-util';
	import { ago, shortKey, fmtSnr, snrColor } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import LiveGroupModal from '$lib/components/LiveGroupModal.svelte';
	import MapRoleFilter from '$lib/components/MapRoleFilter.svelte';
	import BasemapSelector from '$lib/components/BasemapSelector.svelte';
	import NodeModal from '$lib/components/NodeModal.svelte';

	let mapEl: HTMLDivElement;
	let map: maplibregl.Map | null = null;
	let ready = false;
	let nodes: Node[] = [];
	let located: Node[] = $state([]); // all located nodes (pulse resolution uses these)
	let animCount = $state(0);
	let selectedRoles = $state(new Set(['Repeater', 'RoomServer', 'ChatNode', 'Sensor']));

	// ── Low-key audio: a soft chime as each pulse reaches a node ───────────
	let soundOn = $state(false);
	const SOUND_KEY = 'ridgeline-livemap-sound';
	// Overlay collapse state — both default minimized, persisted across visits.
	const MAPCTRL_KEY = 'ridgeline-livemap-mapcontrol';
	const PANEL_KEY = 'ridgeline-livemap-recent';
	let mapCtrlOpen = $state(false);
	let overlaysReady = $state(false); // gate persistence until localStorage is loaded
	let actx: AudioContext | null = null;
	let masterGain: GainNode | null = null;
	let lastTickAt = 0;
	// Pentatonic (C5 D5 E5 G5 A5) — overlapping hits stay pleasant, never jarring.
	const SCALE = [523.25, 587.33, 659.25, 783.99, 880.0];

	function ensureAudio() {
		if (!actx) {
			const AC = window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
			if (!AC) return;
			actx = new AC();
			masterGain = actx.createGain();
			masterGain.gain.value = 0.5; // keep the whole thing quiet
			masterGain.connect(actx.destination);
		}
		if (actx.state === 'suspended') actx.resume();
	}

	function toggleSound() {
		soundOn = !soundOn;
		if (soundOn) ensureAudio(); // created within the click gesture so the browser allows it
		try {
			localStorage.setItem(SOUND_KEY, soundOn ? '1' : '0');
		} catch {
			/* storage unavailable */
		}
	}

	// A short, soft sine blip. seed picks a scale note so different nodes differ.
	function playTick(seed: number) {
		if (!actx || !masterGain) return;
		const wall = performance.now();
		if (wall - lastTickAt < 55) return; // throttle bursts into a gentle trickle
		lastTickAt = wall;
		const t = actx.currentTime;
		const osc = actx.createOscillator();
		const g = actx.createGain();
		osc.type = 'sine';
		osc.frequency.value = SCALE[Math.abs(seed) % SCALE.length];
		g.gain.setValueAtTime(0, t);
		g.gain.linearRampToValueAtTime(0.09, t + 0.008);
		g.gain.exponentialRampToValueAtTime(0.0001, t + 0.22);
		osc.connect(g).connect(masterGain);
		osc.start(t);
		osc.stop(t + 0.24);
	}

	// Recent packets overlay (grouped, max 15) with minimize/maximize.
	let panelOpen = $state(false);
	let selected = $state<LiveGroup | null>(null);
	let nodeKey = $state<string | null>(null);
	const recent = $derived(groupLive(live.events).slice(0, 15));

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
		played?: boolean; // chimed once on arrival
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

	// The visible "comet" line: from the head position back to ~two nodes
	// behind it. The lead reveals the line as it travels; the tail trails off
	// once the head is more than two nodes ahead.
	const TRAIL_NODES = 2;
	function trailCoords(a: Anim, travel: number): [number, number][] {
		const d = travel * a.total;
		let lastV = 0; // last vertex the head has passed
		for (let i = 1; i < a.pts.length; i++) {
			if (a.seglen[i] <= d) lastV = i;
			else break;
		}
		const tailV = Math.max(0, lastV - (TRAIL_NODES - 1));
		const coords = a.pts.slice(tailV, lastV + 1);
		if (d > a.seglen[lastV]) coords.push(along(a, travel)); // head mid-segment
		return coords;
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
			const trail = trailCoords(a, travel);
			if (trail.length >= 2) {
				lines.push({
					type: 'Feature',
					geometry: { type: 'LineString', coordinates: trail },
					properties: { color: a.color, opacity }
				});
			}
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
			if (!r.played) {
				r.played = true; // the dot just reached this node
				if (soundOn) playTick(Math.round((r.at[0] + r.at[1]) * 100));
			}
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
			// Only pulse genuinely fresh arrivals — skip the last-hour history the
			// feed seeds into the shared buffer, so the map doesn't burst on load.
			if (Date.now() - +new Date(ev.receivedAt) > 20000) continue;
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

	// react to theme changes → swap the basemap style
	let basemapLight = false;
	let currentBasemap = basemap.id;
	$effect(() => {
		void theme.mode;
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
				'circle-stroke-width': 1,
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
			soundOn = localStorage.getItem(SOUND_KEY) === '1';
			mapCtrlOpen = localStorage.getItem(MAPCTRL_KEY) === '1';
			panelOpen = localStorage.getItem(PANEL_KEY) === '1';
		} catch {
			/* storage unavailable */
		}
		overlaysReady = true;
		// If sound was left on, the AudioContext can only start after a user
		// gesture — arm it on the first interaction.
		if (soundOn) {
			const unlock = () => {
				ensureAudio();
				window.removeEventListener('pointerdown', unlock);
			};
			window.addEventListener('pointerdown', unlock);
		}

		basemap.init();
		currentBasemap = basemap.id;
		basemapLight = isLight();
		map = new maplibregl.Map({
			container: mapEl,
			style: basemapStyle(currentBasemap, basemapLight),
			center: [-123.9, 49.2],
			zoom: 8.4,
			attributionControl: { compact: true }
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');
		map.on('load', () => {
			map?.resize();
			if (basemapHasHillshade(currentBasemap)) ensureHillshade(map!, basemapLight);
			collapseAttribution(map!);
			addLayers();
			bindEvents();
			ready = true;
			loadNodes();
		});
		// Re-add overlays after a basemap (theme) style swap drops them.
		map.on('styledata', ensureOverlays);
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
		<BasemapSelector />

		<MapRoleFilter bind:selected={selectedRoles} title="Map Control" bind:open={mapCtrlOpen}>
			<div class="label mb-1.5">Audio</div>
			<Tooltip text={soundOn ? 'Mute node chimes' : 'Play a soft chime as pulses reach nodes'} class="block w-full">
				<button
					onclick={toggleSound}
					aria-pressed={soundOn}
					class="flex w-full items-center justify-center gap-1.5 rounded-[var(--radius)] border px-2 py-1 text-[0.68rem] font-medium transition-colors {soundOn
						? 'border-signal/50 text-signal'
						: 'border-line text-fg-dim hover:text-fg'}"
				>
					{#if soundOn}
						<svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M11 5 6 9H2v6h4l5 4z" /><path d="M15.5 8.5a5 5 0 0 1 0 7M19 5a9 9 0 0 1 0 14" /></svg>
						<span>Sound on</span>
					{:else}
						<svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M11 5 6 9H2v6h4l5 4z" /><path d="M22 9l-6 6M16 9l6 6" /></svg>
						<span>Muted</span>
					{/if}
				</button>
			</Tooltip>
		</MapRoleFilter>

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
</div>

<LiveGroupModal group={selected} onclose={() => (selected = null)} />
<NodeModal pubkey={nodeKey} onclose={() => (nodeKey = null)} />
