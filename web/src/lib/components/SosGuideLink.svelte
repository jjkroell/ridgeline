<!--
  A pill in the #sos channel header that opens the #sos guide (/sos): how the
  channel, alerts and auto reply work. Styled like SosPill, but it's a link
  and always shows. compact (phone header): just the icon.
-->
<script lang="ts">
	let { compact = false }: { compact?: boolean } = $props();
</script>

<a
	data-sos-pill
	href="{compact ? '/m' : ''}/sos"
	aria-label="How #sos works — the guide"
	title="How #sos works"
	class="text-coral inline-flex shrink-0 items-center gap-1.5 rounded-[var(--radius)] px-2 py-0.5 font-mono text-[0.68rem] tracking-wide whitespace-nowrap transition-[filter] hover:brightness-125"
	style="background:color-mix(in srgb, var(--color-coral) 12%, transparent); border:1px solid color-mix(in srgb, var(--color-coral) 35%, transparent)"
>
	<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5z" /><path d="M4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5" /></svg>
	{#if !compact}<span data-caps class="uppercase">Guide</span>{/if}
</a>
