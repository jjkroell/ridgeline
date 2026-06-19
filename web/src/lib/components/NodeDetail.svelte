<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import maplibregl from 'maplibre-gl';
	// Needed for marker/control styling when this component is reached directly
	// (e.g. /nodes/[pubkey]) without having visited a full map route first.
	import 'maplibre-gl/dist/maplibre-gl.css';
	import QRCode from 'qrcode';
	import { api, type Node, type NodeAnalytics, type NodeHistoryEntry } from '$lib/api';
	import { basemapStyleUrl, collapseAttribution } from '$lib/map-basemap';
	import { ago, shortKey, fmtCoord, fmtSnr, snrColor, roleColor, roleLabel, nodeStatus } from '$lib/format';
	import PayloadTag from './PayloadTag.svelte';
	import RoleBadge from './RoleBadge.svelte';
	import FavoriteStar from './FavoriteStar.svelte';
	import Tooltip from './Tooltip.svelte';

	interface Props {
		pubkey: string;
		/** Full node list, when the parent already has it (for collision counts). */
		nodes?: Node[];
		/** Render a name + role heading (used in the modal; the page has its own). */
		heading?: boolean;
	}
	let { pubkey, nodes: nodesProp = undefined, heading = false }: Props = $props();

	let node = $state<Node | null>(null);
	let detail = $state<NodeAnalytics | null>(null);
	let nodesList = $state<Node[]>(nodesProp ?? []);
	let loaded = $state(false);

	// On-demand stored history (own adverts + relayed packets) over a chosen range.
	const ranges = [
		{ label: '6h', sec: 21600 },
		{ label: '24h', sec: 86400 },
		{ label: '3d', sec: 259200 }
	];
	let history = $state<NodeHistoryEntry[]>([]);
	let histRange = $state(86400);
	let histLoading = $state(false);

	async function loadHistory() {
		histLoading = true;
		try {
			history = await api.nodeHistory(pubkey, histRange, 300);
		} catch {
			history = [];
		} finally {
			histLoading = false;
		}
	}
	function setRange(sec: number) {
		if (sec === histRange) return;
		histRange = sec;
		loadHistory();
	}

	async function refresh() {
		try {
			const [resp, list] = await Promise.all([
				api.nodeDetail(pubkey),
				nodesProp ? Promise.resolve(nodesProp) : api.nodes()
			]);
			node = resp.node;
			detail = resp.detail;
			nodesList = list;
		} finally {
			loaded = true;
		}
	}
	onMount(() => {
		refresh();
		loadHistory();
		const t = setInterval(refresh, 15000);
		return () => clearInterval(t);
	});

	const hasLoc = $derived(!!node && node.latitude != null && node.longitude != null);

	// Hash ID + collision count (within the known node set).
	const hashId = $derived.by(() => {
		const hs = node?.hashSize ?? 0;
		if (!hs) return null;
		const hex = pubkey.slice(0, hs * 2);
		const shared = nodesList.filter((n) => n.publicKey !== pubkey && n.publicKey.startsWith(hex)).length;
		return { bytes: hs, hex, shared };
	});

	// Liveness from the most recent of advert or relay activity (see nodeStatus).
	const status = $derived(
		nodeStatus({ lastSeen: node?.lastSeen, lastRelayed: detail?.relay.lastRelayed })
	);
	const isRelay = $derived(node?.role === 'Repeater' || node?.role === 'RoomServer');

	// Score → label/colour, mirroring CoreScope's classification bands.
	function trafficLabel(s: number) {
		if (s >= 0.8) return { label: 'Critical', color: 'var(--color-signal)' };
		if (s >= 0.6) return { label: 'Valuable', color: 'var(--color-signal)' };
		if (s >= 0.3) return { label: 'Moderate', color: 'var(--color-amber)' };
		if (s >= 0.1) return { label: 'Marginal', color: 'var(--color-amber)' };
		return { label: 'Redundant', color: 'var(--color-fg-faint)' };
	}
	function bridgeLabel(s: number) {
		if (s >= 0.5) return { label: 'Critical bridge', color: 'var(--color-signal)' };
		if (s >= 0.2) return { label: 'Important', color: 'var(--color-signal)' };
		if (s >= 0.05) return { label: 'Some role', color: 'var(--color-amber)' };
		if (s > 0) return { label: 'Marginal', color: 'var(--color-amber)' };
		return { label: 'No bridge role', color: 'var(--color-fg-faint)' };
	}
	const fmtAbs = (iso?: string) => (iso ? new Date(iso).toLocaleString() : '—');

	// Advert cadence (heartbeat) → friendly "every ~N" string.
	function cadence(sec?: number): string {
		if (sec == null) return '—';
		if (sec < 90) return `every ~${Math.round(sec)}s`;
		if (sec < 5400) return `every ~${Math.round(sec / 60)}m`;
		return `every ~${(sec / 3600).toFixed(1)}h`;
	}
	const activityMax = $derived(Math.max(1, ...(detail?.activity ?? [1])));
	const tl = $derived(trafficLabel(detail?.trafficShare ?? 0));
	const bl = $derived(bridgeLabel(detail?.bridge ?? 0));

	let copied = $state(false);
	async function copyKey() {
		await navigator.clipboard.writeText(pubkey);
		copied = true;
		setTimeout(() => (copied = false), 1200);
	}

	// --- Inset map of the node's location ---
	let mapEl = $state<HTMLDivElement>();
	let map: maplibregl.Map | null = null;
	let marker: maplibregl.Marker | null = null;
	// Create the map ONCE. Gate on the memoized `hasLoc` boolean (not `node`,
	// which refresh() reassigns to a fresh object every 15s — tracking it here
	// would tear down and recreate the whole map on every poll, flickering the
	// attribution back open and never letting the marker settle). Coords are
	// read untracked so a node refresh can't retrigger this effect.
	$effect(() => {
		if (!mapEl || !hasLoc || map) return;
		untrack(() => {
			const lng = node!.longitude!;
			const lat = node!.latitude!;
			const light = document.documentElement.classList.contains('theme-light');
			map = new maplibregl.Map({
				container: mapEl!,
				style: basemapStyleUrl(light),
				center: [lng, lat],
				zoom: 11,
				attributionControl: { compact: true }
			});
			marker = new maplibregl.Marker({ color: '#34e3c4' }).setLngLat([lng, lat]).addTo(map);
			const m = map;
			m.on('load', () => collapseAttribution(m));
			setTimeout(() => m.resize(), 80);
		});
		return () => {
			map?.remove();
			map = null;
			marker = null;
		};
	});
	// Keep the marker on the node's current location without recreating the map.
	$effect(() => {
		const lat = node?.latitude;
		const lng = node?.longitude;
		if (map && marker && lat != null && lng != null) marker.setLngLat([lng, lat]);
	});

	// --- QR contact code (scannable by the MeshCore app) ---
	let qrSvg = $state('');
	$effect(() => {
		if (!node) return;
		const typeMap: Record<string, number> = { ChatNode: 1, Repeater: 2, RoomServer: 3, Sensor: 4 };
		const t = typeMap[node.role] ?? 2;
		const url = `meshcore://contact/add?name=${encodeURIComponent(node.name || 'Unknown')}&public_key=${node.publicKey}&type=${t}`;
		QRCode.toString(url, { type: 'svg', margin: 1, errorCorrectionLevel: 'M' })
			.then((s) => (qrSvg = s))
			.catch(() => (qrSvg = ''));
	});
