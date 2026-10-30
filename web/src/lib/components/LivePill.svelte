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

	let {
		detail = '',
		onnode
	}: {
		/** Extra line in the open capsule, e.g. "312 nodes on the map". */
		detail?: string;
		onnode?: (pubkey: string) => void;
	} = $props();

	const MINUTES = 15;
	let now = $state(Date.now());
	let open = $state(false);
	onMount(() => {
		const t = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(t);
	});

	// One pass over the newest-first buffer: per-minute distinct packets for
	// the last quarter hour, and the latest distinct transmissions.
	const stats = $derived.by(() => {
		const bins = Array<number>(MINUTES).fill(0);
		const seen = new Set<string>();
		const latest: typeof live.events = [];
		let named: (typeof live.events)[number] | undefined;
		for (const e of live.events) {
			const age = now - +new Date(e.receivedAt);
			if (age >= MINUTES * 60_000) break;
			if (seen.has(e.messageHash)) continue;
			seen.add(e.messageHash);
			bins[MINUTES - 1 - Math.max(0, Math.floor(age / 60_000))]++;
			if (latest.length < 4) latest.push(e);
			named ??= e.node ? e : undefined;
		}
		return { bins, latest, named, peak: Math.max(1, ...bins) };
	});
	const rate = $derived(stats.bins[MINUTES - 1]);
	// The closed pill names the last node heard by name; most packets
	// (acks, responses, paths) don't say who sent them.
	const last = $derived(stats.named);
	const who = (e: (typeof live.events)[number]) =>
		e.node ? e.node.name || shortKey(e.node.publicKey) : e.sender ? e.sender : e.observerName ? `heard by ${e.observerName}` : 'Unknown sender';
</script>

<svelte:window onkeydown={(e) => open && e.key === 'Escape' && (open = false)} />

<div data-live-pill class="pointer-events-auto text-[0.72rem]" class:open>
	<button
		class="flex w-full items-center gap-2 px-3 py-1.5 text-left whitespace-nowrap"
		aria-expanded={open}
		aria-label="Live activity: {rate} packets in the last minute{last ? `, last named node ${who(last)}` : ''}"
		onclick={() => (open = !open)}
	>
		{#if live.connected}<span class="live-dot"></span>{:else}<span class="bg-coral/80 h-2 w-2 rounded-full"></span>{/if}
		<span class="tnum font-600">{rate}<span class="opacity-60">/min</span></span>
		{#if last && !open}
			<span class="max-w-[9rem] truncate opacity-70">{who(last)}</span>
		{/if}
		{#if open}<span class="ml-auto pl-3 opacity-60">{live.connected ? 'Live' : 'Reconnecting…'}</span>{/if}
	</button>

	{#if open}
		<div class="px-3 pb-3">
			<div class="flex items-baseline gap-2">
				<span class="tnum text-[1.9rem] leading-none font-700">{rate}</span>
				<span class="opacity-60">packets in the last minute</span>
			</div>
			<!-- Bars: one per minute, oldest on the left. -->
			<div class="mt-2.5 flex h-9 items-end gap-[3px]" aria-label="Packets per minute, last {MINUTES} minutes" role="img">
				{#each stats.bins as b, i (i)}
					<span class="flex-1 rounded-[2px] {i === MINUTES - 1 ? 'bg-signal' : 'bg-current opacity-35'}" style="height:{Math.max(2, (b / stats.peak) * 100)}%"></span>
				{/each}
			</div>
			<div class="mt-1 flex justify-between text-[0.6rem] opacity-50"><span>{MINUTES} min ago</span><span>now</span></div>

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
		background: color-mix(in srgb, var(--color-ink-2) 85%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-line) 60%, transparent);
		box-shadow: 0 4px 18px rgba(0, 0, 0, 0.18);
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
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
	/* Cupertino: the island is black in both light and dark, as on iPhone. */
	:global(html:is([data-theme='apple'], [data-theme='apple-dark'])) [data-live-pill] {
		color: #f5f5f7;
		background: #000;
		border-color: transparent;
		box-shadow: 0 8px 30px rgba(0, 0, 0, 0.3);
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
