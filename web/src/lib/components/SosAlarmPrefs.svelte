<!--
  This browser's #sos alarm switches: a sound and a system notification when an
  emergency word is heard on #sos (sos-alarm.svelte.ts). In the #sos info card
  and the phone Settings page.
-->
<script lang="ts">
	import { sosAlarm } from '$lib/sos-alarm.svelte';

	let { compact = false }: { compact?: boolean } = $props();
	const unsupported = $derived(sosAlarm.permission === 'unsupported');
	const denied = $derived(sosAlarm.permission === 'denied');
</script>

<div class="space-y-2.5" data-sos-alarm>
	<label class="flex cursor-pointer items-start gap-3" for="sos-sound">
		<input
			id="sos-sound"
			type="checkbox"
			checked={sosAlarm.sound}
			onchange={(e) => sosAlarm.setSound(e.currentTarget.checked)}
			class="accent-coral mt-0.5 h-4 w-4 shrink-0"
		/>
		<span class="min-w-0">
			<span class="text-fg block text-sm font-600">Play a sound</span>
			{#if !compact}
				<span class="text-fg-faint mt-0.5 block text-xs leading-relaxed">
					A short bell while Ridgeline is open, even in a background tab.
				</span>
			{/if}
			{#if sosAlarm.sound && sosAlarm.blocked}
				<span class="text-amber mt-0.5 block text-xs">Click or tap anywhere on the page to allow sound.</span>
			{/if}
		</span>
	</label>
	<label class="flex items-start gap-3 {unsupported ? 'opacity-60' : 'cursor-pointer'}" for="sos-notify">
		<input
			id="sos-notify"
			type="checkbox"
			checked={sosAlarm.notify}
			disabled={unsupported}
			onchange={(e) => {
				const box = e.currentTarget;
				sosAlarm.setNotify(box.checked).then(() => (box.checked = sosAlarm.notify));
			}}
			class="accent-coral mt-0.5 h-4 w-4 shrink-0"
		/>
		<span class="min-w-0">
			<span class="text-fg block text-sm font-600">Show a notification</span>
			{#if unsupported}
				<span class="text-fg-faint mt-0.5 block text-xs leading-relaxed">
					This browser can't show notifications. On an iPhone, add Ridgeline to your home screen first.
				</span>
			{:else if denied}
				<span class="text-amber mt-0.5 block text-xs leading-relaxed">
					Notifications are blocked for this site. Allow them in your browser's site settings, then try again.
				</span>
			{:else if !compact}
				<span class="text-fg-faint mt-0.5 block text-xs leading-relaxed">
					A system notification with the message, while Ridgeline is open. Your browser asks first.
				</span>
			{/if}
		</span>
	</label>
	<div class="flex flex-wrap items-center gap-x-3 gap-y-1 pl-7">
		<button
			type="button"
			onclick={() => sosAlarm.test()}
			disabled={!sosAlarm.sound && !sosAlarm.notify}
			class="text-coral text-xs font-600 hover:underline disabled:opacity-40 disabled:hover:no-underline"
			>Test</button
		>
		<span class="text-fg-faint text-xs">Saved in this browser only. Ridgeline has to be open (on a phone, on screen).</span>
	</div>
</div>