</script>

{#if !loaded}
	<div class="panel text-fg-faint px-5 py-12 text-center text-sm">Loading…</div>
{:else if !node}
	<div class="panel text-fg-faint px-5 py-12 text-center text-sm">
		Node not found. It may not have advertised yet.
	</div>
{:else}
	{#if heading}
		<div class="mb-5 flex flex-wrap items-center gap-3">
			<FavoriteStar {pubkey} />
			<h2 class="font-display text-fg text-2xl font-700 tracking-tight">
				{node.name || shortKey(pubkey, 8, 4)}
			</h2>
			<RoleBadge role={node.role} />
			<span class="label rounded-full border px-2 py-0.5" style="color:{status.color};border-color:{status.color}55">{status.label}</span>
		</div>
	{/if}

	<!-- Public key strip -->
	<div class="mb-5 flex items-center gap-2">
		<button onclick={copyKey} class="panel panel-hover flex flex-1 items-center gap-3 px-5 py-3 text-left">
			<span class="label shrink-0">PUBKEY</span>
			<span class="font-mono text-fg break-all text-xs md:text-sm">{pubkey}</span>
			<span class="label ml-auto shrink-0 {copied ? '!text-signal' : ''}">{copied ? 'COPIED' : 'COPY'}</span>
		</button>
	</div>

	<div class="grid gap-5 lg:grid-cols-3">
		<!-- LEFT: map, QR, overview, hash, scores -->
		<div class="space-y-4 lg:col-span-1">
			{#if hasLoc}
				<div bind:this={mapEl} class="border-line h-44 w-full overflow-hidden rounded-[var(--radius)] border"></div>
			{/if}

			{#if qrSvg}
				<div class="panel flex flex-col items-center gap-2 px-5 py-4">
					<div class="label self-start">MeshCore Contact</div>
					<div class="qr w-36 rounded-[var(--radius)] bg-white p-2">{@html qrSvg}</div>
					<div class="text-fg-faint text-center text-[0.62rem]">Scan in the MeshCore app to add</div>
				</div>
			{/if}

			<!-- Overview -->
			<div class="panel divide-line/40 divide-y">
				{#each [{ k: 'Status', v: status.label, c: status.color }, { k: 'Last advert', v: ago(node.lastSeen) + ' ago' }, { k: 'Last relay', v: detail?.relay.lastRelayed ? ago(detail.relay.lastRelayed) + ' ago' + (detail.relay.count1h ? ` · ${detail.relay.count1h}× last hr` : '') : 'none in 24h', c: detail?.relay.lastRelayed ? 'var(--color-fg)' : 'var(--color-fg-faint)' }, { k: 'First seen', v: ago(node.firstSeen) + ' ago' }, { k: 'Packets (6h)', v: detail ? `${detail.totalPackets}` + (detail.totalObservations !== detail.totalPackets ? ` (seen ${detail.totalObservations}×)` : '') : '—' }, { k: 'Packets today', v: detail ? String(detail.packetsToday) : '—' }, { k: 'Adverts (all-time)', v: String(node.advertCount) }, { k: 'Advert cadence', v: cadence(detail?.advertIntervalSec) }, { k: 'Avg SNR', v: detail?.avgSnr != null ? detail.avgSnr.toFixed(1) + ' dB' : '—' }, { k: 'Avg hops', v: detail?.avgHops != null ? detail.avgHops.toFixed(1) : '—' }, { k: 'Location', v: fmtCoord(node.latitude, node.longitude) }] as f (f.k)}
					<div class="flex items-center justify-between px-5 py-2.5">
						<span class="label normal-case">{f.k}</span>
						<span class="font-mono text-sm tnum" style="color:{f.c ?? 'var(--color-fg)'}">{f.v}</span>
					</div>
				{/each}
			</div>

			<!-- Advert activity sparkline (per-hour over the window) -->
			{#if detail?.activity?.length}
				<div class="panel px-5 py-4">
					<div class="label mb-3 flex items-center justify-between">
						<span>Advert Activity</span>
						<span class="font-mono text-fg-faint normal-case">last {detail.windowHours}h</span>
					</div>
					<div class="flex h-12 items-end gap-1">
						{#each detail.activity as count, i (i)}
							<Tooltip
								text="{count} advert{count === 1 ? '' : 's'} · {detail.windowHours - 1 - i === 0 ? 'this hour' : `${detail.windowHours - 1 - i}h ago`}"
								class="h-full min-w-0 flex-1 items-end"
							>
								<div
									class="bg-signal/80 hover:bg-signal w-full rounded-sm transition-all"
									style="height:{count === 0 ? 2 : Math.max(8, (count / activityMax) * 100)}%;{count === 0 ? 'opacity:0.25' : ''}"
								></div>
							</Tooltip>
						{/each}
					</div>
					<div class="text-fg-faint mt-1.5 flex justify-between text-[0.58rem]">
						<span>-{detail.windowHours}h</span><span>now</span>
					</div>
				</div>
			{/if}

			<!-- Hash ID -->
			<div class="panel px-5 py-4">
				<div class="label mb-3 flex items-center justify-between">
					Hash ID
					{#if hashId}<span class="font-mono text-fg-faint normal-case">{hashId.bytes}-byte</span>{/if}
				</div>
				{#if hashId}
					<div class="flex items-baseline gap-3">
						<span class="font-mono text-signal glow-signal text-2xl font-700 tracking-[0.15em]">{hashId.hex}</span>
						{#if hashId.shared === 0}
							<span class="font-mono text-signal bg-signal/10 rounded-[var(--radius)] px-1.5 py-0.5 text-[0.62rem]">unique</span>
						{:else}
							<span class="font-mono text-amber bg-amber/10 rounded-[var(--radius)] px-1.5 py-0.5 text-[0.62rem] tnum">+{hashId.shared} collision{hashId.shared > 1 ? 's' : ''}</span>
						{/if}
					</div>
				{:else}
					<div class="text-fg-faint text-sm">Unknown — not seen advertising yet.</div>
				{/if}
			</div>

			<!-- Relay role + scores (repeaters / room servers) -->
			{#if isRelay && detail}
				<div class="panel px-5 py-4">
					<div class="label mb-3">Relay Activity</div>
					<div class="divide-line/40 divide-y text-sm">
						<div class="flex items-center justify-between py-2">
							<span class="label normal-case">Last relayed</span>
							<span class="font-mono text-xs tnum" style="color:{detail.relay.active ? 'var(--color-signal)' : 'var(--color-fg-dim)'}">
								{detail.relay.lastRelayed ? ago(detail.relay.lastRelayed) + ' ago' : 'never'}
							</span>
						</div>
						<div class="flex items-center justify-between py-2">
							<span class="label normal-case">Relays 1h / 24h</span>
							<span class="font-mono text-fg text-xs tnum">{detail.relay.count1h} / {detail.relay.count24h}</span>
						</div>
							<div class="py-2">
							<div class="mb-1 flex items-center justify-between">
								<Tooltip text="Fraction of relayed traffic that transited this node (24h window)"><span class="label normal-case">Traffic share</span></Tooltip>
								<span class="font-mono text-xs" style="color:{tl.color}">{(detail.trafficShare * 100).toFixed(1)}% · {tl.label}</span>
							</div>
							<div class="bg-panel-2 h-1.5 w-full overflow-hidden rounded-full">
								<div class="h-full rounded-full" style="width:{Math.max(2, detail.trafficShare * 100)}%;background:{tl.color}"></div>
							</div>
						</div>
							<div class="py-2">
							<div class="mb-1 flex items-center justify-between">
								<Tooltip text="Betweenness centrality — how often this node sits on shortest paths (1.0 = most structurally critical)"><span class="label normal-case">Bridge score</span></Tooltip>
								<span class="font-mono text-xs" style="color:{bl.color}">{(detail.bridge * 100).toFixed(1)}% · {bl.label}</span>
							</div>
							<div class="bg-panel-2 h-1.5 w-full overflow-hidden rounded-full">
								<div class="h-full rounded-full" style="width:{Math.max(2, detail.bridge * 100)}%;background:{bl.color}"></div>
							</div>
						</div>
					</div>
				</div>
			{/if}
		</div>

		<!-- RIGHT: observers, recent packets, neighbors -->
		<div class="space-y-4 lg:col-span-2">
			<!-- Heard By -->
			<section class="panel">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3">
					<h3 class="font-display text-fg text-sm font-700 tracking-wide">HEARD BY</h3>
					<span class="label ml-auto tnum">{detail?.observers.length ?? 0} observer{(detail?.observers.length ?? 0) === 1 ? '' : 's'}</span>
				</div>
				{#if !detail || detail.observers.length === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">No observations in the last 6 hours.</div>
				{:else}
					<div class="divide-line/40 divide-y">
						{#each detail.observers as o (o.id)}
							<div class="flex items-center gap-3 px-5 py-2 text-sm">
								<span class="text-fg min-w-0 flex-1 truncate font-mono text-xs">{o.id}</span>
								{#if o.region}<span class="label !text-[0.58rem]">{o.region}</span>{/if}
								<span class="font-mono text-fg-faint w-12 text-right text-xs tnum">{o.count} pkt</span>
								<span class="font-mono w-14 text-right text-xs tnum" style="color:{snrColor(o.avgSnr)}">{o.avgSnr != null ? o.avgSnr.toFixed(1) + ' dB' : '—'}</span>
								<span class="font-mono text-fg-faint w-16 text-right text-xs tnum">{o.avgRssi != null ? o.avgRssi.toFixed(0) + ' dBm' : '—'}</span>
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<!-- Recent packets -->
			<section class="panel">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3">
					<h3 class="font-display text-fg text-sm font-700 tracking-wide">RECENT PACKETS</h3>
					<span class="label ml-auto tnum">{detail?.recentPackets.length ?? 0}</span>
				</div>
				{#if !detail || detail.recentPackets.length === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">No recent packets from this node.</div>
				{:else}
					<div class="divide-line/40 divide-y">
						{#each detail.recentPackets as p (p.messageHash + p.receivedAt)}
							<div class="flex items-center gap-3 px-5 py-2 text-sm">
								<Tooltip text={fmtAbs(p.receivedAt)} class="w-10 shrink-0"><span class="font-mono text-fg-faint text-xs tnum">{ago(p.receivedAt)}</span></Tooltip>
								<PayloadTag type={p.payloadType} />
								<span class="font-mono text-fg-faint min-w-0 flex-1 truncate text-xs">via {p.observerId ?? '—'}</span>
								<span class="font-mono text-fg-faint text-xs tnum">{p.pathHops} hop{p.pathHops === 1 ? '' : 's'}</span>
								<span class="font-mono w-14 text-right text-xs tnum" style="color:{snrColor(p.snr)}">{fmtSnr(p.snr)} dB</span>
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<!-- Activity history (stored observations, on demand) -->
			<section class="panel">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3">
					<h3 class="font-display text-fg text-sm font-700 tracking-wide">ACTIVITY HISTORY</h3>
					<div class="ml-auto flex items-center gap-1">
						{#each ranges as r (r.sec)}
							<button
								onclick={() => setRange(r.sec)}
								class="label rounded-[var(--radius)] border px-2 py-0.5 transition-colors {histRange === r.sec ? 'border-signal text-signal' : 'border-line text-fg-faint hover:text-fg'}"
								>{r.label}</button
							>
						{/each}
					</div>
				</div>
				{#if histLoading && history.length === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">Loading…</div>
				{:else if history.length === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">No observations in this range.</div>
				{:else}
					<div class="divide-line/40 max-h-96 divide-y overflow-y-auto">
						{#each history as h (h.messageHash + h.receivedAt + h.kind + h.hopIndex)}
							<div class="flex items-center gap-3 px-5 py-2 text-sm">
								<Tooltip text={fmtAbs(h.receivedAt)} class="w-10 shrink-0"><span class="font-mono text-fg-faint text-xs tnum">{ago(h.receivedAt)}</span></Tooltip>
								<span class="label shrink-0 rounded px-1.5 py-0.5 text-[0.56rem] {h.kind === 'advert' ? 'text-signal bg-signal/10' : 'text-sky bg-sky/10'}">{h.kind === 'advert' ? 'SENT' : 'RELAY'}</span>
								<PayloadTag type={h.payloadType} />
								<span class="font-mono text-fg-faint min-w-0 flex-1 truncate text-xs">via {h.observerId ?? '—'}</span>
								{#if h.kind === 'relay'}<Tooltip text="this node's position in the packet's path" class="shrink-0"><span class="font-mono text-fg-faint text-xs tnum">hop {h.hopIndex + 1}/{h.pathHops}</span></Tooltip>{/if}
								<span class="font-mono w-14 shrink-0 text-right text-xs tnum" style="color:{snrColor(h.snr)}">{fmtSnr(h.snr)} dB</span>
							</div>
						{/each}
					</div>
					<div class="text-fg-faint border-line/50 border-t px-5 py-2 text-center text-[0.62rem]">
						{history.length} observation{history.length === 1 ? '' : 's'} · adverts sent + packets relayed{history.length >= 300 ? ' (capped)' : ''}
					</div>
				{/if}
			</section>

			<!-- Neighbors -->
			<section class="panel">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3">
					<h3 class="font-display text-fg text-sm font-700 tracking-wide">NEIGHBORS</h3>
					<span class="label ml-auto tnum">{detail?.neighbors.length ?? 0}</span>
				</div>
				{#if !detail || detail.neighbors.length === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">No path-adjacent nodes observed in the window.</div>
				{:else}
					<div class="divide-line/40 divide-y">
						{#each detail.neighbors as nb (nb.publicKey)}
							<a href="/nodes/{nb.publicKey}" class="panel-hover flex items-center gap-3 px-5 py-2 text-sm">
								<span class="h-2 w-2 shrink-0 rounded-full" style="background:{roleColor(nb.role)}"></span>
								<span class="text-fg min-w-0 flex-1 truncate">{nb.name}</span>
								<span class="label !text-[0.58rem]">{roleLabel(nb.role)}</span>
								<span class="font-mono text-fg-faint text-xs tnum">×{nb.count}</span>
							</a>
						{/each}
					</div>
				{/if}
			</section>
		</div>
	</div>
{/if}

<style>
	.qr :global(svg) {
		display: block;
		width: 100%;
		height: auto;
	}
</style>
