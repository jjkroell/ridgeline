<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type Observer, type ObserverAnalytics } from '$lib/api';
	import { ago, fmtNum, skewColor, fmtSkew, snrColor, roleColor, roleLabel } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	const id = $derived(page.params.id ?? '');

	const windows = [
		{ label: '6h', sec: 21600 },
		{ label: '24h', sec: 86400 },
		{ label: '3d', sec: 259200 }
	];
	let windowSec = $state(86400);

	let observer = $state<Observer | null>(null);
	let data = $state<ObserverAnalytics | null>(null);
	let loading = $state(true);

	async function refresh() {
		try {
			const [obs, a] = await Promise.all([
				api.observers(),
				api.observerAnalytics(id, windowSec).catch(() => null)
			]);
			observer = obs.find((o) => o.id === id) ?? null;
			data = a;
		} finally {
			loading = false;
		}
	}
	// Reload on window change + poll.
	$effect(() => {
		windowSec;
		refresh();
	});
	onMount(() => {
		const t = setInterval(refresh, 15000);
		return () => clearInterval(t);
	});

	function fresh(lastSeen?: string): boolean {
		return !!lastSeen && Date.now() - new Date(lastSeen).getTime() < 5 * 60 * 1000;
	}

	const kpis = $derived([
		{ label: 'Packets', value: data ? fmtNum(data.totalPackets) : '—', accent: true },
		{ label: 'Packets / hr', value: data ? data.packetsPerHour.toFixed(1) : '—', accent: true },
		{ label: 'Distinct nodes', value: data ? fmtNum(data.distinctNodes) : '—', hint: 'distinct nodes whose adverts reached this observer' },
		{ label: 'Direct (0-hop)', value: data ? fmtNum(data.directNodes) : '—', color: 'var(--color-lime)', hint: 'nodes heard directly — RF neighbours' },
		{ label: 'Avg SNR', value: data?.avgSnr != null ? data.avgSnr.toFixed(1) + ' dB' : '—', color: snrColor(data?.avgSnr) },
		{ label: 'Clock skew', value: fmtSkew(data?.clockSkewMs), color: skewColor(data?.clockSkewMs), hint: 'median receive-time deviation from consensus on shared packets — large = drifting clock' }
	]);

	const maxAct = $derived(Math.max(1, ...(data?.activity ?? []).map((c) => c)));
	const maxPayload = $derived(Math.max(1, ...(data?.payloadTypes ?? []).map((p) => p.count)));
	const maxSnr = $derived(Math.max(1, ...(data?.snrHist ?? []).map((b) => b.count)));
	const maxNbr = $derived(Math.max(1, ...(data?.neighbors ?? []).map((n) => n.count)));
</script>

