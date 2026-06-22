<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Stats, type Node } from '$lib/api';
	import { live, groupLive, type LiveGroup } from '$lib/live.svelte';
	import { ago, shortKey, fmtNum, snrColor, fmtSnr, nodeStatus, roleColor, roleLabel } from '$lib/format';
	import { favorites } from '$lib/favorites.svelte';
	import LiveGroupModal from '$lib/components/LiveGroupModal.svelte';

	let stats = $state<Stats | null>(null);
	let nodes = $state<Node[]>([]);
	let error = $state<string | null>(null);
	let selected = $state<LiveGroup | null>(null);

	const groups = $derived(groupLive(live.events));

	async function refresh() {
		try {
			[stats, nodes] = await Promise.all([api.stats(), api.nodes()]);
			error = null;
		} catch (e) {
			error = (e as Error).message;
		}
	}
	onMount(() => {
		refresh();
		const t = setInterval(refresh, 5000);
		return () => clearInterval(t);
	});

	const kpis = $derived([
		{ label: 'Nodes', value: stats?.nodes ?? 0, accent: true },
		{ label: 'Observers', value: stats?.observers ?? 0, accent: false },
		{ label: 'Packets', value: stats?.observations ?? 0, accent: false },
		{ label: 'Session', value: live.total, accent: false }
	]);

	const favNodes = $derived(
		favorites.keys
			.map((k) => nodes.find((n) => n.publicKey.toUpperCase() === k))
			.filter((n): n is Node => !!n)
	);
</script>

<div class="px-4 py-4">
	<div class="text-fg-faint mb-3 flex items-center gap-2 font-mono text-[0.68rem]">
		<span>LAST PACKET</span>
		<span class="text-signal tnum">{ago(stats?.lastPacketAt)} ago</span>
	</div>

	{#if error}
		<div class="border-coral/40 bg-coral/5 text-coral mb-4 rounded-xl border px-4 py-3 text-sm">
			Can't reach the daemon — {error}
		</div>
	{/if}

	<!-- KPI tiles -->
	<div class="grid grid-cols-2 gap-3">
		{#each kpis as k (k.label)}
			<div class="border-line/60 bg-panel relative overflow-hidden rounded-2xl border px-4 py-4">
				{#if k.accent}<div class="from-signal/[0.08] absolute inset-0 bg-gradient-to-br to-transparent"></div>{/if}
				<div class="relative">
					<div class="label">{k.label}</div>
					<div class="font-display tnum mt-1.5 text-3xl font-700 {k.accent ? 'text-signal glow-signal' : 'text-fg'}">{fmtNum(k.value)}</div>
				</div>
			</div>
		{/each}
	</div>

	<!-- Favorites -->
	{#if favNodes.length}
		<div class="mt-5 mb-2 flex items-center gap-2">
			<svg viewBox="0 0 24 24" class="text-amber h-4 w-4" fill="currentColor"><path d="M12 2.5l2.9 5.9 6.5.95-4.7 4.6 1.1 6.45L12 17.9l-5.8 3.05 1.1-6.45-4.7-4.6 6.5-.95z" /></svg>
			<h2 class="font-display text-fg text-xs font-700 tracking-wide">FAVORITES</h2>
		</div>
		<div class="-mx-4 flex gap-3 overflow-x-auto px-4 pb-1" style="scrollbar-width:none">
			{#each favNodes as n (n.publicKey)}
				{@const st = nodeStatus(n)}
				<a href="/m/nodes/{n.publicKey}" class="border-line/60 bg-panel w-44 shrink-0 rounded-2xl border p-3.5">
					<div class="flex items-center gap-2">
						<span class="h-2 w-2 shrink-0 rounded-full" style="background:{st.color}"></span>
						<span class="text-fg truncate text-sm font-600">{n.name || shortKey(n.publicKey)}</span>
					</div>
					<div class="text-fg-faint mt-2 flex items-center gap-1.5 font-mono text-[0.62rem]">
						<span style="color:{roleColor(n.role)}">{roleLabel(n.role)}</span><span>·</span><span>{ago(n.lastSeen)}</span>
					</div>
				</a>
			{/each}
		</div>
	{/if}

	<!-- Live feed -->
	<div class="mt-5 mb-2 flex items-center gap-2">
		{#if live.connected}<span class="live-dot"></span>{/if}
		<h2 class="font-display text-fg text-xs font-700 tracking-wide">LIVE FEED</h2>
		<a href="/m/live" class="label text-signal ml-auto">View all →</a>
	</div>
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border">
		{#if groups.length === 0}
			<div class="text-fg-faint px-4 py-8 text-center text-sm">Waiting for packets…</div>
		{:else}
			{#each groups.slice(0, 8) as g (g.key)}
				<button onclick={() => (selected = g)} class="active:bg-line/40 flex w-full items-center gap-2.5 px-4 py-2.5 text-left">
					<span class="shrink-0 rounded-md px-1.5 py-0.5 text-[0.6rem] font-700" style="color:{roleColor(g.node?.role ?? '')};background:color-mix(in srgb, {roleColor(g.node?.role ?? '')} 12%, transparent)">{g.payloadType.slice(0, 4)}</span>
					<span class="min-w-0 flex-1 truncate text-sm">
						{#if g.node}<span class="text-fg">{g.node.name || shortKey(g.node.publicKey)}</span>{:else}<span class="font-mono text-fg-faint text-xs">{g.messageHash}</span>{/if}
						{#if g.count > 1}<span class="text-signal bg-signal/10 ml-1.5 rounded px-1 py-0.5 font-mono text-[0.58rem] tnum">×{g.count}</span>{/if}
					</span>
					<span class="font-mono text-xs tnum" style="color:{snrColor(g.bestSnr)}">{fmtSnr(g.bestSnr)}</span>
					<span class="text-fg-faint w-9 text-right font-mono text-[0.68rem]">{ago(g.latest)}</span>
				</button>
			{/each}
		{/if}
	</div>

	<!-- Recent nodes -->
	<div class="mt-5 mb-2 flex items-center gap-2">
		<h2 class="font-display text-fg text-xs font-700 tracking-wide">RECENT NODES</h2>
		<a href="/m/nodes" class="label text-signal ml-auto">All →</a>
	</div>
	<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border">
		{#each nodes.slice(0, 6) as n (n.publicKey)}
			{@const st = nodeStatus(n)}
			<a href="/m/nodes/{n.publicKey}" class="active:bg-line/40 flex items-center gap-3 px-4 py-3">
				<span class="h-2 w-2 shrink-0 rounded-full" style="background:{st.color}"></span>
				<div class="min-w-0 flex-1">
					<div class="text-fg truncate text-sm font-medium">{n.name || shortKey(n.publicKey)}</div>
					<div class="text-fg-faint mt-0.5 font-mono text-[0.62rem]" style="color:{roleColor(n.role)}">{roleLabel(n.role)}</div>
				</div>
				<span class="text-fg-faint font-mono text-[0.68rem] tnum">{ago(n.lastSeen)}</span>
			</a>
		{/each}
	</div>
</div>

<LiveGroupModal group={selected} onclose={() => (selected = null)} />
