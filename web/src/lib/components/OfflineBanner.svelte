<script lang="ts">
	// Shown while the page is running on saved data because the server can't be
	// reached (see $lib/server-clock). In-flow at the top of the desktop and
	// mobile shells, like DevBanner. It clears itself on the next live response.
	import { serverClock } from '$lib/server-clock.svelte';
	import { ago } from '$lib/format';

	const saved = $derived(serverClock.savedAt ? new Date(serverClock.savedAt).toISOString() : '');
</script>

{#if serverClock.offline}
	<div
		data-offline-banner
		class="bg-coral/12 text-coral border-coral/30 flex shrink-0 flex-wrap items-center justify-center gap-x-2 gap-y-0.5 border-b px-3 py-1 text-center text-[0.72rem] leading-tight font-700"
		role="status"
		aria-live="polite"
	>
		<span
			class="bg-coral text-ink rounded px-1.5 py-0.5 text-[0.6rem] font-900 tracking-[0.15em] uppercase"
			>Offline</span
		>
		<span
			>Can't reach Ridgeline — showing {saved ? `data saved ${ago(saved)} ago` : 'the last data loaded'}.
			Times and Reporting / Silent are out of date until it reconnects.</span
		>
	</div>
{/if}
