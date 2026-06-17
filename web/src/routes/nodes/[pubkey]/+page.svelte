<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type Node } from '$lib/api';
	import { live } from '$lib/live.svelte';
	import { ago, shortKey, fmtCoord, fmtSnr, snrColor } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import RoleBadge from '$lib/components/RoleBadge.svelte';
	import PayloadTag from '$lib/components/PayloadTag.svelte';

	const pubkey = $derived(page.params.pubkey ?? '');
	let allNodes = $state<Node[]>([]);
	let loading = $state(true);
	let copied = $state(false);

	const node = $derived(allNodes.find((n) => n.publicKey === pubkey) ?? null);

	// Each node advertises its own hash length (1, 2, or 3 bytes); its hash ID
	// is that prefix of its public key — how the node is identified as a relay
	// in packet paths. Compute the ID and how many other known nodes collide
	// with it at that length.
	const hashId = $derived.by(() => {
		const hs = node?.hashSize ?? 0;
		if (!hs) return null;
		const hex = pubkey.slice(0, hs * 2);
		const shared = allNodes.filter(
			(n) => n.publicKey !== pubkey && n.publicKey.startsWith(hex)
		).length;
		return { bytes: hs, hex, shared };
	});

	async function refresh() {
		try {
			allNodes = await api.nodes();
		} finally {
			loading = false;
		}
	}
	onMount(() => {
		refresh();
		const t = setInterval(refresh, 8000);
		return () => clearInterval(t);
	});

	const sightings = $derived(live.events.filter((e) => e.node?.publicKey === pubkey));

	async function copyKey() {
		await navigator.clipboard.writeText(pubkey);
		copied = true;
		setTimeout(() => (copied = false), 1200);
	}
</script>

<PageHeader eyebrow="Node Detail" title={node?.name || shortKey(pubkey, 8, 4)}>
	{#if node}<RoleBadge role={node.role} />{/if}
	{#if node?.gpsSuspect}
		<span
			class="text-coral border-coral/40 bg-coral/10 flex items-center gap-1.5 rounded-[var(--radius)] border px-2 py-1 text-xs"
			title="GPS coordinates appear corrupt — this node's reported location is a statistical outlier versus the rest of the mesh, so it is hidden from the maps. The node is otherwise valid and still appears in packet paths."
		>
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
				<path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" />
				<path d="M12 9v4M12 17h.01" />
			</svg>
			Corrupt GPS
		</span>
	{/if}
	<a href="/nodes" class="label hover:text-signal transition-colors">← All nodes</a>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	{#if loading}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">Loading…</div>
	{:else if !node}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">
			Node not found. It may not have advertised yet.
		</div>
	{:else}
		<!-- Public key strip -->
		<button
			onclick={copyKey}
			class="panel panel-hover mb-6 flex w-full items-center gap-3 px-5 py-3 text-left"
		>
			<span class="label shrink-0">PUBKEY</span>
			<span class="font-mono text-fg break-all text-xs md:text-sm">{pubkey}</span>
			<span class="label ml-auto shrink-0 {copied ? '!text-signal' : ''}"
				>{copied ? 'COPIED' : 'COPY'}</span
			>
		</button>

		<div class="grid gap-6 lg:grid-cols-3">
			<!-- Facts -->
			<div class="space-y-3 lg:col-span-1">
				{#each [{ k: 'Role', v: node.role }, { k: 'Location', v: fmtCoord(node.latitude, node.longitude) }, { k: 'Adverts heard', v: String(node.advertCount) }, { k: 'First seen', v: ago(node.firstSeen) + ' ago' }, { k: 'Last seen', v: ago(node.lastSeen) + ' ago' }] as f (f.k)}
					<div class="panel flex items-center justify-between px-5 py-3.5">
						<span class="label">{f.k}</span>
						<span class="font-mono text-fg text-sm tnum">{f.v}</span>
					</div>
				{/each}

				<!-- Hash ID identity -->
				<div class="panel px-5 py-4">
					<div class="label mb-3 flex items-center justify-between">
						Hash ID
						{#if hashId}
							<span class="font-mono text-fg-faint normal-case">{hashId.bytes}-byte</span>
						{/if}
					</div>
					{#if hashId}
						<div class="flex items-baseline gap-3">
							<span class="font-mono text-signal glow-signal text-2xl font-700 tracking-[0.15em]"
								>{hashId.hex}</span
							>
							{#if hashId.shared === 0}
								<span
									class="font-mono text-signal bg-signal/10 rounded-[var(--radius)] px-1.5 py-0.5 text-[0.62rem]"
									>unique</span
								>
							{:else}
								<span
									class="font-mono text-amber bg-amber/10 rounded-[var(--radius)] px-1.5 py-0.5 text-[0.62rem] tnum"
									title="{hashId.shared} other known node{hashId.shared > 1 ? 's' : ''} collide at this length"
									>+{hashId.shared} collision{hashId.shared > 1 ? 's' : ''}</span
								>
							{/if}
						</div>
						<p class="text-fg-faint mt-3 text-[0.68rem] leading-snug">
							This node sets a {hashId.bytes}-byte hash, so it appears as
							<span class="font-mono">{hashId.hex}</span> in packet paths.
						</p>
					{:else}
						<div class="text-fg-faint text-sm">Unknown — not seen advertising yet.</div>
						<p class="text-fg-faint mt-2 text-[0.68rem] leading-snug">
							A node’s hash length is learned from its advertisement.
						</p>
					{/if}
				</div>
			</div>

			<!-- Live sightings for this node -->
			<section class="panel lg:col-span-2">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
					{#if live.connected}<span class="live-dot"></span>{/if}
					<h2 class="font-display text-fg text-sm font-700 tracking-wide">SESSION SIGHTINGS</h2>
					<span class="label ml-auto tnum">{sightings.length}</span>
				</div>
				{#if sightings.length === 0}
					<div class="text-fg-faint px-5 py-12 text-center text-sm">
						No live packets from this node yet this session.
					</div>
				{:else}
					<div class="divide-line/50 divide-y">
						{#each sightings.slice(0, 30) as ev (ev.messageHash + ev.receivedAt)}
							<div class="flex items-center gap-3 px-5 py-2.5 text-sm">
								<span class="font-mono text-fg-faint w-10 text-xs tnum">{ago(ev.receivedAt)}</span>
								<PayloadTag type={ev.payloadType} />
								<span class="font-mono text-fg-faint flex-1 truncate text-xs">
									via {ev.observerId ?? '—'}
								</span>
								<span class="font-mono tnum text-xs" style="color:{snrColor(ev.snr)}"
									>{fmtSnr(ev.snr)} dB</span
								>
							</div>
						{/each}
					</div>
				{/if}
			</section>
		</div>
	{/if}
</div>
