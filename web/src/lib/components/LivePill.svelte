<script lang="ts">
	// Live map status capsule. At rest it shows how busy the mesh is right
	// now (transmissions in the last minute) and who was heard last; tap it
	// and it opens out, like the Dynamic Island, into the last quarter hour
	// minute by minute and the latest few transmissions. Counts are distinct
	// packets, not observations: one advert heard by six observers is one.
	import { onMount } from 'svelte';
	import { live } from '$lib/live.svelte';
	import { shortKey, agoFrom } from '$lib/format';
	import PayloadTag from './PayloadTag.svelte';
	import Tooltip from './Tooltip.svelte';

	let {
		detail = '',
		onnode,
		bar = false
	}: {
		/** Extra line in the open capsule, e.g. "312 nodes on the map". */
		detail?: string;
		onnode?: (pubkey: string) => void;
		/** Sits above the map (desktop) rather than on it: accent-tinted, with
		 *  a hover tooltip saying what the numbers are. */
		bar?: boolean;
	} = $props();

	const TIP =
		'Packets heard across the mesh in the last minute (each transmission counts once, however many observers heard it), and the last node that named itself. Click for the past 15 minutes and the latest packets.';

	const MINUTES = 15;
	let now = $state(Date.now());
	let open = $state(false);
	let pillEl = $state<HTMLElement>();
	// The bar under the pointer (or finger, dragged along them): its minute
	// and count replace the axis line below.
	let hover = $state<number | null>(null);
	function pick(e: PointerEvent) {
		const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
		hover = Math.min(MINUTES - 1, Math.max(0, Math.floor(((e.clientX - r.left) / r.width) * MINUTES)));
	}
	const hoverText = $derived.by(() => {
		if (hover == null) return '';
		const n = stats.bins[hover];
		const at = new Date((Math.floor(now / 60_000) - (MINUTES - 1 - hover)) * 60_000);
		const hm = at.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
		const what = `${n} ${n === 1 ? 'packet' : 'packets'}`;
		return hover === MINUTES - 1 ? `${hm} · ${what} so far` : `${hm} · ${what}`;
	});
	// Open, it closes on Escape, a tap on its top line, or a tap anywhere else.
	function outside(e: PointerEvent) {
		if (open && pillEl && !pillEl.contains(e.target as Node)) open = false;
	}
	onMount(() => {
		const t = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(t);
	});

	// Bars: distinct packets per clock minute (live.perMinute), oldest first,
	// so a finished minute never changes; only the last, current one grows.
	// The pill's rate is a rolling last 60 s, and the latest transmissions
	// come from one pass over the newest-first buffer.
	const stats = $derived.by(() => {
		const cur = Math.floor(now / 60_000);
		const bins = Array.from({ length: MINUTES }, (_, i) => live.perMinute[cur - MINUTES + 1 + i] ?? 0);
		const seen = new Set<string>();
		const latest: typeof live.events = [];
		let named: (typeof live.events)[number] | undefined;
		let rate = 0;
		for (const e of live.events) {
			const age = now - +new Date(e.receivedAt);
			if (age >= MINUTES * 60_000) break;
			if (seen.has(e.messageHash)) continue;
			seen.add(e.messageHash);
			if (age < 60_000) rate++;
			if (latest.length < 4) latest.push(e);
			named ??= e.node ? e : undefined;
		}
		// Scaled to the finished minutes, so the growing current one can't
		// rescale the rest (it tops out at full height).
		return { bins, latest, named, rate, peak: Math.max(1, ...bins.slice(0, -1)) };
	});
	const rate = $derived(stats.rate);
	// The closed pill names the last node heard by name; most packets
	// (acks, responses, paths) don't say who sent them.
	const last = $derived(stats.named);
	const who = (e: (typeof live.events)[number]) =>
		e.node ? e.node.name || shortKey(e.node.publicKey) : e.sender ? e.sender : e.observerName ? `heard by ${e.observerName}` : 'Unknown sender';
</script>

<svelte:window onkeydown={(e) => open && e.key === 'Escape' && (open = false)} onpointerdown={outside} />

