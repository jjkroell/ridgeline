<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import { onMount } from 'svelte';
	import { swipeAction } from '$lib/mobile/swipe';
	import { longpress, nodeMenu, toggleStar } from '$lib/mobile/context-menu.svelte';
	import { api, type Node } from '$lib/api';
	import { ago, shortKey, nodeStatus, roleColor, roleLabel, lastHeard } from '$lib/format';
	import { favorites } from '$lib/favorites.svelte';
	import ClaimedBadge from '$lib/components/ClaimedBadge.svelte';
	import ObserverBadge from '$lib/components/ObserverBadge.svelte';
	import NodeFilterModal from '$lib/components/NodeFilterModal.svelte';
	import { NodeFilters, indexLetter } from '$lib/node-filters.svelte';
	import SearchField from '$lib/mobile/SearchField.svelte';
	import IndexStrip from '$lib/mobile/IndexStrip.svelte';

	let nodes = $state<Node[]>([]);
	const filters = new NodeFilters();
	let filterOpen = $state(false);

	async function refresh() {
		try {
			nodes = await api.nodes();
		} catch {
			/* keep last */
		}
	}
	onMount(() => {
		refresh();
		const t = setInterval(refresh, 10000);
		return () => clearInterval(t);
	});

	// Filtering + ordering live in NodeFilters so /nodes and /m/nodes agree.
	const filtered = $derived(filters.apply(nodes));

	// A–Z: the first row of each letter carries an id the strip jumps to.
	const firstOf = $derived.by(() => {
		const m = new Map<string, string>();
		if (filters.sort !== 'name') return m;
		for (const n of filtered) {
			const l = indexLetter(n);
			if (!m.has(l)) m.set(l, n.publicKey);
		}
		return m;
	});
	const letterRow = $derived(new Map([...firstOf].map(([l, k]) => [k, l])));
	function jump(l: string) {
		document.getElementById(`az-${l === '#' ? 'other' : l}`)?.scrollIntoView({ block: 'start' });
	}
</script>

<Seo title="Nodes" description="Every node heard on the mesh." path="/m/nodes" />

