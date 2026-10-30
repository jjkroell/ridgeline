<!--
  Renders the user's chosen Overview widgets, in their saved order, into a
  responsive grid (2 columns on desktop, 1 on mobile). Shared by the desktop (/)
  and mobile (/m) Overview pages; `base` is '' or '/m' so widget links route to
  the right app. The top stat row lives on the pages, not here.
-->
<script lang="ts">
	import { overview } from '$lib/overview.svelte';
	import { theme } from '$lib/theme.svelte';
	import WidgetSlot from './WidgetSlot.svelte';
	import type { Node, Stats } from '$lib/api';

	import FavoritesWidget from './FavoritesWidget.svelte';
	import LiveFeedWidget from './LiveFeedWidget.svelte';
	import RecentNodesWidget from './RecentNodesWidget.svelte';
	import ClaimedNodesWidget from './ClaimedNodesWidget.svelte';
	import MyNodesWidget from './MyNodesWidget.svelte';
	import NewNodesWidget from './NewNodesWidget.svelte';
	import BackboneWidget from './BackboneWidget.svelte';
	import MiniMapWidget from './MiniMapWidget.svelte';
	import ActivityWidget from './ActivityWidget.svelte';
	import ObserversWidget from './ObserversWidget.svelte';
	import ChannelsWidget from './ChannelsWidget.svelte';

	let { base = '', nodes = [], stats = null }: { base?: string; nodes?: Node[]; stats?: Stats | null } =
		$props();

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const COMPONENTS: Record<string, any> = {
		favorites: FavoritesWidget,
		livefeed: LiveFeedWidget,
		recentnodes: RecentNodesWidget,
		claimed: ClaimedNodesWidget,
		mynodes: MyNodesWidget,
		newnodes: NewNodesWidget,
		backbone: BackboneWidget,
		minimap: MiniMapWidget,
		activity: ActivityWidget,
		observers: ObserversWidget,
		channels: ChannelsWidget
	};

	type Item = { id: string; n: number };
	type Segment = { full: Item; left?: never; right?: never } | { full?: never; left: Item[]; right: Item[] };
	const segments = $derived.by((): Segment[] => {
		const out: Segment[] = [];
		let run: { left: Item[]; right: Item[] } | null = null;
		let n = 0;
		let k = 0;
		for (const e of overview.entries) {
			if (!e.visible || !COMPONENTS[e.id]) continue;
			const item = { id: e.id, n: n++ };
			if (overview.meta(e.id)?.size === 'full') {
				out.push({ full: item });
				run = null;
				continue;
			}
			if (!run) {
				run = { left: [], right: [] };
				out.push(run);
				k = 0;
			}
			(k++ % 2 ? run.right : run.left).push(item);
		}
		return out;
	});
</script>

{#snippet card(id: string)}
	{@const Comp = COMPONENTS[id]}
	<Comp {base} {nodes} {stats} />
{/snippet}

{#if overview.visibleIds.length === 0}
	<div class="panel text-fg-faint mt-6 px-5 py-10 text-center text-sm">
		No cards shown. Use <span class="text-fg-dim">Customize</span> to add some.
	</div>
{:else if theme.isApple}
	<!-- Cupertino: a home-screen widget grid. Small is a square, medium a wide
	     strip, large a tall card; dense flow lets smalls fill the gaps the way
	     iOS packs a home screen. 2 columns on a phone, 4 on a desktop, where a
	     small takes the medium height instead (a 400px square would tower). -->
	<div data-widget-grid class="mt-6 grid grid-cols-2 gap-4 [grid-auto-flow:row_dense] lg:grid-cols-4">
		{#each overview.visibleIds as id (id)}
			{#if COMPONENTS[id]}
				{@const shape = overview.shapeOf(id)}
				<div
					class={shape === 'small'
						? 'col-span-1 aspect-square lg:aspect-auto lg:h-[176px]'
						: shape === 'medium'
							? 'col-span-2 h-[176px]'
							: overview.meta(id)?.size === 'full'
								? 'col-span-2 h-[372px] lg:col-span-4'
								: 'col-span-2 h-[372px]'}
				>
					<WidgetSlot {shape}>{@render card(id)}</WidgetSlot>
				</div>
			{/if}
		{/each}
	</div>
{:else}
	<!-- Masonry on desktop: each run of half-width cards is split into a left and
	     a right column that stack independently, so a short card (an empty
	     Favourites) neither stretches nor leaves a hole beside a tall one.
	     Full-width cards break the run. Below lg the column wrappers are
	     display:contents and `order` restores the saved card order. -->
	<div class="mt-6 flex flex-col gap-5 lg:grid lg:grid-cols-2 lg:items-start">
		{#each segments as seg, si (si)}
			{#if seg.full}
				<div class="max-lg:[order:var(--o)] lg:col-span-2" style="--o:{seg.full.n}">
					{@render card(seg.full.id)}
				</div>
			{:else}
				{#each [seg.left, seg.right] as col, ci (ci)}
					<div class="contents lg:flex lg:flex-col lg:gap-5">
						{#each col as c (c.id)}
							<div class="max-lg:[order:var(--o)]" style="--o:{c.n}">{@render card(c.id)}</div>
						{/each}
					</div>
				{/each}
			{/if}
		{/each}
	</div>
{/if}