{#snippet pillButton()}
	<button
		class="flex w-full items-center gap-2 px-3 py-1.5 text-left whitespace-nowrap"
		aria-expanded={open}
		aria-label={open ? 'Close live activity' : `Live activity: ${rate} packets in the last minute${last ? `, last named node ${who(last)}` : ''}`}
		onclick={() => (open = !open)}
	>
		{#if live.connected}<span class="live-dot"></span>{:else}<span class="bg-coral/80 h-2 w-2 rounded-full"></span>{/if}
		<span class="tnum font-600">{rate}<span class="opacity-60">/min</span></span>
		{#if last && !open}
			<span class="max-w-[9rem] truncate opacity-70">{who(last)}</span>
		{/if}
		{#if open}
			<span class="ml-auto pl-3 opacity-60">{live.connected ? 'Live' : 'Reconnecting…'}</span>
			<span class="-mr-1 flex h-5 w-5 items-center justify-center rounded-full bg-current/15" title="Close">
				<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg>
			</span>
		{/if}
	</button>
{/snippet}

<div bind:this={pillEl} data-live-pill data-bar={bar ? '' : undefined} class="pointer-events-auto text-[0.72rem]" class:open>
	{#if bar && !open}
		<Tooltip text={TIP} below class="w-full">{@render pillButton()}</Tooltip>
	{:else}
		{@render pillButton()}
	{/if}

	{#if open}
		<div class="px-3 pb-3">
			<div class="flex items-baseline gap-2">
				<span class="tnum text-[1.9rem] leading-none font-700">{rate}</span>
				<span class="opacity-60">packets in the last minute</span>
			</div>
			<!-- Bars: one per minute, oldest on the left. -->
			<div
				class="mt-2.5 flex h-9 touch-pan-y items-end gap-[3px]"
				aria-label="Packets per minute, last {MINUTES} minutes"
				role="img"
				onpointermove={pick}
				onpointerdown={pick}
				onpointerleave={() => (hover = null)}
				onpointercancel={() => (hover = null)}
			>
				{#each stats.bins as b, i (i)}
					<span
						class="flex-1 rounded-[2px] transition-opacity {i === MINUTES - 1 ? 'bg-signal' : 'bg-current'} {hover == null
							? i === MINUTES - 1 ? '' : 'opacity-35'
							: i === hover ? 'opacity-100' : 'opacity-25'}"
						style="height:{Math.max(2, Math.min(100, (b / stats.peak) * 100))}%"
					></span>
				{/each}
			</div>
			<div class="mt-1 flex justify-between text-[0.6rem] {hover == null ? 'opacity-50' : ''}">
				{#if hover != null}<span class="tnum mx-auto font-600">{hoverText}</span>
				{:else}<span>{MINUTES} min ago</span><span>now</span>{/if}
			</div>

			{#if stats.latest.length}
				<ul class="mt-2.5 flex flex-col gap-0.5">
					{#each stats.latest as e (e.messageHash)}
						<li>
							<button
								class="flex w-full items-center gap-2 rounded-lg px-1 py-1 text-left hover:bg-white/10 disabled:cursor-default"
								disabled={!e.node || !onnode}
								onclick={() => e.node && onnode?.(e.node.publicKey)}
							>
								<PayloadTag type={e.payloadType} tip={false} />
								<span class="min-w-0 flex-1 truncate {e.node ? '' : 'opacity-60'}">{who(e)}</span>
								<span class="tnum shrink-0 opacity-50">{agoFrom(e.receivedAt, now)}</span>
							</button>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-2 opacity-60">Nothing heard in the last {MINUTES} minutes.</p>
			{/if}
			{#if detail}<p class="mt-2 opacity-50">{detail}</p>{/if}
		</div>
	{/if}
</div>

<style>
	[data-live-pill] {
		width: max-content;
		max-width: calc(100vw - 2rem);
		border-radius: 999px;
		color: var(--color-fg);
		/* On the map (the phone live map): frosted glass, the map showing
		   through, open or closed. The desktop chip ([data-bar]) is solid. */
		background: color-mix(in srgb, var(--color-ink-2) 55%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-line) 60%, transparent);
		box-shadow: 0 4px 18px rgba(0, 0, 0, 0.18);
		/* Unprefixed only: the build adds -webkit- for Safari, and writing both
		   made it drop this one, leaving Chrome with no blur at all. */
		backdrop-filter: blur(18px) saturate(1.6);
		transition:
			border-radius 0.35s cubic-bezier(0.32, 0.72, 0, 1),
			width 0.35s cubic-bezier(0.32, 0.72, 0, 1);
		overflow: hidden;
	}
	[data-live-pill].open {
		width: min(19rem, calc(100vw - 2rem));
		border-radius: 26px;
		animation: island 0.4s cubic-bezier(0.32, 0.72, 0, 1);
	}
	/* Cupertino: the island is dark in both light and dark, as on iPhone —
	   smoked glass here, so the map still shows through. */
	:global(html:is([data-theme='apple'], [data-theme='apple-dark'])) [data-live-pill] {
		color: #f5f5f7;
		background: rgba(0, 0, 0, 0.5);
		border-color: rgba(255, 255, 255, 0.08);
		box-shadow: 0 8px 30px rgba(0, 0, 0, 0.3);
	}
	/* Reduce transparency: no blur, so no see-through (text over map). */
	:global(html[data-solid]) [data-live-pill]:not([data-bar]) {
		background: var(--color-ink-2);
	}
	:global(html:is([data-theme='apple'], [data-theme='apple-dark'])[data-solid]) [data-live-pill]:not([data-bar]) {
		background: #000;
	}
	@media (prefers-reduced-transparency: reduce) {
		[data-live-pill]:not([data-bar]) {
			background: var(--color-ink-2);
		}
		:global(html:is([data-theme='apple'], [data-theme='apple-dark'])) [data-live-pill]:not([data-bar]) {
			background: #000;
		}
	}
	/* Above the map, on the page: tinted with the accent so it reads as a
	   status chip of its own, not a stray map control. */
	[data-live-pill][data-bar],
	:global(html:is([data-theme='apple'], [data-theme='apple-dark'])) [data-live-pill][data-bar] {
		color: var(--color-fg);
		background: color-mix(in srgb, var(--color-signal) 12%, var(--color-panel));
		border: 1px solid color-mix(in srgb, var(--color-signal) 40%, transparent);
		box-shadow: none;
	}
	[data-live-pill][data-bar] button > .tnum {
		color: var(--color-signal);
	}
	/* Open, it drops down over the map: lift it off the page. */
	[data-live-pill][data-bar].open {
		box-shadow: 0 8px 30px rgba(0, 0, 0, 0.25);
	}
	@keyframes island {
		from {
			transform: scale(0.94);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		[data-live-pill],
		[data-live-pill].open {
			transition: none;
			animation: none;
		}
	}
</style>
