<script lang="ts">
	import type { LiveGroup } from '$lib/live.svelte';
	import { api, type Node } from '$lib/api';
	import { ago, shortKey, fmtSnr, snrColor, roleColor, roleLabel, fmtCoord } from '$lib/format';
	import PayloadTag from './PayloadTag.svelte';

	interface Props {
		group: LiveGroup | null;
		onclose: () => void;
	}
	let { group, onclose }: Props = $props();

	let nodes = $state<Node[]>([]);
	let copied = $state('');

	// Pull the node list once (for resolving path-hop prefixes to names).
	$effect(() => {
		if (group && nodes.length === 0) api.nodes().then((n) => (nodes = n)).catch(() => {});
	});

	const resolveHop = (hop: string): Node | undefined => nodes.find((n) => n.publicKey.startsWith(hop));

	// The most complete observation (longest path) represents the packet.
	const lead = $derived.by(() => {
		if (!group) return null;
		return [...group.events].sort((a, b) => (b.path?.length ?? 0) - (a.path?.length ?? 0))[0];
	});

	const events = $derived(
		group ? [...group.events].sort((a, b) => +new Date(a.receivedAt) - +new Date(b.receivedAt)) : []
	);
	const firstAt = $derived(events.length ? +new Date(events[0].receivedAt) : 0);
	const span = $derived(
		events.length > 1 ? (+new Date(events[events.length - 1].receivedAt) - firstAt) / 1000 : 0
	);
	const distinctPaths = $derived(new Set(events.map((e) => (e.path ?? []).join('>'))).size);

	const title = $derived(
		group?.node ? group.node.name || shortKey(group.node.publicKey, 8, 4) : (group?.messageHash ?? '')
	);

	async function copy(text: string, label: string) {
		await navigator.clipboard.writeText(text);
		copied = label;
		setTimeout(() => (copied = ''), 1200);
	}

	function advTime(ts?: number): string {
		if (!ts) return '—';
		return new Date(ts * 1000).toISOString().replace('T', ' ').slice(0, 19) + 'Z';
	}
	const rel = (iso: string) => `+${((+new Date(iso) - firstAt) / 1000).toFixed(1)}s`;
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && group && onclose()} />

