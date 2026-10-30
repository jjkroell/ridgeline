<script lang="ts">
	// The A–Z strip down the right edge of an alphabetical list, as in
	// Contacts: touch a letter, or slide along the strip, to jump there.
	// Letters with no rows are dimmed and jump to the next one that has rows.
	let { letters, onjump }: { letters: Set<string>; onjump: (letter: string) => void } = $props();

	const ALL = [...'ABCDEFGHIJKLMNOPQRSTUVWXYZ', '#'];
	let stripEl = $state<HTMLElement>();
	let active = $state<string | null>(null);

	function at(y: number) {
		// The letter under the finger, clamped to the first/last letter when
		// the finger slides past either end of the column.
		const rows = [...stripEl!.querySelectorAll('button')].map((b) => b.getBoundingClientRect());
		let i = rows.findIndex((r) => y < r.bottom);
		if (i < 0) i = ALL.length - 1;
		// Nearest letter at or after this one that has rows; else the last that does.
		const hit = ALL.slice(i).find((l) => letters.has(l)) ?? ALL.slice(0, i).findLast((l) => letters.has(l));
		if (hit && hit !== active) {
			active = hit;
			navigator.vibrate?.(4);
			onjump(hit);
		}
	}
	function down(e: PointerEvent) {
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		at(e.clientY);
	}
	function move(e: PointerEvent) {
		if (active) at(e.clientY);
	}
	function up() {
		active = null;
	}
</script>

<div
	bind:this={stripEl}
	data-index-strip
	class="fixed right-0.5 z-10 flex touch-none flex-col items-center justify-center px-1 select-none"
	style="top:calc(var(--m-header-h, 0px) + 8px);bottom:calc(var(--m-nav-h, 0px) + 8px);"
	onpointerdown={down}
	onpointermove={move}
	onpointerup={up}
	onpointercancel={up}
	role="navigation"
	aria-label="Jump to letter"
>
	{#each ALL as l (l)}
		<button
			tabindex="-1"
			class="text-signal w-4 py-px text-center text-[0.62rem] leading-[1.05] font-600 {letters.has(l) ? '' : 'opacity-35'}"
			aria-label="Jump to {l === '#' ? 'other' : l}"
			onclick={() => letters.has(l) && onjump(l)}>{l}</button
		>
	{/each}
</div>

{#if active}
	<div class="bg-ink-2/90 border-line/60 text-fg pointer-events-none fixed top-1/2 right-10 z-20 grid h-14 w-14 -translate-y-1/2 place-items-center rounded-2xl border text-2xl font-700 shadow-xl backdrop-blur-xl" aria-hidden="true">
		{active}
	</div>
{/if}
