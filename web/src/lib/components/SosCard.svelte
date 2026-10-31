<!--
  The permanent #sos channel's info card: what the channel is for, 911 first,
  and the way into the #sos guide (/sos), which has the rest — joining,
  monitoring, alerts, the auto reply and the trigger words. Sits above the #sos
  messages; minimizing it leaves a Guide pill (SosPill kind="info") beside the
  channel name. The monitor names are in SosMonitors.
-->
<script lang="ts">
	import { sos } from '$lib/sos.svelte';

	let { mobile = false }: { mobile?: boolean } = $props();
	const guide = $derived(`${mobile ? '/m' : ''}/sos`);
	const links = [
		{ hash: 'join', label: 'Join & monitor' },
		{ hash: 'alerts', label: 'Alerts' },
		{ hash: 'bot', label: 'Auto reply' },
		{ hash: 'triggers', label: 'Trigger words' }
	];
</script>

<section
	data-sos-card
	aria-label="About the #sos channel"
	class="panel relative shrink-0 overflow-hidden {mobile ? 'mb-3 px-4 py-3.5' : 'px-4 py-3.5 md:px-5'}"
	style="border-color:color-mix(in srgb, var(--color-coral) 35%, var(--color-line))"
>
	<span class="bg-coral absolute inset-y-0 left-0 w-[3px]" data-sos-rule aria-hidden="true"></span>
	<div class="flex items-start gap-3">
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-2">
				<span class="bg-coral h-2 w-2 shrink-0 rounded-full" aria-hidden="true"></span>
				<span class="label text-coral!" data-caps>Emergency channel</span>
			</div>
			<p class="text-fg mt-1.5 text-sm leading-relaxed">
				<span class="font-mono">#sos</span> is the mesh's channel for people who need help. Post here if you
				or someone near you needs assistance, and say where you are. Keep it clear otherwise — no chat or
				radio checks.
			</p>
			<p class="text-fg-dim mt-1.5 text-xs leading-relaxed">
				In a life-threatening emergency call <span class="text-fg font-600">911</span> first; the mesh is a
				backup, not a replacement. Ridgeline only shows #sos — to post or keep watch, add it to your own MeshCore
				app or radio.
			</p>
			<div class="mt-2.5 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs">
				<a
					href={guide}
					class="text-coral inline-flex items-center gap-1.5 rounded-[var(--radius)] px-2 py-0.5 font-600 transition-[filter] hover:brightness-125"
					style="background:color-mix(in srgb, var(--color-coral) 12%, transparent); border:1px solid color-mix(in srgb, var(--color-coral) 35%, transparent)"
				>
					<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5z" /><path d="M4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5" /></svg>
					How #sos works
				</a>
				{#each links as l (l.hash)}
					<a href="{guide}#{l.hash}" class="text-fg-dim hover:text-coral transition-colors">{l.label}</a>
				{/each}
			</div>
		</div>
		<button
			onclick={() => sos.setCard('info', false)}
			aria-label="Minimize the #sos info"
			title="Minimize"
			class="text-fg-faint hover:text-fg {mobile ? '-mr-1 p-1' : ''} shrink-0 transition-colors"
		>
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 15l-6-6-6 6" /></svg>
		</button>
	</div>
</section>
