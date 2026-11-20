<script lang="ts">
	import { onMount } from 'svelte';
	import Seo from '$lib/components/Seo.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import WindowToggle from '$lib/components/WindowToggle.svelte';
	import TopologyGraph from '$lib/components/TopologyGraph.svelte';
	import TopologyMap from '$lib/components/TopologyMap.svelte';
	import TopologyLegend from '$lib/components/TopologyLegend.svelte';
	import { api, type MeshAnalytics } from '$lib/api';
	import { roleLabel, shortKey, fmtNum } from '$lib/format';
	import { hasWebGL } from '$lib/webgl';
	import { visibleEdges, stepsFor, autoMinWeight, linksOf } from '$lib/topology';

	const windows = [
		{ label: '1h', sec: 3600, bucket: 2 },
		{ label: '6h', sec: 21600, bucket: 10 },
		{ label: '24h', sec: 86400, bucket: 30 }
	];
	let windowSec = $state(21600);
	let bucketMin = $derived(windows.find((w) => w.sec === windowSec)?.bucket ?? 10);
	let data = $state<MeshAnalytics | null>(null);
	let error = $state<string | null>(null);
	let loading = $state(true);

	// Layout: force graph or real positions. Remembered per viewer.
	const LAYOUT_KEY = 'ridgeline-topology-layout';
	let layout = $state<'graph' | 'geo'>('graph');
	let webglOk = $state(true);
	function setLayout(l: 'graph' | 'geo') {
		layout = l;
		try {
			localStorage.setItem(LAYOUT_KEY, l);
		} catch {
			/* storage unavailable */
		}
	}

	async function refresh() {
		try {
			data = await api.meshAnalytics(windowSec, bucketMin);
			error = null;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}
	// Reload on window change; poll while mounted (the graph only remounts on a
	// window change, so panning/zoom survives the 30s refresh).
	$effect(() => {
		windowSec;
		pickedStep = null; // each window gets its own sensible default
		selected = null;
		refresh();
	});
	onMount(() => {
		webglOk = hasWebGL();
		try {
			const saved = localStorage.getItem(LAYOUT_KEY);
			if (saved === 'geo' && webglOk) layout = 'geo';
		} catch {
			/* storage unavailable */
		}
		const t = setInterval(refresh, 30000);
		return () => clearInterval(t);
	});

	const topo = $derived(data?.topology);
	const hasTopo = $derived(!!topo && topo.nodes.length > 0 && topo.edges.length > 0);

	// Link threshold: auto until the viewer moves the slider.
	let pickedStep = $state<number | null>(null);
	const steps = $derived(topo ? stepsFor(topo.edges) : [1]);
	const autoStep = $derived(topo ? autoMinWeight(topo.edges, topo.nodes.length) : 1);
	const minWeight = $derived(pickedStep ?? autoStep);
	const stepIdx = $derived(Math.max(0, steps.indexOf(minWeight)));
	const shown = $derived(topo ? visibleEdges(topo.edges, minWeight).length : 0);

	// Selection → side panel
	let selected = $state<string | null>(null);
	const byKey = $derived(new Map((topo?.nodes ?? []).map((n) => [n.publicKey, n])));
	const selNode = $derived(selected ? byKey.get(selected) : undefined);
	const selLinks = $derived(selected && topo ? linksOf(selected, topo.edges) : []);
	const selMax = $derived(Math.max(1, ...selLinks.map((l) => l.weight)));
	const rank = $derived(
		selNode && topo ? [...topo.nodes].sort((a, b) => b.relayed - a.relayed).findIndex((n) => n.publicKey === selNode.publicKey) + 1 : 0
	);
	const label = (k: string) => byKey.get(k)?.name || shortKey(k);
</script>

<Seo
	title="Topology — Ridgeline"
	description="Interactive mesh topology for the MeshCore network — the relay backbone of nodes that hand off traffic to one another, inferred from observed flood paths."
	path="/topology"
/>

<PageHeader eyebrow="Network Observatory" title="Topology">
	<WindowToggle options={windows} bind:value={windowSec} />
</PageHeader>

<div class="px-6 py-6 md:px-10">
	{#if error}
		<div class="panel border-coral/40 text-coral mb-6 px-4 py-3 text-sm">
			Can't reach the daemon — {error}
		</div>
	{/if}

	<section class="panel rise flex h-[calc(100dvh-13rem)] min-h-[520px] flex-col overflow-hidden">
		<div class="border-line/70 flex flex-wrap items-center gap-x-4 gap-y-2 border-b px-5 py-3">
			<h2 class="font-display text-fg text-sm font-700 tracking-wide">RELAY BACKBONE</h2>

			<div data-seg class="bg-panel border-line flex overflow-hidden rounded-[var(--radius)] border text-xs">
				<button
					onclick={() => setLayout('graph')}
					aria-pressed={layout === 'graph'}
					class="font-mono px-3 py-1.5 transition-colors {layout === 'graph'
						? 'bg-signal/15 text-signal'
						: 'text-fg-dim hover:text-fg'}">Graph</button
				>
				<button
					onclick={() => setLayout('geo')}
					disabled={!webglOk}
					title={webglOk ? 'Relays at their advertised positions' : 'Needs WebGL, which this browser has disabled'}
					aria-pressed={layout === 'geo'}
					class="font-mono px-3 py-1.5 transition-colors disabled:opacity-40 {layout === 'geo'
						? 'bg-signal/15 text-signal'
						: 'text-fg-dim hover:text-fg'}">Map</button
				>
			</div>

			{#if hasTopo && topo}
				<label class="text-fg-dim flex items-center gap-2 font-mono text-[0.68rem]">
					<span class="whitespace-nowrap">Links seen ≥ <span class="text-fg tnum">{minWeight}</span>×</span>
					<input
						type="range"
						min="0"
						max={steps.length - 1}
						step="1"
						value={stepIdx}
						oninput={(e) => (pickedStep = steps[+(e.currentTarget as HTMLInputElement).value])}
						class="accent-[var(--color-signal)] w-32"
						aria-label="Minimum times a link was seen"
					/>
				</label>
				{#if pickedStep !== null && pickedStep !== autoStep}
					<button onclick={() => (pickedStep = null)} class="text-fg-faint hover:text-signal font-mono text-[0.62rem]"
						>auto</button
					>
				{/if}
				<span class="font-mono text-fg-faint ml-auto text-[0.68rem] tnum"
					>{topo.nodes.length} relays · {shown} of {topo.edges.length} links</span
				>
			{/if}
		</div>

		<div class="relative flex-1 overflow-hidden">
			{#if loading && !data}
				<div class="text-fg-faint flex h-full items-center justify-center text-sm">Loading…</div>
			{:else if !hasTopo || !topo}
				<div class="text-fg-faint flex h-full items-center justify-center px-6 text-center text-sm">
					Not enough multi-hop relaying in this window to map the backbone — try a longer window.
				</div>
			{:else}
				{#key `${windowSec}-${layout}`}
					{#if layout === 'geo'}
						<TopologyMap nodes={topo.nodes} edges={topo.edges} {minWeight} {selected} onselect={(k) => (selected = k)} />
					{:else}
						<TopologyGraph nodes={topo.nodes} edges={topo.edges} {minWeight} {selected} onselect={(k) => (selected = k)} />
					{/if}
				{/key}

				<TopologyLegend nodes={topo.nodes} {layout} />

				{#if selNode}
					<aside
						class="border-line/70 bg-panel/92 absolute top-12 right-3 bottom-3 flex w-72 max-w-[calc(100%-1.5rem)] flex-col overflow-hidden rounded-[var(--radius)] border backdrop-blur"
					>
						<div class="border-line/60 border-b px-4 py-3">
							<div class="flex items-start gap-2">
								<div class="min-w-0 flex-1">
									<div class="text-fg truncate text-sm font-600">{selNode.name || shortKey(selNode.publicKey)}</div>
									<div class="text-fg-faint font-mono text-[0.65rem]">
										{roleLabel(selNode.role)} · {shortKey(selNode.publicKey)}
									</div>
								</div>
								<button onclick={() => (selected = null)} class="text-fg-faint hover:text-fg -mt-0.5 text-lg leading-none" aria-label="Close"
									>×</button
								>
							</div>
							<div class="text-fg-dim mt-2 font-mono text-[0.68rem]">
								Forwarded <span class="text-fg tnum">{fmtNum(selNode.relayed)}</span> transmissions · #{rank} of {topo.nodes.length}
							</div>
						</div>
						<div class="text-fg-faint px-4 pt-3 pb-1 font-mono text-[0.62rem] tracking-wide">
							{selLinks.length} LINK{selLinks.length === 1 ? '' : 'S'} · STRONGEST FIRST
						</div>
						<ul class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
							{#each selLinks as l (l.other)}
								<li>
									<button
										onclick={() => (selected = l.other)}
										class="hover:bg-signal/10 group flex w-full items-center gap-2 rounded px-2 py-1.5 text-left"
									>
										<span class="text-fg-dim group-hover:text-fg min-w-0 flex-1 truncate text-xs"
											>{label(l.other)}{#if l.inferred}<span class="text-fg-faint" title="Inferred from 1-byte hops"> ·inferred</span>{/if}</span
										>
										<span class="bg-line/60 relative h-1.5 w-14 shrink-0 overflow-hidden rounded-full">
											<span
												class="bg-signal absolute inset-y-0 left-0 rounded-full {l.weight < minWeight ? 'opacity-40' : ''}"
												style="width:{Math.max(4, (l.weight / selMax) * 100)}%"
											></span>
										</span>
										<span class="text-fg-faint w-9 shrink-0 text-right font-mono text-[0.65rem] tnum">{l.weight}×</span>
									</button>
								</li>
							{/each}
						</ul>
						<a
							href="/nodes/{selNode.publicKey}"
							class="border-line/60 text-signal hover:bg-signal/10 border-t px-4 py-2.5 text-center font-mono text-xs font-600"
							>Open node →</a
						>
					</aside>
				{/if}
			{/if}
		</div>
	</section>

	{#if topo?.edges?.some((e) => e.inferred)}
		<p class="text-fg-faint mt-3 text-center text-[0.68rem] leading-relaxed">
			<span class="text-fg-dim">Dashed links are inferred.</span> Every observation of them was resolved
			from 1-byte path hops, which only distinguish 256 nodes — so the adjacency is probably real, but
			it is not a measurement.
			<a href="/hash-ids#why" class="text-signal hover:underline">Why →</a>
		</p>
	{/if}
	<p class="text-fg-faint mt-3 text-center font-mono text-[0.62rem]">
		Inferred from observed flood paths — who hands off to whom across the mesh. Weak links are hidden
		(each relay always keeps its strongest); click a relay to see all of its links. Window {data?.windowHours?.toFixed(
			1
		) ?? '—'}h · updates every 30s.
	</p>
</div>
