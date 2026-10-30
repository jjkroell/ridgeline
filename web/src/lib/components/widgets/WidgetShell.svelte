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
	// classic card below renders unchanged. Derived, so a size picked in
	// Customize applies at once (read once, the card kept its old size's
	// clipping until a reload).
	const getShape = getContext<(() => WidgetShape) | undefined>('widget-shape');
	const shape = $derived(getShape?.());

	// Neither size ever shows a row cut in half (rows are measured, since their
	// heights differ by widget and font):
	// - medium is a fixed height that hides the rows that don't fit, and the
	//   last one shown loses its divider, so nothing suggests a scroll;
	// - large scrolls, but its window stops at the last whole row and a
	//   "+N more" line below says how many are left (tap it to scroll on).
	// Content that isn't a list (a chart) fades out instead when it doesn't fit.
	const MORE_H = 34; // the "+N more" line, px
	let more = $state(0);
	let bodyEl = $state<HTMLElement>();

	function wholeRows(body: HTMLElement, mode: WidgetShape | undefined) {
		const rowsOf = () => {
			const list = body.querySelector<HTMLElement>(':scope > .divide-y');
			return list ? ([...list.children] as HTMLElement[]) : null;
		};
		// Rows still below the window (large).
		const count = () => {
			const rows = rowsOf();
			if (!rows || mode !== 'large' || !body.style.height) return (more = 0);
			// 2px slack: fractional row heights can leave the end a pixel short.
			const limit = body.getBoundingClientRect().bottom + 2;
			more = rows.filter((r) => r.getBoundingClientRect().bottom > limit).length;
		};
		const fit = () => {
			const rows = rowsOf();
			for (const r of rows ?? []) r.style.display = r.style.borderBottomWidth = '';
			body.style.height = body.style.flex = '';
			delete body.dataset.fade;
			if (mode === 'medium' || mode === 'large') {
				if (!rows) {
					if (mode === 'medium' && body.scrollHeight > body.clientHeight + 1) body.dataset.fade = '';
				} else if (mode === 'medium') {
					const limit = body.getBoundingClientRect().bottom + 0.5;
					let last: HTMLElement | null = null;
					let cut = false;
					for (const r of rows) {
						if (cut || r.getBoundingClientRect().bottom > limit) {
							cut = true;
							r.style.display = 'none';
						} else last = r;
					}
					if (last) last.style.borderBottomWidth = '0';
				} else if (body.scrollHeight > body.clientHeight + 1) {
					// Large and overflowing: end the window on a row boundary,
					// leaving room for the "+N more" line.
					const top = body.getBoundingClientRect().top - body.scrollTop;
					// (The line may already be showing, taking its share below.)
					const shown = body.nextElementSibling?.hasAttribute('data-more') ? MORE_H : 0;
					const room = body.clientHeight + shown - MORE_H;
					let h = 0;
					for (const r of rows) {
						const b = r.getBoundingClientRect().bottom - top;
						if (b > room + 0.5) break;
						h = b;
					}
					if (h > 0) {
						body.style.flex = 'none';
						body.style.height = `${Math.round(h)}px`;
					}
				}
			}
			count();
		};
		const ro = new ResizeObserver(fit);
		ro.observe(body.parentElement ?? body);
		// Rows come and go as data arrives; style writes don't retrigger this.
		const mo = new MutationObserver(fit);
		mo.observe(body, { childList: true, subtree: true });
		body.addEventListener('scroll', count, { passive: true });
		fit();
		return {
			update(next: WidgetShape | undefined) {
				mode = next;
				fit();
			},
			destroy() {
				ro.disconnect();
				mo.disconnect();
				body.removeEventListener('scroll', count);
			}
		};
	}

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
			<div bind:this={bodyEl} data-widget-body use:wholeRows={shape} class="mt-2 min-h-0 flex-1 {shape === 'large' ? 'overflow-y-auto' : 'overflow-hidden'}">
				{@render children()}
			</div>
			{#if shape === 'large' && more > 0}
				<button
					type="button"
					data-more
					onclick={() => bodyEl?.scrollBy({ top: bodyEl.clientHeight, behavior: 'smooth' })}
					class="text-signal border-line/50 flex shrink-0 items-center justify-center gap-1 border-t text-xs font-600"
					style="height:{MORE_H}px"
				>
					+{more} more
					<svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
				</button>
			{/if}
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
	/* Non-list content that doesn't fit a medium widget (wholeRows). */
	[data-widget-body]:global([data-fade]) {
		-webkit-mask-image: linear-gradient(to bottom, #000 65%, transparent);
		mask-image: linear-gradient(to bottom, #000 65%, transparent);
	}
</style>
