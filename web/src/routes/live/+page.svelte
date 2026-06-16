<script lang="ts">
	import { live } from '$lib/live.svelte';
	import { ago, shortKey, fmtSnr, snrColor } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';

	let paused = $state(false);
	let frozen = $state<typeof live.events>([]);

	const rows = $derived(paused ? frozen : live.events);

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
			class="label border-line/70 grid grid-cols-[64px_110px_1fr_70px_60px] gap-4 border-b px-5 py-3 md:grid-cols-[70px_120px_1fr_90px_80px_70px_60px]"
		>
			<span>Time</span>
			<span>Type</span>
			<span>Source</span>
			<span class="hidden text-right md:block">Route</span>
			<span class="text-right">SNR</span>
			<span class="hidden text-right md:block">RSSI</span>
			<span class="text-right">Hops</span>
		</div>

		{#if rows.length === 0}
			<div class="text-fg-faint px-5 py-16 text-center text-sm">
				{#if live.connected}Waiting for the first packet…{:else}Connecting to live feed…{/if}
			</div>
		{:else}
			<div class="divide-line/40 divide-y">
				{#each rows as ev (ev.messageHash + ev.receivedAt)}
					<div
						class="grid grid-cols-[64px_110px_1fr_70px_60px] items-center gap-4 px-5 py-2.5 text-sm md:grid-cols-[70px_120px_1fr_90px_80px_70px_60px]"
					>
						<span class="font-mono text-fg-faint text-xs tnum">{ago(ev.receivedAt)}</span>
						<span><PayloadTag type={ev.payloadType} /></span>
						<span class="min-w-0 truncate">
							{#if ev.node}
								<a href="/nodes/{ev.node.publicKey}" class="text-fg hover:text-signal"
									>{ev.node.name || shortKey(ev.node.publicKey)}</a
								>
							{:else}
								<span class="font-mono text-fg-faint">{ev.messageHash}</span>
							{/if}
							{#if ev.observerId}
								<span class="font-mono text-fg-faint ml-2 text-[0.66rem]">via {ev.observerId}</span
								>
							{/if}
						</span>
						<span class="font-mono text-fg-dim hidden text-right text-xs md:block"
							>{ev.routeType}</span
						>
						<span class="font-mono tnum text-right text-xs" style="color:{snrColor(ev.snr)}"
							>{fmtSnr(ev.snr)}</span
						>
						<span class="font-mono tnum text-fg-dim hidden text-right text-xs md:block"
							>{ev.rssi ?? '—'}</span
						>
						<span class="font-mono tnum text-fg-dim text-right text-xs">{ev.pathHops}</span>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
