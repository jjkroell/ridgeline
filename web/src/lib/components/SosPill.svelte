<!--
  A minimized #sos card: a pill beside the channel name that reopens it.
  kind="info" for the channel description, kind="monitors" for the monitor
  list (it shows the count).
-->
<script lang="ts">
	import { sos } from '$lib/sos.svelte';

	let { kind }: { kind: 'info' | 'monitors' } = $props();

	function open() {
		if (kind === 'info') sos.setInfoMinimized(false);
		else sos.setMonitorsMinimized(false);
	}
</script>

<button
	data-sos-pill
	onclick={open}
	aria-label={kind === 'info' ? 'Show the #sos channel info' : `Show who monitors #sos — ${sos.monitors.length}`}
	title={kind === 'info' ? 'About #sos' : 'Who monitors #sos'}
	class="text-coral inline-flex shrink-0 items-center gap-1.5 rounded-[var(--radius)] px-2 py-0.5 font-mono text-[0.68rem] tracking-wide whitespace-nowrap transition-[filter] hover:brightness-125"
	style="background:color-mix(in srgb, var(--color-coral) 12%, transparent); border:1px solid color-mix(in srgb, var(--color-coral) 35%, transparent)"
>
	{#if kind === 'info'}
		<span class="bg-coral h-1.5 w-1.5 rounded-full" aria-hidden="true"></span>
		<span data-caps class="uppercase">Info</span>
	{:else}
		<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="8" r="3.2" /><path d="M3 20a6 6 0 0 1 12 0" /><path d="M16 4.5a3.2 3.2 0 0 1 0 6.3M18 20a6 6 0 0 0-3-5.2" /></svg>
		<span class="tnum">{sos.loaded ? sos.monitors.length : '…'}</span>
		<span data-caps class="uppercase">monitoring</span>
	{/if}
</button>
