<script lang="ts">
	// Every preference the phone app keeps, in one place, laid out like iOS
	// Settings: grouped lists with a short caption under each group. All of it
	// lives in this browser (localStorage) except the account row.
	import Seo from '$lib/components/Seo.svelte';
	import ThemePicker from '$lib/components/ThemePicker.svelte';
	import MeshRadio from '$lib/components/MeshRadio.svelte';
	import { BASEMAPS } from '$lib/map-basemap';
	import { basemap } from '$lib/basemap.svelte';
	import { timeMode } from '$lib/time-mode.svelte';
	import { favorites } from '$lib/favorites.svelte';
	import { auth } from '$lib/auth.svelte';
	import { env } from '$lib/env.svelte';

	const maps = BASEMAPS.filter((b) => !b.id.startsWith('local'));
	const times = [
		{ id: 'relative', label: 'Time ago', eg: '2m' },
		{ id: 'clock', label: 'Clock time', eg: '16:47:12' }
	] as const;
</script>

<Seo title="Settings" description="Theme, maps and display preferences." path="/m/settings" />

{#snippet check()}
	<svg viewBox="0 0 24 24" class="text-signal h-5 w-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
{/snippet}

<div class="px-4 py-4">
	<h2 data-caps class="font-display text-fg-dim mb-2 px-1 text-xs font-700 tracking-wide uppercase">Appearance</h2>
	<div class="border-line/60 bg-panel rounded-2xl border px-4 py-4">
		<ThemePicker compact />
	</div>
	<p class="text-fg-faint mt-2 mb-6 px-1 text-xs">Auto follows your phone’s light and dark setting.</p>

	<h2 data-caps class="font-display text-fg-dim mb-2 px-1 text-xs font-700 tracking-wide uppercase">Map style</h2>
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border" role="radiogroup" aria-label="Map style">
		{#each maps as b (b.id)}
			<button role="radio" aria-checked={basemap.id === b.id} onclick={() => basemap.set(b.id)} class="active:bg-line/40 flex w-full items-center gap-3 px-4 py-3 text-left">
				<span class="min-w-0 flex-1">
					<span class="text-fg block text-sm">{b.label}</span>
					<span class="text-fg-faint block text-xs">{b.desc}</span>
				</span>
				{#if basemap.id === b.id}{@render check()}{/if}
			</button>
		{/each}
	</div>
	<p class="text-fg-faint mt-2 mb-6 px-1 text-xs">Used on every map. A shared map link can show a different style without changing this.</p>

	<h2 data-caps class="font-display text-fg-dim mb-2 px-1 text-xs font-700 tracking-wide uppercase">Packet times</h2>
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border" role="radiogroup" aria-label="Packet times">
		{#each times as t (t.id)}
			<button role="radio" aria-checked={timeMode.mode === t.id} onclick={() => timeMode.set(t.id)} class="active:bg-line/40 flex w-full items-center gap-3 px-4 py-3 text-left">
				<span class="text-fg flex-1 text-sm">{t.label}</span>
				<span class="text-fg-faint font-mono text-xs">{t.eg}</span>
				<span class="w-5">{#if timeMode.mode === t.id}{@render check()}{/if}</span>
			</button>
		{/each}
	</div>
	<p class="text-fg-faint mt-2 mb-6 px-1 text-xs">How the live feed labels when each packet arrived.</p>

	<h2 data-caps class="font-display text-fg-dim mb-2 px-1 text-xs font-700 tracking-wide uppercase">You</h2>
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border">
		<a href="/m/account" class="active:bg-line/40 flex items-center gap-3 px-4 py-3">
			<span class="text-fg flex-1 text-sm">Account</span>
			<span class="text-fg-faint truncate text-sm">{auth.user ? (auth.user.displayName || 'Signed in') : 'Not signed in'}</span>
		</a>
		<a href="/m/nodes" class="active:bg-line/40 flex items-center gap-3 px-4 py-3">
			<span class="text-fg flex-1 text-sm">Starred nodes</span>
			<span class="text-fg-faint text-sm tnum">{favorites.count}</span>
		</a>
	</div>
	<p class="text-fg-faint mt-2 mb-6 px-1 text-xs">Stars and display settings are saved on this device only.</p>

	<h2 data-caps class="font-display text-fg-dim mb-2 px-1 text-xs font-700 tracking-wide uppercase">About</h2>
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border">
		{#if env.meshRadio}
			<div class="px-4 py-3"><MeshRadio variant="inline" /></div>
		{/if}
		<a href="/about" class="active:bg-line/40 flex items-center gap-3 px-4 py-3"><span class="text-fg flex-1 text-sm">About Ridgeline</span></a>
		<a href="/privacy" class="active:bg-line/40 flex items-center gap-3 px-4 py-3"><span class="text-fg flex-1 text-sm">Privacy</span></a>
	</div>
</div>
