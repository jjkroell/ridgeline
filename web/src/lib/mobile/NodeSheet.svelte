<script lang="ts">
	// Bottom sheet for a node tapped on a mobile map. Keeps the map in view,
	// the way Apple Maps does, instead of navigating away. It rests at two
	// heights: a short peek with the essentials, and full height with the
	// rest of the node's numbers. Drag the top of the sheet between them (a
	// flick counts), tap the grabber to switch, drag down from the peek to
	// dismiss. "Details" opens the full node page.
	import type { Node } from '$lib/api';
	import { ago, shortKey, nodeStatus, roleColor, roleLabel, lastHeard, fmtCoord, fmtNum, fmtRadio, fmtClockDrift } from '$lib/format';
	import { favorites } from '$lib/favorites.svelte';
	import { toggleStar } from './context-menu.svelte';

	let { node, onclose }: { node: Node; onclose: () => void } = $props();

	const st = $derived(nodeStatus(node));
	const heard = $derived(lastHeard(node));

	const more = $derived(
		[
			{ k: 'First heard', v: ago(node.firstSeen) },
			{ k: 'Last advert', v: node.lastAdvert ? ago(node.lastAdvert) : '—' },
			{ k: 'Adverts', v: fmtNum(node.advertTxCount) },
			{ k: 'Path hash', v: node.hashSize ? `${node.hashSize} byte${node.hashSize > 1 ? 's' : ''}` : '—' },
			{ k: 'Last relayed', v: node.lastRelayed ? ago(node.lastRelayed) : '—' },
			{ k: 'Relayed, last hour', v: node.relayCount1h != null ? fmtNum(node.relayCount1h) : '—' },
			{ k: 'Clock', v: node.clockDriftSec != null ? fmtClockDrift(node.clockDriftSec) : '—' },
			{ k: 'Radio', v: fmtRadio(node.viaBridgeRadio || node.radio), wide: true },
			node.viaBridge ? { k: 'Reached through', v: node.viaBridgeName || shortKey(node.viaBridge), wide: true } : null
		]
			.filter((r) => r != null)
			// An odd number of half-width cells would leave a hole: widen the last.
			.map((r, i, all) => {
				const half = all.filter((x) => !x.wide);
				return half.length % 2 && r === half.at(-1) ? { ...r, wide: true } : r;
			})
	);

	// Heights: `peekH` is measured from the sheet at rest (essentials only);
	// `fullH` is everything the map area allows. `h` is null until measured,
	// then always explicit so changes can animate.
	let sheetEl = $state<HTMLElement>();
	let peekH = 0;
	let h = $state<number | null>(null);
	let detent = $state<'peek' | 'full'>('peek');
	let dragging = $state(false);
	const fullH = () => (sheetEl?.parentElement?.clientHeight ?? 600) - 16;

	$effect(() => {
		if (sheetEl && h == null) {
			peekH = sheetEl.offsetHeight;
			h = peekH;
		}
	});

	function snap(to: 'peek' | 'full') {
		detent = to;
		h = to === 'full' ? fullH() : peekH;
	}

	let y0 = 0;
	let h0 = 0;
	let lastY = 0;
	let lastT = 0;
	let v = 0;
	let moved = false;
	function down(e: PointerEvent) {
		if ((e.target as Element).closest('button:not([data-grabber]), a')) return;
		y0 = lastY = e.clientY;
		lastT = e.timeStamp;
		h0 = h ?? peekH;
		v = 0;
		moved = false;
		dragging = true;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function move(e: PointerEvent) {
		if (!dragging) return;
		const dy = e.clientY - y0;
		if (Math.abs(dy) > 4) moved = true;
		const max = fullH();
		let next = h0 - dy;
		// Rubber band past full height, as iOS does.
		if (next > max) next = max + (next - max) * 0.25;
		h = Math.max(0, next);
		if (e.timeStamp > lastT) v = (e.clientY - lastY) / (e.timeStamp - lastT); // px/ms, + = down
		lastY = e.clientY;
		lastT = e.timeStamp;
	}
	function up() {
		if (!dragging) return;
		dragging = false;
		if (!moved) {
			h = h0;
			return;
		}
		// Where the sheet would coast to, then the nearest resting height.
		const coast = (h ?? 0) - v * 160;
		const max = fullH();
		if (coast < peekH * 0.55) {
			h = 0;
			setTimeout(onclose, 200);
		} else snap(coast > (peekH + max) / 2 ? 'full' : 'peek');
	}
	function key(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose();
	}
</script>

<svelte:window onkeydown={key} onresize={() => h != null && snap(detent)} />

<div
	bind:this={sheetEl}
	class="node-sheet bg-panel absolute inset-x-2 bottom-2 z-30 flex flex-col overflow-hidden rounded-[22px] px-4 pb-4"
	style:height={h == null ? undefined : `${h}px`}
	style:transition={dragging ? 'none' : 'height 0.35s cubic-bezier(0.32, 0.72, 0, 1)'}
	role="dialog"
	aria-label={node.name || 'Node'}
>
	<!-- The sheet's top (grabber + title row) is the drag handle. -->
	<div
		class="shrink-0 touch-none pt-1"
		onpointerdown={down}
		onpointermove={move}
		onpointerup={up}
		onpointercancel={up}
		role="presentation"
	>
	<button
		data-grabber
		class="mx-auto flex h-5 w-16 cursor-grab items-center justify-center"
		aria-label={detent === 'full' ? 'Show less' : 'Show more'}
		aria-expanded={detent === 'full'}
		onclick={() => !moved && snap(detent === 'full' ? 'peek' : 'full')}
	>
		<span class="bg-fg-faint/40 h-1.5 w-9 rounded-full"></span>
	</button>

<div class="flex items-start gap-3">
		<span class="mt-1.5 h-3 w-3 shrink-0 rounded-full" style="background:{roleColor(node.role)}"></span>
		<div class="min-w-0 flex-1">
			<div class="font-display text-fg truncate text-[1.25rem] leading-tight font-700 tracking-tight">
				{node.name || shortKey(node.publicKey)}
			</div>
			<div class="text-fg-dim mt-0.5 text-[0.8rem]">
				<span style="color:{roleColor(node.role)}">{roleLabel(node.role)}</span>
				· <span style="color:{st.color}">{st.label}</span> · {ago(heard)}
			</div>
		</div>
		<button
			onclick={() => toggleStar(node.publicKey)}
			class="bg-ink-2 grid h-8 w-8 shrink-0 place-items-center rounded-full"
			aria-label="Toggle favorite"
		>
			<svg viewBox="0 0 24 24" class="h-[18px] w-[18px] {favorites.has(node.publicKey) ? 'text-amber' : 'text-fg-dim'}" fill={favorites.has(node.publicKey) ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.6"><path d="M12 2.5l2.9 5.9 6.5.95-4.7 4.6 1.1 6.45L12 17.9l-5.8 3.05 1.1-6.45-4.7-4.6 6.5-.95z" /></svg>
		</button>
		<button onclick={onclose} class="bg-ink-2 text-fg-dim grid h-8 w-8 shrink-0 place-items-center rounded-full" aria-label="Close">
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18" /></svg>
		</button>
	</div>

	</div>

	<div class="bg-ink-2 mt-3 grid shrink-0 grid-cols-2 gap-px overflow-hidden rounded-xl text-[0.8rem]">
		<div class="bg-panel-2 px-3 py-2">
			<div class="text-fg-faint text-[0.7rem]">Key</div>
			<div class="text-fg font-mono">{shortKey(node.publicKey, 6, 4)}</div>
		</div>
		<div class="bg-panel-2 px-3 py-2">
			<div class="text-fg-faint text-[0.7rem]">Location</div>
			<div class="text-fg font-mono truncate">{fmtCoord(node.latitude, node.longitude)}</div>
		</div>
	</div>

	<a
		href="/m/nodes/{node.publicKey}"
		class="bg-signal mt-3 block shrink-0 rounded-xl py-3 text-center text-[0.95rem] font-600" style="color:var(--on-signal, var(--color-ink))"
	>
		Details
	</a>

	<!-- Revealed as the sheet grows; scrolls at full height. Hidden from
	     assistive tech while the sheet only peeks. -->
	{#if h != null}
		<div class="mt-4 min-h-0 flex-1 overflow-y-auto" aria-hidden={detent !== 'full' && !dragging} inert={detent !== 'full'}>
			<div class="text-fg-dim mb-2 px-1 text-[0.75rem] font-600">About this node</div>
			<dl class="bg-ink-2 grid grid-cols-2 gap-px overflow-hidden rounded-xl text-[0.8rem]">
				{#each more as r (r.k)}
					<div class="bg-panel-2 px-3 py-2 {r.wide ? 'col-span-2' : ''}">
						<dt class="text-fg-faint text-[0.7rem]">{r.k}</dt>
						<dd class="text-fg truncate font-mono">{r.v}</dd>
					</div>
				{/each}
			</dl>
		</div>
	{/if}
</div>

<style>
	.node-sheet {
		border: 1px solid var(--hairline, var(--color-line));
		box-shadow: var(--sh-overlay, 0 -8px 40px rgba(0, 0, 0, 0.45));
		animation: sheet-in 0.35s cubic-bezier(0.32, 0.72, 0, 1);
	}
	@keyframes sheet-in {
		from {
			transform: translateY(110%);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.node-sheet {
			animation: none;
		}
	}
</style>