{#snippet barRow(label: string, count: number, max: number, color: string)}
	<div class="flex items-center gap-3 px-5 py-1.5">
		<div class="text-fg-dim w-28 shrink-0 truncate text-xs">{label}</div>
		<div class="bg-line/40 relative h-3 flex-1 overflow-hidden rounded-[var(--radius)]">
			<div class="h-full rounded-[var(--radius)]" style="width:{(count / max) * 100}%;background:{color}"></div>
		</div>
		<div class="font-mono tnum text-fg-dim w-12 shrink-0 text-right text-xs">{fmtNum(count)}</div>
	</div>
{/snippet}

<PageHeader eyebrow="Listening Post" title={id}>
	<div class="flex items-center gap-3">
		{#if observer}
			<Tooltip text={fresh(observer.lastSeen) ? 'reporting' : 'silent'}>
				<span class="h-2 w-2 rounded-full" style="background:{fresh(observer.lastSeen) ? 'var(--color-signal)' : 'var(--color-fg-faint)'}"></span>
			</Tooltip>
		{/if}
		<div class="bg-panel border-line flex overflow-hidden rounded-[var(--radius)] border text-xs">
			{#each windows as wn (wn.sec)}
				<button
					onclick={() => (windowSec = wn.sec)}
					class="font-mono px-3 py-1.5 transition-colors {windowSec === wn.sec ? 'bg-signal/15 text-signal' : 'text-fg-dim hover:text-fg'}"
					>{wn.label}</button
				>
			{/each}
		</div>
		<a href="/observers" class="label hover:text-signal transition-colors">← All</a>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	{#if loading && !data}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">Loading…</div>
	{:else if !observer}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">
			Observer not found. It may not have reported recently.
		</div>
	{:else}
		<!-- KPI strip -->
		<div class="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
			{#each kpis as c, i (c.label)}
				<div class="panel rise relative overflow-hidden px-4 py-4" style="animation-delay:{i * 40}ms">
					{#if c.accent}<div class="from-signal/[0.07] absolute inset-0 bg-gradient-to-br to-transparent"></div>{/if}
					<div class="relative">
						<div class="label flex items-center gap-1">
							{c.label}
							{#if c.hint}<Tooltip text={c.hint}><span class="text-fg-faint cursor-help text-[0.6rem]">ⓘ</span></Tooltip>{/if}
						</div>
						<div class="font-display tnum mt-2 text-2xl font-700 lg:text-3xl" style="color:{c.color ?? (c.accent ? 'var(--color-signal)' : 'var(--color-fg)')}">
							{c.value}
						</div>
					</div>
				</div>
			{/each}
		</div>

		<!-- Feed throughput timeline -->
		<section class="panel rise mt-6" style="animation-delay:120ms">
			<div class="border-line/70 flex items-center justify-between border-b px-5 py-3.5">
				<h2 class="font-display text-fg text-sm font-700 tracking-wide">FEED THROUGHPUT</h2>
				<span class="font-mono text-fg-faint text-[0.68rem]">packets / hour</span>
			</div>
			<div class="px-5 py-5">
				{#if !data || data.activity.length === 0 || data.totalPackets === 0}
					<div class="text-fg-faint py-8 text-center text-sm">No packets in window.</div>
				{:else}
					<div class="flex h-[140px] items-end gap-px">
						{#each data.activity as count, i (i)}
							<Tooltip text="{data.activity.length - 1 - i === 0 ? 'this hour' : `${data.activity.length - 1 - i}h ago`} · {count} pkt" class="flex-1 items-end">
								<div class="bg-signal/70 hover:bg-signal w-full rounded-t-[2px] transition-colors" style="height:{Math.max(2, (count / maxAct) * 140)}px"></div>
							</Tooltip>
						{/each}
					</div>
					<div class="text-fg-faint mt-2 flex justify-between font-mono text-[0.62rem]">
						<span>-{Math.round(data.windowHours)}h</span><span>peak {maxAct} pkt/h</span><span>now</span>
					</div>
				{/if}
			</div>
		</section>

		<!-- Payload mix + SNR distribution -->
		<div class="mt-6 grid gap-6 lg:grid-cols-2">
			<section class="panel rise" style="animation-delay:180ms">
				<div class="border-line/70 border-b px-5 py-3.5">
					<h2 class="font-display text-fg text-sm font-700 tracking-wide">PAYLOAD MIX</h2>
				</div>
				<div class="py-3">
					{#each data?.payloadTypes ?? [] as p (p.label)}
						{@render barRow(p.label, p.count, maxPayload, 'var(--color-signal)')}
					{/each}
				</div>
			</section>
			<section class="panel rise" style="animation-delay:220ms">
				<div class="border-line/70 flex items-center justify-between border-b px-5 py-3.5">
					<h2 class="font-display text-fg text-sm font-700 tracking-wide">SNR DISTRIBUTION</h2>
					<span class="font-mono text-fg-faint text-[0.68rem]">dB</span>
				</div>
				<div class="py-3">
					{#each data?.snrHist ?? [] as b (b.label)}
						{@render barRow(b.label, b.count, maxSnr, 'var(--color-amber)')}
					{/each}
				</div>
			</section>
		</div>

		<!-- Direct RF neighbours -->
		<section class="panel rise mt-6" style="animation-delay:260ms">
			<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3">
				<h3 class="font-display text-fg text-sm font-700 tracking-wide">DIRECTLY HEARS</h3>
				<span class="label normal-case ml-2 text-fg-faint">zero-hop RF neighbours</span>
				<span class="label ml-auto tnum">{data?.neighbors.length ?? 0} node{(data?.neighbors.length ?? 0) === 1 ? '' : 's'}</span>
			</div>
			{#if !data || data.neighbors.length === 0}
				<div class="text-fg-faint px-5 py-8 text-center text-sm">No zero-hop adverts heard in window.</div>
			{:else}
				<div class="divide-line/40 divide-y">
					{#each data.neighbors as l (l.nodeKey)}
						<a href="/nodes/{l.nodeKey}" class="panel-hover flex items-center gap-3 px-5 py-2 text-sm">
							<span class="h-2 w-2 shrink-0 rounded-full" style="background:{roleColor(l.role)}"></span>
							<span class="text-fg min-w-0 flex-1 truncate">{l.nodeName}</span>
							<span class="label !text-[0.58rem]">{roleLabel(l.role)}</span>
							<Tooltip text="times heard at zero hops in window" class="shrink-0">
								<span class="font-mono text-fg-faint text-xs tnum">×{l.count}</span>
							</Tooltip>
						</a>
					{/each}
				</div>
			{/if}
		</section>

		<!-- Seen facts -->
		<div class="panel rise divide-line/40 mt-6 divide-y" style="animation-delay:320ms">
			{#each [{ k: 'Status', v: fresh(observer.lastSeen) ? 'Reporting' : 'Silent', c: fresh(observer.lastSeen) ? 'var(--color-signal)' : 'var(--color-fg-faint)' }, { k: 'Region', v: observer.region || data?.region || '—' }, { k: 'First seen', v: ago(observer.firstSeen) + ' ago' }, { k: 'Last heard', v: ago(observer.lastSeen) + ' ago' }, { k: 'Packets (all-time)', v: fmtNum(observer.packetCount) }] as f (f.k)}
				<div class="flex items-center justify-between px-5 py-2.5">
					<span class="label normal-case">{f.k}</span>
					<span class="font-mono text-sm tnum" style="color:{f.c ?? 'var(--color-fg)'}">{f.v}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>
