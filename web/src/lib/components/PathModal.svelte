<script lang="ts">
	import type { LiveGroup } from '$lib/live.svelte';
	import type { Node } from '$lib/api';
	import { shortKey, roleColor } from '$lib/format';
	import PayloadTag from './PayloadTag.svelte';

	interface Props {
		group: LiveGroup | null;
		nodes: Node[];
		showIds?: boolean;
		onclose: () => void;
	}
	let { group, nodes, showIds = false, onclose }: Props = $props();

	const resolveHop = (hop: string): Node | undefined => nodes.find((n) => n.publicKey.startsWith(hop));
	const hashId = (n: Node) => n.publicKey.slice(0, Math.max(1, n.hashSize || 2) * 2);

	// The most complete observed path (longest hop list) for this transmission.
	const path = $derived.by(() => {
		if (!group) return [] as string[];
		return [...group.events].sort((a, b) => (b.path?.length ?? 0) - (a.path?.length ?? 0))[0]?.path ?? [];
	});
	const variants = $derived(
		group ? new Set(group.events.map((e) => (e.path ?? []).join('>'))).size : 0
	);
	const title = $derived(
		group?.node ? group.node.name || shortKey(group.node.publicKey, 8, 4) : (group?.messageHash ?? '')
	);
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && group && onclose()} />

{#if group}
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
			class="panel rise flex max-h-[88vh] w-full flex-col md:max-w-lg"
			style="animation-duration:.2s"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="border-line/70 flex items-start gap-3 border-b px-5 py-4">
				<div class="min-w-0 flex-1">
					<div class="mb-2 flex flex-wrap items-center gap-2">
						<PayloadTag type={group.payloadType} />
						<span class="label">Full Path · {path.length} hops</span>
					</div>
					<h2 class="font-display text-fg truncate text-lg font-700">{title}</h2>
					<div class="font-mono text-fg-faint text-[0.68rem]">{group.messageHash}</div>
				</div>
				<button onclick={onclose} class="text-fg-faint hover:text-fg shrink-0 text-xl leading-none" aria-label="Close">✕</button>
			</div>

			<div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
				{#if path.length === 0}
					<div class="text-fg-faint text-sm">Direct transmission — no relay hops recorded.</div>
				{:else}
					{#if variants > 1}
						<div class="label text-amber mb-3 normal-case">
							Showing the most complete route · {variants} variants seen across repeats (open the packet for per-repeat paths)
						</div>
					{/if}
					<div class="flex flex-wrap items-center gap-1.5">
						{#each path as hop, i (i)}
							{#if i > 0}<span class="text-fg-faint">→</span>{/if}
							{@const n = resolveHop(hop)}
							{#if n}
								<a
									href="/nodes/{n.publicKey}"
									onclick={onclose}
									class="border-line bg-panel-2/60 hover:border-signal/50 rounded-[var(--radius)] border px-2 py-1 text-xs"
									style="color:{roleColor(n.role)}">{showIds ? hashId(n) : n.name || shortKey(n.publicKey)}</a
								>
							{:else}
								<span
									class="border-line/60 font-mono text-fg-faint rounded-[var(--radius)] border border-dashed px-2 py-1 text-xs"
									title="no located node with this key prefix">{hop}</span
								>
							{/if}
						{/each}
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}
