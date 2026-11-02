<!--
  Seasonal theme toggle + legend, floating at the top centre of the desktop
  live map (under Copy link) — kept out of Map Control, where its legend made
  the panel long enough to run under Recent packets. Shown only while a season
  is in force (see $lib/seasonal.svelte); drives the shared `seasonal`
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
	<div
		class="border-line bg-ink-2/85 flex max-w-full flex-col items-center rounded-[var(--radius)] border shadow-lg backdrop-blur-md"
		style={seasonal.on ? `border-color:color-mix(in srgb, ${s.accent} 45%, transparent)` : ''}
	>
		<Tooltip text={seasonal.on ? 'Back to plain packet dots' : s.tip}>
			<button
				onclick={() => seasonal.toggle()}
				aria-pressed={seasonal.on}
				class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors {seasonal.on
					? ''
					: 'text-fg-dim hover:text-fg'}"
				style={seasonal.on ? `color:${s.accent}` : ''}
			>
				<img src={seasonal.src(s.emoji)} alt="" class="h-4 w-4 {seasonal.on ? '' : 'opacity-70 grayscale'}" />
				<span>{seasonal.on ? `${s.label} on` : s.label}</span>
			</button>
		</Tooltip>
		{#if seasonal.on}
			<div class="border-line/60 flex flex-wrap justify-center gap-x-3 gap-y-1 border-t px-3 py-1.5">
				{#each s.legend as l (l.label)}
					<span class="text-fg-dim flex items-center gap-1 text-[0.66rem] whitespace-nowrap">
						<img src={seasonal.src(l.icon)} alt="" class="h-3.5 w-auto object-contain" />{l.label}
					</span>
				{/each}
			</div>
		{/if}
	</div>
{/if}
