<!--
  Halloween mode toggle + legend for the live map's "Map Control" panel
  (MapRoleFilter's children slot), beside ChimeControls. Drives the shared
  `spooky` singleton, so the MapLibre and WebGL-free live maps agree.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { spooky, SPOOKY_LEGEND } from '$lib/spooky.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	onMount(() => spooky.load());
</script>

<div class="label mt-3 mb-1">Halloween</div>
<Tooltip
	text={spooky.on ? 'Back to plain packet dots' : 'Packets travel as pumpkins, bats, ghosts and witches'}
	class="block w-full"
>
	<button
		onclick={() => spooky.toggle()}
		aria-pressed={spooky.on}
		class="flex w-full items-center justify-center gap-1.5 rounded-[var(--radius)] border px-2 py-1 text-[0.68rem] font-medium transition-colors {spooky.on
			? 'border-[#ff7a1a]/60 text-[#ff8c33]'
			: 'border-line text-fg-dim hover:text-fg'}"
	>
		<span class="text-[0.85rem] leading-none" aria-hidden="true">🎃</span>
		<span>{spooky.on ? 'Spooky packets on' : 'Spooky packets'}</span>
	</button>
</Tooltip>
{#if spooky.on}
	<div class="border-line/60 mt-2 space-y-0.5 border-t pt-2">
		{#each SPOOKY_LEGEND as l (l.label)}
			<div class="text-fg-dim flex items-center gap-1.5 text-[0.66rem]">
				<span class="w-4 text-center text-[0.8rem] leading-none" aria-hidden="true">{l.icon}</span>{l.label}
			</div>
		{/each}
	</div>
{/if}
