<script lang="ts">
	import type { LiveGroup } from '$lib/live.svelte';
	import { ago, shortKey, fmtSnr, snrColor } from '$lib/format';
	import PayloadTag from './PayloadTag.svelte';

	interface Props {
		group: LiveGroup | null;
		onclose: () => void;
	}
	let { group, onclose }: Props = $props();

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
			class="panel rise flex max-h-[85vh] w-full flex-col md:max-w-2xl"
			style="animation-duration:.25s"
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Header -->
			<div class="border-line/70 flex items-start gap-3 border-b px-5 py-4">
				<div class="min-w-0 flex-1">
					<div class="mb-2 flex items-center gap-2">
						<PayloadTag type={group.payloadType} />
						<span class="label">{group.routeType}</span>
						{#if group.kind === 'node'}
							<span class="label">· all from node</span>
						{:else}
							<span class="label">· one transmission</span>
						{/if}
					</div>
					<h2 class="font-display text-fg truncate text-lg font-700">{title}</h2>
					{#if group.node}
						<a
							href="/nodes/{group.node.publicKey}"
							class="font-mono text-fg-faint hover:text-signal text-[0.68rem]"
							onclick={onclose}>{shortKey(group.node.publicKey, 12, 6)} →</a
						>
					{:else}
						<div class="font-mono text-fg-faint text-[0.68rem]">message hash</div>
					{/if}
				</div>
				<button
					onclick={onclose}
					class="text-fg-faint hover:text-fg shrink-0 text-xl leading-none"
					aria-label="Close">✕</button
				>
			</div>

			<!-- Summary -->
			<div class="border-line/50 grid grid-cols-3 gap-3 border-b px-5 py-3">
				{#each [{ k: 'Observations', v: String(group.count) }, { k: 'Observers', v: String(group.observers.length) }, { k: 'Best SNR', v: fmtSnr(group.bestSnr) + ' dB' }] as s (s.k)}
					<div>
						<div class="label">{s.k}</div>
						<div class="font-display tnum text-fg mt-1 text-xl font-700">{s.v}</div>
					</div>
				{/each}
			</div>

			<!-- Observation list -->
			<div class="min-h-0 flex-1 overflow-y-auto">
				<div
					class="label border-line/50 sticky top-0 grid grid-cols-[60px_1fr_70px_64px] gap-3 border-b bg-[var(--color-panel)] px-5 py-2.5"
				>
					<span>Time</span>
					<span>Observer</span>
					<span class="text-right">SNR</span>
					<span class="text-right">RSSI</span>
				</div>
				<div class="divide-line/40 divide-y">
					{#each group.events as ev (ev.observerId + ev.receivedAt + ev.messageHash)}
						<div class="grid grid-cols-[60px_1fr_70px_64px] gap-3 px-5 py-2.5 text-sm">
							<span class="font-mono text-fg-faint text-xs tnum">{ago(ev.receivedAt)}</span>
							<span class="font-mono text-fg-dim truncate text-xs">{ev.observerId ?? '—'}</span>
							<span class="font-mono tnum text-right text-xs" style="color:{snrColor(ev.snr)}"
								>{fmtSnr(ev.snr)}</span
							>
							<span class="font-mono tnum text-fg-dim text-right text-xs">{ev.rssi ?? '—'}</span>
						</div>
					{/each}
				</div>
			</div>
		</div>
	</div>
{/if}
