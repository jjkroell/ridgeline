<!--
  Packet travel-speed slider for the live map's "Map Control" panel. Steps
  through the SPEEDS presets (¼× … 3×) on the shared `liveSpeed` singleton;
  new comets pick it up as they launch.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { liveSpeed, SPEEDS, speedLabel } from '$lib/live-speed.svelte';

	onMount(() => liveSpeed.load());
	const idx = $derived(Math.max(0, SPEEDS.indexOf(liveSpeed.value)));
</script>

<div class="mb-3">
	<div class="label mb-1 flex items-center justify-between">
		<span>Speed</span>
		<button
			onclick={() => liveSpeed.set(1)}
			title="Back to normal speed"
			class="tnum transition-colors {liveSpeed.value === 1 ? 'text-fg-faint' : 'text-signal hover:text-fg'}"
			>{speedLabel(liveSpeed.value)}</button
		>
	</div>
	<input
		type="range"
		min="0"
		max={SPEEDS.length - 1}
		step="1"
		value={idx}
		oninput={(e) => liveSpeed.set(SPEEDS[+e.currentTarget.value])}
		aria-label="Packet speed"
		aria-valuetext={speedLabel(liveSpeed.value)}
		class="accent-signal h-1 w-full cursor-pointer"
	/>
	<div class="text-fg-faint mt-0.5 flex justify-between text-[0.58rem]">
		<span>Slower</span><span>Faster</span>
	</div>
</div>