<!-- Room on the right for the A–Z strip. -->
<div class="py-4 pl-4 {filters.sort === 'name' && filtered.length > 20 ? 'pr-7' : 'pr-4'}">
	<!-- Controls: quiet by design — search (with the filter inside it) and a
	     count line that names the active filters and the order. -->
	<SearchField bind:value={filters.query} placeholder="Search nodes">
		{#snippet trailing()}
			<button
				onclick={() => (filterOpen = true)}
				class="-my-1 -mr-1 shrink-0 p-1 {filters.activeCount ? 'text-signal' : 'text-fg-faint'}"
				aria-label={filters.activeCount ? `Filters: ${filters.summary}` : 'Filter'}
			>
				<svg viewBox="0 0 24 24" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 5h18M7 12h10M11 19h2" /></svg>
			</button>
		{/snippet}
	</SearchField>

	<div class="mb-2 flex items-center gap-2 px-1">
		<div class="text-fg-faint min-w-0 flex-1 truncate font-mono text-[0.62rem]">
			{filtered.length} node{filtered.length === 1 ? '' : 's'}{#if filters.summary}<span class="text-signal"> · {filters.summary}</span>{/if}
		</div>
		<div data-seg class="border-line flex shrink-0 rounded-lg border p-0.5 text-[0.68rem]" role="radiogroup" aria-label="Order">
			<button role="radio" aria-checked={filters.sort === 'heard'} onclick={() => (filters.sort = 'heard')} class="rounded-md px-2.5 py-0.5 {filters.sort === 'heard' ? 'bg-signal/15 text-signal' : 'text-fg-dim'}">Recent</button>
			<button role="radio" aria-checked={filters.sort === 'name'} onclick={() => (filters.sort = 'name')} class="rounded-md px-2.5 py-0.5 {filters.sort === 'name' ? 'bg-signal/15 text-signal' : 'text-fg-dim'}">A–Z</button>
		</div>
	</div>

	<!-- List -->
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border">
		{#if filtered.length === 0}
			<div class="px-4 py-10 text-center">
				<p class="text-fg-dim text-sm">
					{#if filters.favOnly && !favorites.count}
						No favorites yet. Star a node to keep it at the top.
					{:else if filters.claimedOnly}
						No claimed nodes match.
					{:else}
						No nodes match these filters.
					{/if}
				</p>
				{#if filters.activeCount}
					<button onclick={() => filters.clear()} class="text-signal mt-2 text-xs">Clear filters</button>
				{/if}
			</div>
		{:else}
			{#each filtered as n (n.publicKey)}
				{@const st = nodeStatus(n)}
				{@const heard = lastHeard(n)}
				{@const fav = favorites.has(n.publicKey)}
				{@const letter = letterRow.get(n.publicKey)}
				{#if letter}
					<div id="az-{letter === '#' ? 'other' : letter}" data-az-head class="bg-panel-2 text-fg-dim px-4 py-1 text-[0.72rem] font-600">{letter}</div>
				{/if}
				<div data-swipe class="relative overflow-hidden">
				<!-- Revealed by swiping the row left; release to toggle the star. -->
				<div class="bg-amber absolute inset-y-0 right-0 flex w-24 items-center justify-center text-[0.7rem] font-600 text-white" style="opacity:var(--swipe, 0)" aria-hidden="true">
					<span class="flex flex-col items-center gap-0.5">
						<svg viewBox="0 0 24 24" class="h-5 w-5" fill={fav ? 'none' : 'currentColor'} stroke="currentColor" stroke-width="1.6"><path d="M12 2.5l2.9 5.9 6.5.95-4.7 4.6 1.1 6.45L12 17.9l-5.8 3.05 1.1-6.45-4.7-4.6 6.5-.95z" /></svg>
						{fav ? 'Unstar' : 'Star'}
					</span>
				</div>
				<a href="/m/nodes/{n.publicKey}" use:swipeAction={() => toggleStar(n.publicKey)} use:longpress={() => nodeMenu(n)} class="bg-panel active:bg-line/40 relative flex items-center gap-3 px-4 py-3">
					<span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background:{st.color}"></span>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-1.5">
							<span class="text-fg truncate text-sm font-medium">{n.name || shortKey(n.publicKey)}</span>
							{#if n.observerId}<ObserverBadge id={n.observerId} />{/if}
							{#if n.claimed}<ClaimedBadge pubkey={n.publicKey} />{/if}
						</div>
						<div class="mt-0.5 flex items-center gap-1.5 font-mono text-[0.62rem]">
							<span style="color:{roleColor(n.role)}">{roleLabel(n.role)}</span>
							<span class="text-fg-faint">· {st.label} · {ago(heard)}</span>
						</div>
					</div>
					<button
						onclick={(e) => {
							e.preventDefault();
							favorites.toggle(n.publicKey);
						}}
						class="shrink-0 p-1.5"
						aria-label="Toggle favorite"
					>
						<svg viewBox="0 0 24 24" class="h-5 w-5 {favorites.has(n.publicKey) ? 'text-amber' : 'text-fg-faint'}" fill={favorites.has(n.publicKey) ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.5"><path d="M12 2.5l2.9 5.9 6.5.95-4.7 4.6 1.1 6.45L12 17.9l-5.8 3.05 1.1-6.45-4.7-4.6 6.5-.95z" /></svg>
					</button>
				</a>
				</div>
			{/each}
		{/if}
	</div>
</div>

{#if filters.sort === 'name' && filtered.length > 20}
	<IndexStrip letters={new Set(firstOf.keys())} onjump={jump} />
{/if}

{#if filterOpen}
	<NodeFilterModal {filters} {nodes} onclose={() => (filterOpen = false)} />
{/if}
