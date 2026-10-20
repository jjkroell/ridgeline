<script lang="ts">
	import Modal from './Modal.svelte';
	import { announce } from '$lib/announce.svelte';

	// Each entry is one new capability, in user-facing terms.
	const items: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M5 12a2 2 0 100-4 2 2 0 000 4zM19 7a2 2 0 100-4 2 2 0 000 4zM19 21a2 2 0 100-4 2 2 0 000 4zM6.8 9.2l10.4-4.4M6.8 10.8l10.4 7.4',
			title: 'RF neighbours, straight from the repeaters',
			body: 'Node pages have a new RF neighbours section listing the nodes a repeater hears directly over the air, with the SNR it hears each at and how long ago. The data comes from the repeaters themselves: openHop repeaters (and firmware built with neighbour publishing) can send their neighbour table to Ridgeline\'s MQTT broker about once a day. To build it, the repeater runs a zero-hop node discover, records who answers, asks each one which regions it serves, and publishes the table. Ridgeline keeps the latest table from each repeater, so it appears on that repeater\'s page, and every node it lists gets a "Hears this node directly" line on its own page. Only lists from trusted observers on this mesh are kept, and anything older than about two weeks is hidden.'
		},
		{
			icon: 'M4 12h4l2-5 4 10 2-5h4',
			title: 'Turning it on for your repeater',
			body: 'If you run openHop and feed Ridgeline, set neighbors: true on your Ridgeline broker entry; the table publishes on its own schedule (daily by default), or right away from the neighbour publish option in the web UI. Coverage grows with every repeater that opts in. Nodes outside their range keep the existing list, now called Path neighbours: links inferred from which nodes relay packets one after another, a useful guess rather than a measurement.'
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
