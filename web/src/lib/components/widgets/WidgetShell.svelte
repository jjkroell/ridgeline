<!--
  Shared frame for Overview dashboard widgets: a panel with an icon + title
  header (optional "view all" link) and a body. Used by every widget so the
  customizable cards read as one system on both desktop and mobile.
-->
<script lang="ts">
	import { getContext, type Snippet } from 'svelte';
	import type { WidgetShape } from '$lib/overview.svelte';

	let {
		title,
		icon,
		href,
		linkLabel = 'View →',
		color = 'dim',
		fill = false,
		stat,
		statLabel = '',
		children
	}: {
		title: string;
		icon: string;
		href?: string;
		linkLabel?: string;
		color?: 'signal' | 'amber' | 'dim';
		fill?: boolean;
		/** The one number this card is about, shown big on a Cupertino widget. */
		stat?: string | number | null;
		statLabel?: string;
		children: Snippet;
	} = $props();

	// Set only inside the Cupertino widget grid (WidgetSlot); elsewhere the
	// classic card below renders unchanged.
	const shape = getContext<(() => WidgetShape) | undefined>('widget-shape')?.();

	const colorClass = $derived({ signal: 'text-signal', amber: 'text-amber', dim: 'text-fg-dim' }[color]);
</script>

{#if shape}
	<section data-widget={shape} class="panel relative flex h-full flex-col overflow-hidden">
		<div class="flex items-center gap-1.5 px-4 pt-3.5 {colorClass === 'text-fg-dim' ? 'text-signal' : colorClass}">
			<svg viewBox="0 0 24 24" class="h-[15px] w-[15px] shrink-0" fill={fill ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d={icon} /></svg>
			<h2 class="truncate text-[0.8rem] font-600">{title}</h2>
		</div>
		{#if stat != null && stat !== ''}
			<!-- A small widget sits its number at the foot of the square, iOS-style. -->
			<div class="px-4 {shape === 'small' ? 'mt-auto pb-4' : 'pt-1'}">
				<div class="text-fg tnum text-[2.1rem] leading-none font-700 tracking-tight">{stat}</div>
				{#if statLabel}<div class="text-fg-dim mt-1 text-xs {shape === 'small' ? 'line-clamp-2' : 'truncate'}">{statLabel}</div>{/if}
			</div>
		{/if}
		{#if shape === 'small'}
			{#if href}<a {href} class="absolute inset-0" aria-label={title}></a>{/if}
		{:else}
			<div data-widget-body class="mt-2 min-h-0 flex-1 {shape === 'large' ? 'overflow-y-auto' : 'overflow-hidden'}">
				{@render children()}
			</div>
		{/if}
	</section>
{:else}
<section class="panel flex h-full flex-col overflow-hidden">
	<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
		<svg
			viewBox="0 0 24 24"
			class="h-4 w-4 shrink-0 {colorClass}"
			fill={fill ? 'currentColor' : 'none'}
			stroke="currentColor"
			stroke-width="1.6"
			stroke-linecap="round"
			stroke-linejoin="round"><path d={icon} /></svg
		>
		<h2 data-caps class="font-display text-fg truncate text-sm font-700 tracking-wide uppercase">{title}</h2>
		{#if href}
			<a {href} class="label hover:text-signal ml-auto shrink-0 transition-colors">{linkLabel}</a>
		{/if}
	</div>
	<div class="min-h-0 flex-1 overflow-y-auto" style="max-height:20rem">
		{@render children()}
	</div>
</section>
{/if}

<style>
	/* A medium widget shows what fits and fades out rather than scrolling. */
	[data-widget='medium'] [data-widget-body] {
		-webkit-mask-image: linear-gradient(to bottom, #000 65%, transparent);
		mask-image: linear-gradient(to bottom, #000 65%, transparent);
	}
</style>
