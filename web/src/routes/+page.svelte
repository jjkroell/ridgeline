<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Stats, type Node } from '$lib/api';
	import { live } from '$lib/live.svelte';
	import { ago, shortKey, fmtNum, snrColor, fmtSnr } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import RoleBadge from '$lib/components/RoleBadge.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';

	let stats = $state<Stats | null>(null);
	let nodes = $state<Node[]>([]);
	let error = $state<string | null>(null);

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

	const cards = $derived([
		{ label: 'Nodes', value: stats?.nodes ?? 0, accent: true, icon: 'node' },
		{ label: 'Observers', value: stats?.observers ?? 0, accent: false, icon: 'eye' },
		{ label: 'Observations', value: stats?.observations ?? 0, accent: false, icon: 'pulse' },
		{ label: 'Session Events', value: live.total, accent: false, icon: 'bolt' }
	]);

	const icons: Record<string, string> = {
		node: 'M12 8a4 4 0 100 8 4 4 0 000-8zM12 2v4m0 12v4M2 12h4m12 0h4',
		eye: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7-10-7-10-7z',
		pulse: 'M2 12h4l3 8 4-16 3 8h6',
		bolt: 'M13 2 4 14h7l-1 8 9-12h-7z'
	};
</script>

<PageHeader eyebrow="Network Observatory" title="Overview">
	<div class="font-mono text-fg-dim flex items-center gap-2 text-xs">
		<span class="text-fg-faint">LAST PACKET</span>
		<span class="text-signal tnum">{ago(stats?.lastPacketAt)} ago</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	{#if error}
		<div class="panel border-coral/40 text-coral mb-6 px-4 py-3 text-sm">
			Can't reach the daemon — {error}
		</div>
	{/if}

	<!-- Stat row -->
	<div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
		{#each cards as c, i (c.label)}
			<div
				class="panel rise relative overflow-hidden px-5 py-5"
				style="animation-delay:{i * 50}ms"
			>
				{#if c.accent}
					<div
						class="from-signal/[0.07] absolute inset-0 bg-gradient-to-br to-transparent"
					></div>
				{/if}
				<div class="relative">
					<div class="label flex items-center justify-between">
						{c.label}
						<svg
							viewBox="0 0 24 24"
							class="h-4 w-4 {c.accent ? 'text-signal' : 'text-fg-faint'}"
							fill="none"
							stroke="currentColor"
							stroke-width="1.5"
							stroke-linecap="round"
							stroke-linejoin="round"><path d={icons[c.icon]} /></svg
						>
					</div>
					<div
						class="font-display tnum mt-3 text-4xl font-700 {c.accent
							? 'text-signal glow-signal'
							: 'text-fg'}"
					>
						{fmtNum(c.value)}
					</div>
				</div>
			</div>
		{/each}
	</div>

	<!-- Two columns -->
	<div class="mt-6 grid gap-6 lg:grid-cols-5">
		<!-- Live feed -->
		<section class="panel rise lg:col-span-3" style="animation-delay:240ms">
			<div class="border-line/70 flex items-center justify-between border-b px-5 py-3.5">
				<div class="flex items-center gap-2.5">
					{#if live.connected}<span class="live-dot"></span>{/if}
					<h2 class="font-display text-fg text-sm font-700 tracking-wide">LIVE FEED</h2>
				</div>
				<a href="/live" class="label hover:text-signal transition-colors">View all →</a>
			</div>
			<div class="divide-line/50 divide-y">
				{#if live.events.length === 0}
					<div class="text-fg-faint px-5 py-10 text-center text-sm">
						Waiting for packets…
					</div>
				{:else}
					{#each live.events.slice(0, 9) as ev (ev.messageHash + ev.receivedAt)}
						<div class="flex items-center gap-3 px-5 py-2.5 text-sm">
							<PayloadTag type={ev.payloadType} />
							<span class="text-fg-dim min-w-0 flex-1 truncate">
								{#if ev.node}
									<span class="text-fg">{ev.node.name || shortKey(ev.node.publicKey)}</span>
								{:else}
									<span class="font-mono text-fg-faint">{ev.messageHash}</span>
								{/if}
							</span>
							<span class="font-mono tnum text-xs" style="color:{snrColor(ev.snr)}"
								>{fmtSnr(ev.snr)} dB</span
							>
							<span class="font-mono text-fg-faint w-10 text-right text-xs"
								>{ev.observerId ?? '—'}</span
							>
						</div>
					{/each}
				{/if}
			</div>
		</section>

		<!-- Recent nodes -->
		<section class="panel rise lg:col-span-2" style="animation-delay:300ms">
			<div class="border-line/70 flex items-center justify-between border-b px-5 py-3.5">
				<h2 class="font-display text-fg text-sm font-700 tracking-wide">RECENT NODES</h2>
				<a href="/nodes" class="label hover:text-signal transition-colors">All →</a>
			</div>
			<div class="divide-line/50 divide-y">
				{#if nodes.length === 0}
					<div class="text-fg-faint px-5 py-10 text-center text-sm">No nodes yet.</div>
				{:else}
					{#each nodes.slice(0, 7) as n (n.publicKey)}
						<a
							href="/nodes/{n.publicKey}"
							class="panel-hover flex items-center gap-3 px-5 py-2.5"
						>
							<div class="min-w-0 flex-1">
								<div class="text-fg truncate text-sm font-medium">
									{n.name || shortKey(n.publicKey)}
								</div>
								<div class="font-mono text-fg-faint mt-0.5 text-[0.68rem]">
									{shortKey(n.publicKey, 8, 4)}
								</div>
							</div>
							<RoleBadge role={n.role} />
							<span class="font-mono text-fg-faint w-8 text-right text-xs tnum"
								>{ago(n.lastSeen)}</span
							>
						</a>
					{/each}
				{/if}
			</div>
		</section>
	</div>
</div>
