<script lang="ts">
	import Modal from './Modal.svelte';
	import { announce } from '$lib/announce.svelte';

	// Each entry is one new capability, in user-facing terms.
	const items: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M4 9h16M4 15h16M10 3 8 21M16 3l-2 18',
			title: 'Find the hashtag channels people are using',
			body: 'The Channels page now lists the named #hashtag channels active on the mesh under "Discovered on the mesh", on desktop and mobile. Add one with a click to read along. A channel\'s name never goes over the air, so Ridgeline only lists a name once it has proved it matches real traffic. When you add a hashtag channel yourself you choose whether to share it with the list, and the Add panel now has a Hashtag / Private switch.'
		},
		{
			icon: 'M4.9 19.1a10 10 0 0 1 0-14.2M19.1 4.9a10 10 0 0 1 0 14.2M7.8 16.2a6 6 0 0 1 0-8.4M16.2 7.8a6 6 0 0 1 0 8.4M12 13a1 1 0 100-2 1 1 0 000 2z',
			title: 'Radio settings are shown once for the whole mesh',
			body: 'Every node is on the same settings now, so they appear once, in the sidebar (in the More menu on mobile), instead of on each node\'s page, where some nodes were still showing their old 910.425 MHz frequency.'
		},
		{
			icon: 'M13 2 3 14h9l-1 8 10-12h-9l1-8z',
			title: 'Faster, steadier pages',
			body: 'Fixed several slowdowns that could leave pages, maps and the node list on "Loading…" for seconds at a time, most noticeably for a few minutes after an update. Node pages, the Overview and the analytics views now share their heavy work between everyone viewing them.'
		},
		{
			icon: 'M12 8v4l3 2M12 21a9 9 0 100-18 9 9 0 000 18z',
			title: 'Individual packets are kept for 45 days',
			body: 'That covers everything the site shows; the longest view, a node\'s activity heatmap, goes back 30 days. Node details, first-seen dates and all-time counts are kept as before.'
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
