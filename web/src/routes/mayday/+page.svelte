<script lang="ts">
	import PageHeader from '$lib/components/PageHeader.svelte';
	import SosGuide from '$lib/components/SosGuide.svelte';
	import Seo from '$lib/components/Seo.svelte';
	import { sos } from '$lib/sos.svelte';
	import { env } from '$lib/env.svelte';
	// The guide shows only while an admin has the emergency integration on;
	// off, the page (old /sos links land here) says so instead.
</script>
{#snippet off()}
	<div class="mx-auto max-w-3xl px-6 py-6 md:px-10">
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


<Seo
	title={sos.enabled ? 'The #mayday emergency channel' : 'Emergency channel'}
	description={sos.enabled
		? 'How the MeshCore #mayday channel works in coastal BC: how to join and monitor it, the trigger words, and what happens to a call for help — Ridgeline alerts, emails and the auto reply.'
		: 'Ridgeline no longer monitors #sos. In an emergency, call 911.'}
	path="/mayday"
/>

<PageHeader eyebrow="Guide" title={sos.enabled ? 'The #mayday channel' : 'Emergency channel'}>
	{#snippet children()}
		{#if sos.enabled}<a
			href="/channels?channel=mayday"
			class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
			>Open #mayday →</a
		>{/if}
	{/snippet}
</PageHeader>

{#if sos.enabled}
	<SosGuide />
{:else if env.loaded}
	{@render off()}
{/if}
