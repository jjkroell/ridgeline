<!--
  Who monitors #sos: every account that says so, under its radio's name, and
  the Yes / No switch for the viewer's own (SosMonitorToggle). Its own card
  beside the info card; minimizing it leaves a "N monitoring" pill
  (SosPill kind="monitors") by the channel name.
-->
<script lang="ts">
	import { sos } from '$lib/sos.svelte';
	import SosMonitorToggle from './SosMonitorToggle.svelte';

	let { mobile = false }: { mobile?: boolean } = $props();
</script>

<section
	data-sos-card
	aria-label="Who monitors #sos"
	class="panel relative shrink-0 overflow-hidden {mobile ? 'mb-3 px-4 py-3' : 'px-4 py-3 md:px-5'}"
	style="border-color:color-mix(in srgb, var(--color-coral) 35%, var(--color-line))"
>
	<span class="bg-coral absolute inset-y-0 left-0 w-[3px]" data-sos-rule aria-hidden="true"></span>
	<div class="mb-2 flex items-center gap-2">
		<span class="label">Monitoring</span>
		{#if sos.loaded}<span class="text-coral tnum font-mono text-xs">{sos.monitors.length}</span>{/if}
		<button
			onclick={() => sos.setCard('monitors', false)}
			aria-label="Minimize the #sos monitor list"
			title="Minimize"
			class="text-fg-faint hover:text-fg {mobile ? '-mr-1 p-1' : ''} ml-auto shrink-0 transition-colors"
		>
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 15l-6-6-6 6" /></svg>
		</button>
	</div>
	{#if !sos.loaded}
		<div class="text-fg-faint text-xs">Loading…</div>
	{:else if sos.monitors.length === 0}
		<div class="text-fg-faint text-xs">No one is listed yet.</div>
	{:else}
		<ul class="flex max-h-32 flex-wrap gap-1.5 overflow-y-auto">
			{#each sos.monitors as m (m.name)}
				<li
					data-sos-monitor
					title="Since {new Date(m.since).toLocaleDateString()}"
					class="border-line bg-panel-2/50 text-fg font-600 max-w-full truncate rounded-[var(--radius)] border px-2 py-0.5 text-xs"
				>
					{m.name}
				</li>
			{/each}
		</ul>
	{/if}
	<div class="border-line/60 mt-2.5 border-t pt-2.5">
		<SosMonitorToggle {mobile} />
	</div>
</section>
