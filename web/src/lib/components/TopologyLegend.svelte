<script lang="ts">
	// Legend for the topology views: colour ramp = forwarding volume, plus a
	// marker key for only the roles actually on screen.
	import type { TopologyNode } from '$lib/api';
	import { ROLE_SHAPE, ROLE_NAME } from '$lib/topology';

	let { nodes, layout }: { nodes: TopologyNode[]; layout: 'graph' | 'geo' } = $props();

	const roles = $derived(
		[...new Set(nodes.map((n) => n.role))]
			.filter((r) => ROLE_NAME[r])
			.sort((a, b) => (a === 'Repeater' ? -1 : b === 'Repeater' ? 1 : a.localeCompare(b)))
	);
	const count = (r: string) => nodes.filter((n) => n.role === r).length;
</script>

<div
	class="border-line/70 bg-panel/85 text-fg-dim pointer-events-none absolute bottom-3 left-3 flex max-w-[calc(100%-1.5rem)] flex-col gap-1.5 rounded-[var(--radius)] border px-3 py-2 font-mono text-[0.62rem] backdrop-blur"
>
	<div class="flex items-center gap-2">
		<span>Forwarded</span>
		<span class="text-fg-faint">less</span>
		<span
			class="inline-block h-2 w-24 rounded-full"
			style="background:linear-gradient(90deg,var(--color-fg-faint),var(--color-signal),var(--color-amber))"
		></span>
		<span class="text-fg-faint">more</span>
	</div>
	<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
		{#each roles as r (r)}
			<span class="flex items-center gap-1.5">
				<svg width="11" height="11" viewBox="-6 -6 12 12" class="text-fg-dim" aria-hidden="true">
					{#if layout === 'geo'}
						<circle r="4" fill="var(--color-signal)" stroke={r === 'Repeater' ? 'var(--color-ink)' : 'var(--color-fg)'} stroke-width={r === 'Repeater' ? 1 : 2} />
					{:else if ROLE_SHAPE[r] === 'square'}
						<rect x="-4" y="-4" width="8" height="8" fill="currentColor" />
					{:else if ROLE_SHAPE[r] === 'diamond'}
						<path d="M0-5L5 0L0 5L-5 0Z" fill="currentColor" />
					{:else if ROLE_SHAPE[r] === 'triangle'}
						<path d="M0-5L4.5 3.5L-4.5 3.5Z" fill="currentColor" />
					{:else}
						<circle r="4.5" fill="currentColor" />
					{/if}
				</svg>
				{ROLE_NAME[r]} <span class="text-fg-faint tnum">{count(r)}</span>
			</span>
		{/each}
		<span class="flex items-center gap-1.5">
			<svg width="22" height="8" aria-hidden="true"
				><line x1="1" y1="2" x2="21" y2="2" stroke="currentColor" stroke-width="0.8" /><line
					x1="1"
					y1="6.5"
					x2="21"
					y2="6.5"
					stroke="currentColor"
					stroke-width="3"
				/></svg
			>
			link width = times seen
		</span>
	</div>
	<span class="text-fg-faint">
		{layout === 'geo' ? 'click a relay for its links' : 'scroll / pinch = zoom · drag = pan · drag a node = move it · click = details'}
	</span>
</div>
