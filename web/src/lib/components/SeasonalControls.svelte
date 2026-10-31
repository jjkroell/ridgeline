<!--
  Seasonal theme toggle + legend for the live map's "Map Control" panel
  (MapRoleFilter's children slot), beside ChimeControls. Shown only while a
  season is in force (see $lib/seasonal.svelte); drives the shared `seasonal`
  singleton, so the MapLibre and WebGL-free live maps agree.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { seasonal } from '$lib/seasonal.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	onMount(() => seasonal.load());
	const s = $derived(seasonal.season);
</script>

{#if s}
	<div class="label mt-3 mb-1">{s.name}</div>
	<Tooltip text={seasonal.on ? 'Back to plain packet dots' : s.tip} class="block w-full">
		<button
			onclick={() => seasonal.toggle()}
			aria-pressed={seasonal.on}
			class="flex w-full items-center justify-center gap-1.5 rounded-[var(--radius)] border px-2 py-1 text-[0.68rem] font-medium transition-colors {seasonal.on
				? ''
				: 'border-line text-fg-dim hover:text-fg'}"
			style={seasonal.on ? `border-color:color-mix(in srgb, ${s.accent} 60%, transparent);color:${s.accent}` : ''}
		>
			<span class="text-[0.85rem] leading-none" aria-hidden="true">{s.emoji}</span>
			<span>{seasonal.on ? `${s.label} on` : s.label}</span>
		</button>
	</Tooltip>
	{#if seasonal.on}
		<div class="border-line/60 mt-2 space-y-0.5 border-t pt-2">
			{#each s.legend as l (l.label)}
				<div class="text-fg-dim flex items-center gap-1.5 text-[0.66rem]">
					<span class="w-4 text-center text-[0.8rem] leading-none" aria-hidden="true">{l.icon}</span>{l.label}
				</div>
			{/each}
		</div>
	{/if}
{/if}
