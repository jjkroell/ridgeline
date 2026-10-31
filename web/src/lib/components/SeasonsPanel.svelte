<script lang="ts">
	// Admin → Seasonal themes: which holiday theme the live map offers everyone
	// (the Spooky / Festive packets toggle, its legend and What's new items).
	// One switch per theme, only one on at a time; turning the lit one off turns
	// themes off altogether. Until an admin chooses, each theme follows its own
	// dates (drafts never start by date). Shared by /admin and /m/admin like
	// EmailLogPanel; `compact` switches to the mobile card chrome.
	import { auth } from '$lib/auth.svelte';
	import { env } from '$lib/env.svelte';
	import { adminSeason } from '$lib/api';
	import { SEASONS, seasonal } from '$lib/seasonal.svelte';

	let { compact = false }: { compact?: boolean } = $props();

	const shell = $derived(
		compact ? 'border-line/60 bg-panel mb-3 rounded-2xl border overflow-hidden' : 'panel rise mt-6 overflow-hidden'
	);
	const pad = $derived(compact ? 'px-4' : 'px-5');

	let busy = $state(false);
	let error = $state('');

	const active = $derived(seasonal.scheduled?.id ?? '');

	async function set(v: string) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await adminSeason.set(auth.csrf, v);
			env.season = v || null;
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}

	const fmt = (iso: string) =>
		new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'America/Vancouver' });
	// `until` is exclusive; show the last day it runs.
	const range = (s: (typeof SEASONS)[number]) =>
		`${fmt(s.from)} – ${fmt(new Date(Date.parse(s.until) - 1).toISOString())}`;
</script>

<section class={shell}>
	<div class="border-line/70 border-b {pad} {compact ? 'py-3' : 'py-3.5'}">
		<h2 class="font-display text-fg font-700 tracking-wide {compact ? 'text-xs' : 'text-sm'}">SEASONAL THEMES</h2>
		<p class="text-fg-dim mt-1 text-xs">
			The holiday theme people can turn on from the live map. Only one at a time.
		</p>
	</div>
	<ul class="divide-line/60 divide-y">
		{#each SEASONS as s (s.id)}
			{@const on = active === s.id}
			<li class="flex items-center gap-3 {pad} py-3">
				<img src={seasonal.src(s.emoji)} alt="" class="h-6 w-6 shrink-0" />
				<div class="min-w-0 flex-1">
					<div class="text-fg text-sm font-600">
						{s.name} <span class="text-fg-dim font-400">· {s.label}</span>
					</div>
					<div class="text-fg-faint text-xs">
						{range(s)}{s.draft ? ' · not scheduled by date' : ''}{s.snow ? ' · with snowfall' : ''}
					</div>
				</div>
				<button
					role="switch"
					aria-checked={on}
					aria-label="{s.name} theme"
					disabled={busy}
					onclick={() => set(on ? 'none' : s.id)}
					class="relative h-6 w-11 shrink-0 rounded-full border transition-colors disabled:opacity-50 {on
						? 'border-transparent'
						: 'border-line bg-panel-2'}"
					style={on ? `background:${s.accent}` : ''}
				>
					<span
						class="absolute top-0.5 left-0.5 h-[18px] w-[18px] rounded-full bg-white shadow transition-transform"
						style="transform: translateX({on ? 20 : 0}px)"
					></span>
				</button>
			</li>
		{/each}
	</ul>
	<div class="border-line/60 text-fg-dim flex flex-wrap items-center gap-x-3 gap-y-1 border-t {pad} py-2.5 text-xs">
		{#if env.season === null}
			<span>Following the calendar: each theme turns itself on and off by its dates.</span>
		{:else}
			<span>{env.season === 'none' ? 'All themes off' : 'Set by an admin'}: the dates are ignored.</span>
			<button onclick={() => set('')} disabled={busy} class="text-signal hover:underline disabled:opacity-50"
				>Follow the calendar</button
			>
		{/if}
		{#if error}<span class="text-coral">{error}</span>{/if}
	</div>
</section>
