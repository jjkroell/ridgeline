<script lang="ts">
	// The value bubble for a scrubbed bar chart or heatmap: sits just above the
	// chart, centred on the mark being read (as a % across the chart), clamped so
	// it never runs off either edge.
	let { at, children }: { at: number; children: import('svelte').Snippet } = $props();
</script>

<div
	data-scrub
	class="bg-ink-2 border-line/60 text-fg pointer-events-none absolute bottom-full z-10 mb-1.5 -translate-x-1/2 rounded-lg border px-2 py-1 font-mono text-[0.68rem] whitespace-nowrap shadow-lg tnum"
	style="left:clamp(70px, {at}%, calc(100% - 70px))"
>
	{@render children()}
</div>
