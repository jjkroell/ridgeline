<!--
  A minimized #sos card: a pill beside the channel name that reopens it.
  kind="info" for the channel description and the way into the #sos guide
  (shown as "Guide"), kind="monitors" for the monitor
  list (it shows the count), kind="alerts" for this browser's sound and
  notification switches (a dot when one is on). Opening one closes the others.
-->
<script lang="ts">
	import { sos, type SosCardKind } from '$lib/sos.svelte';
	import { sosAlarm } from '$lib/sos-alarm.svelte';

	// compact (phone header): monitors drop the word, Alerts is just the bell.
	let { kind, compact = false }: { kind: SosCardKind; compact?: boolean } = $props();
	const alarmOn = $derived(sosAlarm.sound || sosAlarm.notify);
	const label = $derived(
		kind === 'info'
			? 'Show the #sos channel info and guide'
			: kind === 'monitors'
				? `Show who monitors #sos — ${sos.monitors.length}`
				: `#sos alerts in this browser — ${alarmOn ? 'on' : 'off'}`
	);
	const title = $derived(kind === 'info' ? 'About #sos and the guide' : kind === 'monitors' ? 'Who monitors #sos' : 'Sound and notification alerts');
</script>

<button
	data-sos-pill
	onclick={() => sos.setCard(kind, true)}
	aria-label={label}
	{title}
	class="text-coral inline-flex shrink-0 items-center gap-1.5 rounded-[var(--radius)] px-2 py-0.5 font-mono text-[0.68rem] tracking-wide whitespace-nowrap transition-[filter] hover:brightness-125"
	style="background:color-mix(in srgb, var(--color-coral) 12%, transparent); border:1px solid color-mix(in srgb, var(--color-coral) 35%, transparent)"
>
	{#if kind === 'info'}
		<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5z" /><path d="M4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5" /></svg>
		{#if !compact}<span data-caps class="uppercase">Guide</span>{/if}
	{:else if kind === 'alerts'}
		<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0" /></svg>
		{#if !compact}<span data-caps class="uppercase">Alerts</span>{/if}
		{#if alarmOn}<span class="bg-coral h-1.5 w-1.5 rounded-full" aria-hidden="true"></span>{/if}
	{:else}
		<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="8" r="3.2" /><path d="M3 20a6 6 0 0 1 12 0" /><path d="M16 4.5a3.2 3.2 0 0 1 0 6.3M18 20a6 6 0 0 0-3-5.2" /></svg>
		<span class="tnum">{sos.loaded ? sos.monitors.length : '…'}</span>
		{#if !compact}<span data-caps class="uppercase">monitoring</span>{/if}
	{/if}
</button>
