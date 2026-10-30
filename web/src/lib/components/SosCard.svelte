<!--
  The permanent #sos channel's info card: what the channel is for and how to
  join or leave its monitor list. Sits above the #sos messages; minimizing it
  leaves an Info pill (SosPill kind="info") beside the channel name. The names
  themselves are in SosMonitors.
-->
<script lang="ts">
	import { sos } from '$lib/sos.svelte';

	let { mobile = false }: { mobile?: boolean } = $props();
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
				backup, not a replacement.
			</p>
			<p class="text-fg-dim mt-1.5 text-xs leading-relaxed">
				<span class="text-fg font-600">Can you keep an eye on it?</span> Join #sos and send one — just one —
				<span class="text-fg font-mono">test</span> message. You're listed as a monitor, so everyone can see how
				many people are listening. Then <span class="text-fg font-600">leave notifications on for #sos</span> on
				your radio or app, so a real call for help reaches you.
			</p>
			<p class="text-fg-dim mt-1.5 text-xs leading-relaxed">
				Send <span class="text-fg font-mono">unsub</span> to come off the list. To keep the channel clean, test
				and unsub messages are removed from the chat here after 5 minutes — your name stays on the list. (They
				have already gone out over the radio, so other apps may still show them.)
			</p>
			<details class="group mt-1.5 text-xs">
				<summary class="text-fg-dim hover:text-fg cursor-pointer list-none select-none">
					<span class="inline-block transition-transform group-open:rotate-90" aria-hidden="true">›</span>
					Why a separate channel?
				</summary>
				<div class="text-fg-dim mt-1 space-y-1.5 pl-3 leading-relaxed">
					<p>
						<span class="text-fg font-600">Why not Public?</span> A call for help can get lost in the chatter
						there, and many people have Public notifications set to mentions only. #sos stays quiet, so every
						message on it is worth a notification.
					</p>
					<p>
						<span class="text-fg font-600">Why not #emergency?</span> #emergency is for emergencies that affect
						everyone, like an earthquake. #sos is for one person or group who needs help. Join #emergency too.
					</p>
				</div>
			</details>
		</div>
		<button
			onclick={() => sos.setInfoMinimized(true)}
			aria-label="Minimize the #sos info"
			title="Minimize"
			class="text-fg-faint hover:text-fg {mobile ? '-mr-1 p-1' : ''} shrink-0 transition-colors"
		>
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 15l-6-6-6 6" /></svg>
		</button>
	</div>
</section>
