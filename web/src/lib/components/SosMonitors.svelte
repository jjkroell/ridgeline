<!--
  Who monitors #sos: everyone who has sent "test" there (and not "unsub"
  since), most recent first. Its own card beside the info card; minimizing it
  leaves a "N monitoring" pill (SosPill kind="monitors") by the channel name.
-->
<script lang="ts">
	import { sos } from '$lib/sos.svelte';
	import { ago } from '$lib/format';

	let { mobile = false }: { mobile?: boolean } = $props();

	// A monitor who has not checked in for a month is shown, but dimmed — the
	// list is "who has said they listen", and that goes stale.
	const STALE_MS = 30 * 86400_000;
	const stale = (iso: string) => sos.now - +new Date(iso) > STALE_MS;
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
		<div class="text-fg-faint text-xs">No one has checked in yet. Send <span class="font-mono">test</span> on #sos to be the first.</div>
	{:else}
		<ul class="flex max-h-32 flex-wrap gap-1.5 overflow-y-auto">
			{#each sos.monitors as m (m.name)}
				<li
					data-sos-monitor
					title="{m.tests} test{m.tests === 1 ? '' : 's'} · first {new Date(m.firstTest).toLocaleDateString()} · last {new Date(m.lastTest).toLocaleString()}"
					class="border-line bg-panel-2/50 inline-flex items-baseline gap-1.5 rounded-[var(--radius)] border px-2 py-0.5 text-xs {stale(m.lastTest) ? 'opacity-55' : ''}"
				>
					<span class="text-fg font-600 max-w-[12rem] truncate">{m.name}</span>
					<span class="text-fg-faint font-mono text-[0.62rem]">{ago(m.lastTest)}</span>
				</li>
			{/each}
		</ul>
		<div class="text-fg-faint mt-2 text-[0.68rem]">
			Names are as sent on the channel. Send <span class="font-mono">unsub</span> to come off the list.
		</div>
	{/if}
</section>
