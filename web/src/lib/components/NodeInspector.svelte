<script lang="ts">
	// Node detail docked inside a map, as a side panel (macOS inspector) rather
	// than a modal — the map stays visible and pannable beside it, and clicking
	// another node swaps the panel in place. Esc closes.
	import NodeDetail from './NodeDetail.svelte';

	let { pubkey, onclose }: { pubkey: string | null; onclose: () => void } = $props();

	function key(e: KeyboardEvent) {
		if (e.key === 'Escape' && pubkey) onclose();
	}
</script>

<svelte:window onkeydown={key} />

{#if pubkey}
	<aside
		data-inspector
		class="border-line bg-panel absolute top-3 right-3 bottom-3 z-30 flex w-[400px] max-w-[calc(100%-1.5rem)] flex-col overflow-hidden rounded-[var(--radius)] border shadow-2xl"
		aria-label="Node detail"
	>
		<div class="border-line/70 flex shrink-0 items-center gap-3 border-b px-4 py-3">
			<div class="label">Node detail</div>
			<a href="/nodes/{pubkey}" class="text-signal ml-auto text-xs font-600 hover:underline">Open page →</a>
			<button onclick={onclose} class="text-fg-faint hover:text-fg shrink-0 text-lg leading-none" aria-label="Close">✕</button>
		</div>
		<div class="min-h-0 flex-1 overflow-y-auto px-4 py-4">
			{#key pubkey}
				<NodeDetail {pubkey} heading compact />
			{/key}
		</div>
	</aside>
{/if}

<style>
	aside {
		animation: inspector-in 0.28s cubic-bezier(0.25, 1, 0.5, 1);
	}
	@keyframes inspector-in {
		from {
			opacity: 0;
			transform: translateX(16px);
		}
	}
	/* NodeDetail was laid out for a wide modal; long keys wrap in the panel. */
	aside :global(.font-mono) {
		overflow-wrap: anywhere;
	}
	@media (prefers-reduced-motion: reduce) {
		aside {
			animation: none;
		}
	}
</style>
