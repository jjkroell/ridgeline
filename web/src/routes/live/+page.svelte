<script lang="ts">
	import { live, groupLive, type LiveGroup } from '$lib/live.svelte';
	import { ago, shortKey, fmtSnr, snrColor } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';
	import LiveGroupModal from '$lib/components/LiveGroupModal.svelte';

	let paused = $state(false);
	let frozen = $state<typeof live.events>([]);
	let selected = $state<LiveGroup | null>(null);

	const groups = $derived(groupLive(paused ? frozen : live.events));

	function togglePause() {
		if (!paused) frozen = [...live.events];
		paused = !paused;
	}
</script>

<PageHeader eyebrow="Real-time Telemetry" title="Live Feed">
	<button
		onclick={togglePause}
		class="font-mono flex items-center gap-2 rounded-[var(--radius)] border px-3 py-1.5 text-xs transition-colors
			{paused
			? 'border-amber/50 text-amber bg-amber/10'
			: 'border-line text-fg-dim hover:border-line-bright hover:text-fg'}"
	>
		{#if paused}▶ Resume{:else}❙❙ Pause{/if}
	</button>
	<div class="font-mono text-fg-dim flex items-center gap-2 text-xs">
		{#if live.connected}<span class="live-dot"></span>{/if}
		<span class="tnum text-signal">{live.total}</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	<div class="panel overflow-hidden">
		<div
			class="label border-line/70 grid grid-cols-[64px_110px_1fr_70px_60px] gap-4 border-b px-5 py-3 md:grid-cols-[70px_120px_1fr_90px_80px_60px]"
		>
			<span>Latest</span>
			<span>Type</span>
			<span>Source</span>
			<span class="hidden text-right md:block">Route</span>
			<span class="text-right">Best SNR</span>
			<span class="text-right">Hops</span>
		</div>

		{#if groups.length === 0}
			<div class="text-fg-faint px-5 py-16 text-center text-sm">
				{#if live.connected}Waiting for the first packet…{:else}Connecting to live feed…{/if}
			</div>
		{:else}
			<div class="divide-line/40 divide-y">
				{#each groups as g (g.key)}
					<button
						onclick={() => (selected = g)}
						class="panel-hover grid w-full grid-cols-[64px_110px_1fr_70px_60px] items-center gap-4 px-5 py-2.5 text-left text-sm md:grid-cols-[70px_120px_1fr_90px_80px_60px]"
					>
						<span class="font-mono text-fg-faint text-xs tnum">{ago(g.latest)}</span>
						<span><PayloadTag type={g.payloadType} /></span>
						<span class="flex min-w-0 items-center gap-2">
							<span class="truncate">
								{#if g.node}
									<span class="text-fg">{g.node.name || shortKey(g.node.publicKey)}</span>
								{:else}
									<span class="font-mono text-fg-faint">{g.messageHash}</span>
								{/if}
							</span>
							{#if g.count > 1}
								<span
									class="font-mono text-signal bg-signal/10 shrink-0 rounded-[var(--radius)] px-1.5 py-0.5 text-[0.62rem] tnum"
									title="{g.count} observations from {g.observers.length} observers">×{g.count}</span
								>
							{/if}
						</span>
						<span class="font-mono text-fg-dim hidden text-right text-xs md:block">{g.routeType}</span>
						<span class="font-mono tnum text-right text-xs" style="color:{snrColor(g.bestSnr)}"
							>{fmtSnr(g.bestSnr)}</span
						>
						<span class="font-mono tnum text-fg-dim text-right text-xs">{g.events[0].pathHops}</span>
					</button>
				{/each}
			</div>
		{/if}
	</div>
</div>

<LiveGroupModal group={selected} onclose={() => (selected = null)} />
