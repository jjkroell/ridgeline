<script lang="ts">
	import NodeDetail from './NodeDetail.svelte';

	interface Props {
		pubkey: string | null;
		onclose: () => void;
	}
	let { pubkey, onclose }: Props = $props();
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && pubkey && onclose()} />

{#if pubkey}
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
			<div class="border-line/70 flex items-center gap-3 border-b px-5 py-4">
				<div class="label flex items-center gap-2">
					<span class="bg-signal/70 inline-block h-px w-6"></span> Node Detail
				</div>
				<a href="/nodes/{pubkey}" class="label hover:text-signal ml-auto transition-colors">Open full page ↗</a>
				<button onclick={onclose} class="text-fg-faint hover:text-fg shrink-0 text-xl leading-none" aria-label="Close">✕</button>
			</div>
			<div class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
				<NodeDetail {pubkey} heading />
			</div>
		</div>
	</div>
{/if}