{#if group && lead}
	{@const node = lead.node}
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 backdrop-blur-sm md:items-center md:p-6"
		onclick={onclose}
		role="dialog"
		aria-modal="true"
		tabindex="-1"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div
			class="panel rise flex max-h-[88vh] w-full flex-col md:max-w-2xl"
			style="animation-duration:.25s"
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Header -->
			<div class="border-line/70 flex items-start gap-3 border-b px-5 py-4">
				<div class="min-w-0 flex-1">
					<div class="mb-2 flex flex-wrap items-center gap-2">
						<PayloadTag type={group.payloadType} />
						<span class="label">{group.routeType}</span>
						<span class="label">v{lead.payloadVersion ?? 0}</span>
						<span class="label">· {group.kind === 'node' ? 'all from node' : 'one transmission'}</span>
					</div>
					<h2 class="font-display text-fg truncate text-lg font-700">{title}</h2>
					{#if node}
						<a
							href="/nodes/{node.publicKey}"
							onclick={onclose}
							class="font-mono text-fg-faint hover:text-signal text-[0.68rem]"
							>{shortKey(node.publicKey, 12, 6)} →</a
						>
					{/if}
				</div>
				<button onclick={onclose} class="text-fg-faint hover:text-fg shrink-0 text-xl leading-none" aria-label="Close">✕</button>
			</div>

			<!-- Summary -->
			<div class="border-line/50 grid grid-cols-4 gap-3 border-b px-5 py-3">
				{#each [{ k: 'Observations', v: String(group.count) }, { k: 'Observers', v: String(group.observers.length) }, { k: 'Best SNR', v: fmtSnr(group.bestSnr) }, { k: 'Hops', v: String(lead.pathHops) }] as s (s.k)}
					<div>
						<div class="label">{s.k}</div>
						<div class="font-display tnum text-fg mt-1 text-xl font-700">{s.v}</div>
					</div>
				{/each}
			</div>

			<div class="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-4">
				<!-- Packet facts -->
				<section>
					<div class="label mb-2">Packet</div>
					<div class="divide-line/40 border-line/50 divide-y rounded-[var(--radius)] border text-sm">
						{#each [{ k: 'Message Hash', v: group.messageHash, mono: true }, { k: 'Route Type', v: group.routeType }, { k: 'Payload Type', v: group.payloadType }, { k: 'Payload Version', v: String(lead.payloadVersion ?? 0) }, { k: 'Hop Count', v: String(lead.pathHops) }, ...(lead.hashSize ? [{ k: 'Hash Size', v: `${lead.hashSize}-byte` }] : []), ...(lead.transportCodes ? [{ k: 'Transport Codes', v: lead.transportCodes.map((c) => '0x' + c.toString(16).toUpperCase()).join(', '), mono: true }] : []), { k: 'First Heard', v: ago(events[0]?.receivedAt) + ' ago' }, { k: 'Reception Span', v: span > 0 ? span.toFixed(1) + 's across observers' : 'single observer' }] as f (f.k)}
							<div class="flex items-center justify-between gap-3 px-3 py-2">
								<span class="label normal-case">{f.k}</span>
								<span class="text-fg text-right {f.mono ? 'font-mono text-xs' : 'text-sm'} tnum">{f.v}</span>
							</div>
						{/each}
					</div>
				</section>

				<!-- Path -->
				{#if (lead.path ?? []).length > 0}
					<section>
						<div class="label mb-2 flex items-center justify-between">
							<span>Path · {lead.path?.length} hops</span>
							{#if distinctPaths > 1}<span class="text-amber normal-case">{distinctPaths} variants across observers</span>{/if}
						</div>
						<div class="flex flex-wrap items-center gap-1.5">
							{#each lead.path ?? [] as hop, i (i)}
								{#if i > 0}<span class="text-fg-faint">→</span>{/if}
								{@const n = resolveHop(hop)}
								{#if n}
									<a
										href="/nodes/{n.publicKey}"
										onclick={onclose}
										class="border-line bg-panel-2/60 hover:border-signal/50 rounded-[var(--radius)] border px-2 py-1 text-xs"
										style="color:{roleColor(n.role)}">{n.name || shortKey(n.publicKey)}</a
									>
								{:else}
									<span
										class="border-line/60 font-mono text-fg-faint rounded-[var(--radius)] border border-dashed px-2 py-1 text-xs"
										title="no located node with this key prefix">{hop}</span
									>
								{/if}
							{/each}
						</div>
					</section>
				{/if}

				<!-- Payload -->
				<section>
					<div class="label mb-2">Payload</div>
					{#if node}
						<div class="divide-line/40 border-line/50 divide-y rounded-[var(--radius)] border text-sm">
							<div class="flex items-center justify-between px-3 py-2">
								<span class="label normal-case">Role</span>
								<span class="text-sm" style="color:{roleColor(node.role)}">{roleLabel(node.role)}</span>
							</div>
							<div class="flex items-center justify-between px-3 py-2">
								<span class="label normal-case">Location</span>
								<span class="font-mono text-fg text-xs tnum">{fmtCoord(node.latitude, node.longitude)}</span>
							</div>
							<div class="flex items-center justify-between px-3 py-2">
								<span class="label normal-case">Advertised Time</span>
								<span class="font-mono text-fg text-xs tnum">{advTime(node.timestamp)}</span>
							</div>
						</div>
					{:else if lead.payloadRaw}
						<button
							onclick={() => copy(lead.payloadRaw ?? '', 'payload')}
							class="panel-hover w-full rounded-[var(--radius)] border border-line/50 px-3 py-2 text-left"
						>
							<div class="label mb-1 flex justify-between">
								<span class="normal-case">{lead.payloadRaw.length / 2} bytes (encrypted/opaque)</span>
								<span class={copied === 'payload' ? '!text-signal' : ''}>{copied === 'payload' ? 'copied' : 'copy'}</span>
							</div>
							<div class="font-mono text-fg-dim break-all text-[0.68rem] leading-relaxed">{lead.payloadRaw}</div>
						</button>
					{:else}
						<div class="text-fg-faint text-sm">No payload.</div>
					{/if}
				</section>

				<!-- Raw -->
				{#if lead.raw}
					<section>
						<button
							onclick={() => copy(lead.raw ?? '', 'raw')}
							class="panel-hover w-full rounded-[var(--radius)] border border-line/50 px-3 py-2 text-left"
						>
							<div class="label mb-1 flex justify-between">
								<span>Raw Packet · {lead.raw.length / 2} bytes</span>
								<span class={copied === 'raw' ? '!text-signal' : ''}>{copied === 'raw' ? 'copied' : 'copy'}</span>
							</div>
							<div class="font-mono text-fg-dim break-all text-[0.68rem] leading-relaxed">{lead.raw}</div>
						</button>
					</section>
				{/if}

				<!-- Receptions -->
				<section>
					<div class="label mb-2">Receptions · {events.length}</div>
					<div class="overflow-hidden rounded-[var(--radius)] border border-line/50">
						<div class="label grid grid-cols-[52px_1fr_44px_52px_40px] gap-2 border-b border-line/50 px-3 py-2">
							<span>Δt</span><span>Observer</span><span class="text-right">Hops</span><span class="text-right">SNR</span><span class="text-right">RSSI</span>
						</div>
						<div class="divide-line/30 max-h-52 divide-y overflow-y-auto">
							{#each events as e (e.observerId + e.receivedAt)}
								<div class="grid grid-cols-[52px_1fr_44px_52px_40px] items-center gap-2 px-3 py-1.5 text-xs">
									<span class="font-mono text-fg-faint tnum">{rel(e.receivedAt)}</span>
									<span class="font-mono text-fg-dim truncate">{e.observerId ?? '—'}</span>
									<span class="font-mono text-fg-dim text-right tnum">{e.path?.length ?? 0}</span>
									<span class="font-mono text-right tnum" style="color:{snrColor(e.snr)}">{fmtSnr(e.snr)}</span>
									<span class="font-mono text-fg-dim text-right tnum">{e.rssi ?? '—'}</span>
								</div>
							{/each}
						</div>
					</div>
				</section>
			</div>
		</div>
	</div>
{/if}
