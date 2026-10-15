<script lang="ts">
	import Modal from './Modal.svelte';
	import { announce } from '$lib/announce.svelte';

	// Each entry is one new capability, in user-facing terms.
	const items: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M4.9 19.1a10 10 0 0 1 0-14.2M19.1 4.9a10 10 0 0 1 0 14.2M7.8 16.2a6 6 0 0 1 0-8.4M16.2 7.8a6 6 0 0 1 0 8.4M12 13a1 1 0 100-2 1 1 0 000 2z',
			title: 'More direct radio links, from node-discover replies',
			body: 'When someone runs a node discover, every repeater in range answers directly and names itself by its full key, so any observer that hears the answer is within radio range of that repeater. Ridgeline now counts those answers as direct links alongside zero-hop adverts, which roughly doubles the links known for the last few hours. You\'ll see more neighbours on observer pages and more connections in the RF adjacency graph on the Analytics page. Observers also no longer list themselves as their own neighbour.'
		}
	];
</script>

{#if announce.open}
	<Modal onclose={() => announce.close()} size="2xl">
		<div class="border-line/70 flex items-center gap-3 border-b px-6 py-4">
			<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-700">New</span>
			<h2 class="font-display text-fg text-lg font-700">What's new on Ridgeline</h2>
			<button
				onclick={() => announce.close()}
				class="text-fg-faint hover:text-fg ml-auto text-xl leading-none"
				aria-label="Close">✕</button
			>
		</div>

		<div class="min-h-0 flex-1 overflow-y-auto px-6 py-5">
			<ul class="flex flex-col gap-4">
				{#each items as item (item.title)}
					<li class="flex items-start gap-3">
						<span
							class="bg-signal/10 text-signal mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full"
						>
							<svg
								viewBox="0 0 24 24"
								class="h-[18px] w-[18px]"
								fill="none"
								stroke="currentColor"
								stroke-width="1.6"
								stroke-linecap="round"
								stroke-linejoin="round"><path d={item.icon} /></svg
							>
						</span>
						<div class="min-w-0">
							<div class="text-fg text-sm font-600">{item.title}</div>
							<div class="text-fg-dim mt-0.5 text-sm leading-relaxed">{item.body}</div>
						</div>
					</li>
				{/each}
			</ul>
		</div>

		<div class="border-line/70 flex items-center justify-end gap-3 border-t px-6 py-4">
			<a
				href="/about"
				onclick={() => announce.close()}
				class="text-fg-dim hover:text-fg text-sm transition-colors">Learn more</a
			>
			<button
				onclick={() => announce.close()}
				class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors"
				>Got it</button
			>
		</div>
	</Modal>
{/if}
