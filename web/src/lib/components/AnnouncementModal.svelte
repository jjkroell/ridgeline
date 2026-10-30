<script lang="ts">
	import Modal from './Modal.svelte';
	import { announce } from '$lib/announce.svelte';
	import { theme } from '$lib/theme.svelte';

	// Each entry is one new capability, in user-facing terms. The newest
	// release's items lead (red for #sos); `items` below are the previous
	// release's, kept under an "Earlier this week" label.
	const latest: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M12 9v4M12 17h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z',
			title: '#sos, the mesh’s emergency channel',
			body: 'Everyone now has #sos under Channels, next to Public. It is for people who need help: post there if you or someone near you needs assistance, and say where you are. Keep it clear otherwise. In a life-threatening emergency, call 911 first; the mesh is a backup, not a replacement.'
		},
		{
			icon: 'M9 8a3.2 3.2 0 100-.01M3 20a6 6 0 0112 0M16 4.5a3.2 3.2 0 010 6.3M18 20a6 6 0 00-3-5.2',
			title: 'Help keep watch on #sos',
			body: 'Join #sos on your radio or app and send one (just one) test message. You will be listed as a monitor, so everyone can see how many people are listening. Then leave notifications on for #sos. Send unsub to come off the list. Test and unsub messages leave the #sos chat on Ridgeline after 5 minutes, and your name stays on the list.'
		},
		{
			icon: 'M18 8a6 6 0 00-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 01-3.4 0',
			title: '#sos lights up when someone needs help',
			body: 'If a #sos message mentions sos, emergency, mayday, 911, threat or life, or says someone needs help, the #sos row and Channels in the menu (More on a phone) pulse red until you open #sos. They also pulse if it arrived while Ridgeline was closed, for up to 12 hours.'
		},
		{
			icon: 'M3 6h18v12H3zM3 7l9 6 9-6',
			title: 'Get an email when someone calls for help',
			body: 'If you have a Ridgeline account, you can turn on #sos alert emails under Account → Account settings (on a phone, More → Settings → Account). When one of the words above is heard on #sos, Ridgeline emails you the message with a link to the channel. Messages that arrive close together come in one email, with at least 10 minutes between emails. Alerts are off unless you turn them on, your email address needs to be confirmed first, and you can turn them off again at any time.'
		}
	];
	const items: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M12 3a9 9 0 100 18c1.1 0 1.6-.9 1.2-1.8-.5-1-.1-2.2 1.1-2.2H17a4 4 0 004-4c0-5-4-10-9-10zM7.5 12a1 1 0 100-2 1 1 0 000 2zM10.5 8a1 1 0 100-2 1 1 0 000 2zM15 8a1 1 0 100-2 1 1 0 000 2z',
			title: 'Harbour, a new look for Ridgeline',
			body: 'A clean, rounded new theme: soft grey surfaces, frosted glass bars, your device’s own system font, rounder cards and a single accent colour. Pick Light, Dark, or Auto to follow your device’s appearance, and choose an accent: blue, purple, pink, red, orange, green or graphite. To try it, open Theme at the bottom of the sidebar (on a phone, the round button at the top right, or More → Settings). Your choice is saved in this browser, and every other theme is still there.'
		},
		{
			icon: 'M8 3h8a2 2 0 012 2v14a2 2 0 01-2 2H8a2 2 0 01-2-2V5a2 2 0 012-2zM11 18h2',
			title: 'On a phone, it feels like an app',
			body: 'With Harbour on, the phone layout gets large page titles that tuck into the bar as you scroll, content that slides under frosted bars, full-screen maps, toggle switches and segmented controls, and grouped lists. Add Ridgeline to your home screen and it opens with a proper launch screen, and you can swipe in from the left edge to go back.'
		},
		{
			icon: 'M4 14h16M4 14a2 2 0 01-2-2V6a2 2 0 012-2h16a2 2 0 012 2v6a2 2 0 01-2 2M4 14v4a2 2 0 002 2h12a2 2 0 002-2v-4M12 8h.01',
			title: 'Node cards on the phone maps',
			body: 'Tapping a node on the phone map or live map opens a card over the map instead of leaving it. Drag the card up for everything else about the node (first heard, adverts, relays, clock and radio), drag it down to close it, or tap Details for the full page.'
		},
		{
			icon: 'M9 11V6a1.5 1.5 0 013 0v5m0-1a1.5 1.5 0 013 0v1m0 0a1.5 1.5 0 013 0v3a6 6 0 01-6 6h-1a5 5 0 01-4-2l-2.5-3.5a1.5 1.5 0 012.4-1.8L9 15',
			title: 'Shortcuts for touch and mouse',
			body: 'Press and hold a node or observer in a list on your phone, or right-click it on a computer, for a quick menu: open it, star it, show it on the map, copy its key or share a link. On a phone, swipe a node to the left to star it, and pull down on a list to refresh it. On charts such as battery, noise, activity, throughput and channel use, drag along the chart to read each value.'
		},
		{
			icon: 'M10.5 18a7.5 7.5 0 100-15 7.5 7.5 0 000 15zM21 21l-5.2-5.2',
			title: 'Faster to get around',
			body: 'Press Ctrl+K (⌘K on a Mac), or Search in the sidebar, to jump to any page or node by name. Clicking a node on the map opens a side panel so the map stays in view. On a phone, the Nodes list can be sorted A–Z with a letter strip down the side to jump through it, and Nodes and Observers both have a search field (in Harbour, pull the list down to show it). On an iPad held sideways, Nodes shows the list and the selected node side by side.'
		},
		{
			icon: 'M2 12h3l2-7 3.5 15L16 8l2 4h4',
			title: 'How busy is the mesh right now?',
			body: 'Both live maps now show a small pill at the top with the number of packets heard in the last minute and the last node heard. Tap it to see the last 15 minutes minute by minute and the latest packets; tap a packet’s node to open it.'
		},
		{
			icon: 'M4 4h7v7H4zM13 4h7v4h-7zM13 10h7v10h-7zM4 13h7v7H4z',
			title: 'Widgets in three sizes, and a Settings page',
			body: 'In Harbour, Customize on the overview lets you make each widget small, medium or large, like home-screen widgets, with its headline number up front. Phones get a Settings page (More → Settings) for theme, map style, packet times and your account.'
		},
		{
			icon: 'M12 3v18M3 12h18M5.6 5.6l12.8 12.8',
			title: 'Easier to read',
			body: 'Turn on Reduce transparency (under the theme choices) to make frosted bars and panels solid. It also switches on by itself if your device asks for less transparency, and in Harbour, edges and secondary text get firmer when Increase Contrast is on. Copying a key or starring a node now shows a short confirmation in the middle of the screen.'
		},
		{
			icon: 'M3 20l6-9 4 5 3-4 5 8zM16 4.5a1.5 1.5 0 100 3 1.5 1.5 0 000-3z',
			title: 'A calmer map, and a new logo on the way',
			body: 'There is a new Muted base map, a soft, low-colour map that lets the nodes stand out (Harbour uses it by default; change it under Base map on any map). Ridgeline’s logo is being replaced: the one you see now is a stand-in, and the new one is being chosen by public vote at logo.ve7kod.ca until Friday, November 6 at 18:00 BC time.'
		},
		{
			icon: 'M5 12.5l4.5 4.5L19 7.5',
			title: 'Fixes in every theme',
			body: 'Bold text is bold again across the site, so headings and numbers stand out more in every theme. The live map now plays its sound as soon as you tap or click anywhere, instead of staying silent until you switched sound off and on. Pages show a quiet outline while they load instead of “Loading…”, and the node count on the phone map is no longer hidden under Copy link.'
		}
	];
	const onHarbour = $derived(theme.choice === 'apple' || theme.choice === 'apple-dark' || theme.choice === 'apple-auto');
	function tryIt() {
		theme.set('apple-auto');
		announce.close();
	}
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
				{#each latest as item (item.title)}
					<li class="flex items-start gap-3">
						<span class="text-coral mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full" style="background:color-mix(in srgb, var(--color-coral) 14%, transparent)">
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

			<div class="border-line/60 mt-5 mb-4 flex items-center gap-3 border-t pt-4">
				<span class="label" data-caps>Earlier this week: Harbour</span>
			</div>

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
			<a
				href="/channels?channel=sos"
				onclick={() => announce.close()}
				class="text-coral hover:brightness-125 text-sm font-600 transition-[filter]">Open #sos</a
			>
			{#if !onHarbour}
				<button
					onclick={tryIt}
					class="bg-signal text-ink hover:bg-signal-deep rounded-[var(--radius)] px-4 py-2 text-sm font-600 transition-colors"
					>Try Harbour</button
				>
			{/if}
			<button
				onclick={() => announce.close()}
				class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors"
				>Got it</button
			>
		</div>
	</Modal>
{/if}
