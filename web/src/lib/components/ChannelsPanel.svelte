<script lang="ts">
	// Admin → Discovered channels: take a channel off the public discovery list,
	// or put it back. Removed names stay removed even if traffic is heard again.
	// Shared by /admin and /m/admin; `compact` switches to the mobile card chrome.
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import { adminChannels, channelSourceColor, type DiscoveredChannel } from '$lib/api';

	let { compact = false }: { compact?: boolean } = $props();

	const shell = $derived(
		compact ? 'border-line/60 bg-panel mb-3 rounded-2xl border overflow-hidden' : 'panel rise mt-6 overflow-hidden'
	);
	const pad = $derived(compact ? 'px-4' : 'px-5');

	let channels = $state<DiscoveredChannel[]>([]);
	let removed = $state<DiscoveredChannel[]>([]);
	let busy = $state(false);
	let error = $state('');

	const byName = (a: DiscoveredChannel, b: DiscoveredChannel) => a.name.localeCompare(b.name);

	async function load() {
		const d = await adminChannels.list();
		channels = [...d.channels].sort(byName);
		removed = [...d.removed].sort(byName);
	}

	async function change(action: (csrf: string, name: string) => Promise<unknown>, name: string) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await action(auth.csrf, name);
			await load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		load().catch((e) => (error = String((e as Error).message ?? e)));
	});

	const btn = 'border-line text-fg-dim hover:border-signal/50 hover:text-signal shrink-0 rounded-[var(--radius)] border px-2 py-1 text-xs transition-colors disabled:opacity-50';
</script>

<section class={shell}>
	<div class="border-line/70 border-b {pad} {compact ? 'py-3' : 'py-3.5'}">
		<h2 class="font-display text-fg font-700 tracking-wide {compact ? 'text-xs' : 'text-sm'}">DISCOVERED CHANNELS · {channels.length}</h2>
		<p class="text-fg-dim mt-1 text-xs">
			Removing takes a channel off the public list and stops Ridgeline decoding it, even if it's heard again.
		</p>
		{#if error}<p class="text-coral mt-1 text-xs">{error}</p>{/if}
	</div>
	<ul class="divide-line/60 max-h-96 divide-y overflow-y-auto">
		{#each channels as c (c.name)}
			<li class="flex items-center gap-3 {pad} py-2">
				<span class="min-w-0 flex-1 truncate font-mono text-sm {channelSourceColor(c.source)}">#{c.name}</span>
				{#if c.name !== 'sos'}
					<button type="button" disabled={busy} class={btn} onclick={() => change(adminChannels.remove, c.name)}>Remove</button>
				{/if}
			</li>
		{/each}
	</ul>
	{#if removed.length}
		<div class="border-line/70 border-t {pad} pt-3 pb-1">
			<div class="label">Removed · {removed.length}</div>
		</div>
		<ul class="divide-line/60 divide-y">
			{#each removed as c (c.name)}
				<li class="flex items-center gap-3 {pad} py-2">
					<span class="text-fg-faint min-w-0 flex-1 truncate font-mono text-sm">#{c.name}</span>
					<button type="button" disabled={busy} class={btn} onclick={() => change(adminChannels.restore, c.name)}>Restore</button>
				</li>
			{/each}
		</ul>
	{/if}
</section>
