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
	let node = $state<Node | null>(null);
	let loading = $state(true);
	let copied = $state(false);

	async function refresh() {
		try {
			const nodes = await api.nodes();
			node = nodes.find((n) => n.publicKey === pubkey) ?? null;
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
