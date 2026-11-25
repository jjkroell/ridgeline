<script lang="ts">
	import { FEED_TYPES, matchingNodes, spanLabel, type FeedFilter } from '$lib/feed-filter.svelte';
	import type { Node } from '$lib/api';
	import { ago } from '$lib/format';
	import { serverNow } from '$lib/server-clock.svelte';

	// Search box + packet-type chips for the Feed page (desktop and phone).
	let {
		filter,
		counts,
		shown,
		total,
		oldest,
		nodes = [],
		nodeBase = '/nodes/',
		compact = false
	}: {
		filter: FeedFilter;
		counts: Record<string, number>;
		shown: number;
		total: number;
		/** receivedAt of the oldest packet the page holds. */
		oldest?: string;
		nodes?: Node[];
		nodeBase?: string;
		compact?: boolean;
	} = $props();

	const span = $derived(spanLabel(oldest, serverNow()));
	const hits = $derived(matchingNodes(nodes, filter.q));

	let input = $state<HTMLInputElement>();
	// "/" jumps to the search box on desktop, like most sites' search.
	function onKey(e: KeyboardEvent) {
		if (compact || e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey) return;
		const t = e.target as HTMLElement;
		if (t.closest('input, textarea, select, [contenteditable="true"]')) return;
		e.preventDefault();
		input?.focus();
	}
</script>

<svelte:window onkeydown={onKey} />

<div class={compact ? 'mb-3 space-y-2' : 'panel mb-4 space-y-3 px-4 py-3'}>
	<div class="flex items-center gap-2">
		<label class="relative min-w-0 flex-1">
			<span class="sr-only">Search the feed</span>
			<svg
				viewBox="0 0 24 24"
				class="text-fg-faint pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2"
				fill="none"
				stroke="currentColor"
				stroke-width="1.8"
				stroke-linecap="round"
				aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg
			>
			<input
				bind:this={input}
				type="search"
				value={filter.q}
				oninput={(e) => filter.setQuery((e.currentTarget as HTMLInputElement).value)}
				onkeydown={(e) => e.key === 'Escape' && filter.setQuery('')}
				placeholder={compact ? 'Search names, messages…' : 'Search node, observer, channel, message, repeater, hash…   ( / )'}
				autocomplete="off"
				autocapitalize="off"
				spellcheck="false"
				class="border-line bg-ink-2 text-fg placeholder:text-fg-faint focus:border-signal/60 w-full border py-2 pr-3 pl-9 text-base outline-none md:text-sm {compact
					? 'rounded-full'
					: 'rounded-[var(--radius)]'}"
			/>
		</label>
		{#if filter.active}
			<button
				type="button"
				onclick={() => filter.clear()}
				class="border-line text-fg-dim hover:text-fg shrink-0 border px-3 py-2 text-xs font-600 {compact
					? 'rounded-full'
					: 'rounded-[var(--radius)]'}">Clear</button
			>
		{/if}
	</div>

	<div class="flex gap-1.5 {compact ? '-mx-4 overflow-x-auto px-4 pb-0.5 [scrollbar-width:none]' : 'flex-wrap'}">
		{#each FEED_TYPES as t (t.id)}
			{@const on = filter.types.includes(t.id)}
			{@const n = counts[t.id] ?? 0}
			<button
				type="button"
				onclick={() => filter.toggle(t.id)}
				aria-pressed={on}
				class="flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors {on
					? 'border-signal/60 text-signal bg-signal/10'
					: n
						? 'border-line text-fg-dim hover:border-line-bright hover:text-fg'
						: 'border-line/50 text-fg-faint'}"
			>
				{t.label}<span class="tnum font-mono text-[0.66rem] opacity-75">{n}</span>
			</button>
		{/each}
	</div>

	{#if filter.active}
		<p class="text-fg-faint text-xs" aria-live="polite">
			Showing <span class="text-fg tnum">{shown}</span> of <span class="tnum">{total}</span> packets from {span}.{#if !compact}{' '}Every word must match; start a word with <span class="font-mono">-</span> to exclude it.{/if}
		</p>
	{/if}

	{#if hits.length}
		<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
			<span class="text-fg-faint">Matching {hits.length === 1 ? 'node' : 'nodes'}:</span>
			{#each hits as n (n.publicKey)}
				<a href={nodeBase + n.publicKey} class="text-signal hover:underline"
					>{n.name || n.publicKey.slice(0, 8)}<span class="text-fg-faint">{' '}· last heard {ago(n.lastHeard ?? n.lastSeen)} ago</span
					></a
				>
			{/each}
		</div>
	{/if}
</div>
