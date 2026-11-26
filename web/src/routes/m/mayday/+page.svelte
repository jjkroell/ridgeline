<script lang="ts">
	import SosGuide from '$lib/components/SosGuide.svelte';
	import { sos } from '$lib/sos.svelte';
	import { env } from '$lib/env.svelte';
</script>
{#snippet off()}
	<div class="px-4 py-4">
		<section class="panel relative overflow-hidden px-5 py-5 text-sm leading-relaxed" style="border-color:color-mix(in srgb, var(--color-coral) 35%, var(--color-line))">
			<span class="bg-coral absolute inset-y-0 left-0 w-[3px]" aria-hidden="true"></span>
			<h2 class="font-display text-fg mb-2 text-base font-700">Ridgeline isn’t monitoring an emergency channel</h2>
			<p class="text-fg-dim">
				Ridgeline no longer monitors <span class="font-mono">#sos</span>: it sends no alert emails or alarms for it,
				marks no callers on the maps and keeps no list of who is monitoring. #sos still works on the mesh as an
				ordinary channel, but nobody is watching it through Ridgeline.
			</p>
			<p class="text-fg mt-3">
				In an emergency, <span class="font-700">call 911 first</span> if you have any phone or cell service. The mesh
				is not an emergency service, and nothing on it reaches emergency services by itself.
			</p>
			<p class="text-fg-dim mt-3">
				Should Ridgeline monitor an emergency channel again?
				<a href="/survey" class="text-coral font-600 hover:underline">Tell us in a two-minute survey</a>.
			</p>
		</section>
	</div>
{/snippet}


<svelte:head><title>{sos.enabled ? 'The #mayday channel' : 'Emergency channel'} · Ridgeline</title></svelte:head>

{#if sos.enabled}
	<SosGuide compact />
{:else if env.loaded}
	{@render off()}
{/if}
