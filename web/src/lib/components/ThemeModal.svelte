<script lang="ts">
	// The theme picker, kept out of the way until asked for: the sidebar and the
	// phone app bar each show one small button naming the current theme, and it
	// opens this. Changes apply live, so there is nothing to save.
	import Modal from './Modal.svelte';
	import ThemePicker from './ThemePicker.svelte';

	let { onclose }: { onclose: () => void } = $props();
</script>

<Modal {onclose} maxWidth="md:max-w-sm">
	<div class="border-line/70 flex items-center gap-3 border-b px-5 py-3.5">
		<h2 class="font-display text-fg text-base font-700">Appearance</h2>
		<button onclick={onclose} class="text-signal ml-auto text-sm font-600">Done</button>
	</div>
	<div class="px-5 pt-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))]">
		<ThemePicker compact />
		<p class="text-fg-faint mt-4 text-xs">Changes apply straight away and are saved on this device.</p>
	</div>
</Modal>
