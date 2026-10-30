<!--
  How Ridgeline can alert you about #sos: this browser's sound and notification
  switches (SosAlarmPrefs) and a pointer to the account's alert emails. Its own
  card beside the info and monitor cards; minimizing it leaves an Alerts pill
  (SosPill kind="alerts") by the channel name.
-->
<script lang="ts">
	import { sos } from '$lib/sos.svelte';
	import SosAlarmPrefs from './SosAlarmPrefs.svelte';

	let { mobile = false }: { mobile?: boolean } = $props();
</script>

<section
	data-sos-card
	aria-label="#sos alerts"
	class="panel relative shrink-0 overflow-hidden {mobile ? 'mb-3 px-4 py-3' : 'px-4 py-3 md:px-5'}"
	style="border-color:color-mix(in srgb, var(--color-coral) 35%, var(--color-line))"
>
	<span class="bg-coral absolute inset-y-0 left-0 w-[3px]" data-sos-rule aria-hidden="true"></span>
	<div class="mb-2 flex items-center gap-2">
		<span class="label">Alerts</span>
		<button
			onclick={() => sos.setCard('alerts', false)}
			aria-label="Minimize the #sos alerts"
			title="Minimize"
			class="text-fg-faint hover:text-fg {mobile ? '-mr-1 p-1' : ''} ml-auto shrink-0 transition-colors"
		>
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 15l-6-6-6 6" /></svg>
		</button>
	</div>
	<p class="text-fg-dim mb-2.5 text-xs leading-relaxed">
		When someone calls for help on #sos — sos, emergency, mayday, 911, threat, life, “need help” or “help me” —
		this browser can:
	</p>
	<SosAlarmPrefs compact={mobile} />
	<p class="text-fg-dim border-line/60 mt-2.5 border-t pt-2.5 text-xs leading-relaxed">
		<span class="text-fg font-600">While Ridgeline is closed:</span> with a Ridgeline account you can get an
		email instead — turn on #sos alerts under
		<a href="{mobile ? '/m' : ''}/account#sos-alerts" class="text-coral hover:underline">Account</a>.
	</p>
</section>
