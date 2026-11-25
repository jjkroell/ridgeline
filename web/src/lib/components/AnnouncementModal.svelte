<script lang="ts">
	import Modal from './Modal.svelte';
	import { announce } from '$lib/announce.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { seasonal } from '$lib/seasonal.svelte';
	import { sos as sosState } from '$lib/sos.svelte';

	// Each entry is one new capability, in user-facing terms. While a season is
	// in force its own items lead (see $lib/seasonal.svelte; they vanish when it
	// ends), then the newest release's (`latest`, red: it is about
	// #sos); `settings` and `sos` are earlier ones, under "Earlier" labels.
	onMount(() => seasonal.load());
	const season = $derived(seasonal.season);
	function trySeason() {
		if (!seasonal.on) seasonal.toggle();
		announce.close();
		goto('/live-map');
	}
	// Land on #sos with its Monitoring card (the Yes / No) open.
	function openMonitoring() {
		sosState.setCard('monitors', true);
		announce.close();
	}
	const latest: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M12 9v4M12 17h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z',
			title: 'The #sos monitoring list has been cleared: please add yourself again',
			body: 'People are no longer listed by sending join on the channel, and everyone who was on the old list has come off it. If you monitor #sos, please go through the new selection below, so others can see you are listening. Your radio still receives #sos as before; only the list on Ridgeline was cleared.'
		},
		{
			icon: 'M9 8a3.2 3.2 0 100-.01M3 20a6 6 0 0112 0M16 11l2 2 4-4',
			title: 'How to get back on the list',
			body: 'Sign in to Ridgeline, open #sos under Channels and tap the monitoring count beside its name. Enter your companion radio’s name and answer Yes to “Are you monitoring #sos?”. Answer No at any time to come off. If it says the name isn’t valid, Ridgeline hasn’t heard your radio: send a flood advert, wait a minute and try again.'
		},
		{
			icon: 'M18.4 5.6a9 9 0 11-12.8 0 9 9 0 0112.8 0zM5.6 5.6l12.8 12.8',
			title: 'Please don’t send join or leave on #sos',
			body: 'They do nothing now. Keep #sos clear for people who need help.'
		}
	];
	const settings: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M4 6h9m4 0h3M4 12h3m4 0h9M4 18h11m4 0h1M15 4v4M9 10v4M17 16v4',
			title: 'Recommended settings, all on one page',
			body: 'The About page is now Settings: the radio settings every node on the mesh needs (909.000 MHz, 62.5 kHz, SF7, CR5), which path hash ID size to use, and the recommended settings for repeaters. Find it under Settings in the sidebar (on a phone, More → Radio settings). The sections on bridging and on what the mesh carries moved to the FAQ, and old links to them still work.'
		},
		{
			icon: 'M4 5h16v14H4zM7 9l3 3-3 3M12 15h5',
			title: 'Run a repeater? Pick your neighbour count, copy the commands',
			body: 'The page has advert intervals, and TX delay values based on how many neighbours your repeater hears, with a short explanation of what each setting does. Choose your neighbour count and it lists the commands to send, each with its own Copy button.'
		}
	];
	const sos: { icon: string; title: string; body: string }[] = [
		{
			icon: 'M12 9v4M12 17h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z',
			title: '#sos, the mesh’s emergency channel',
			body: 'Everyone now has #sos under Channels, next to Public. It is for people who need help: post there if you or someone near you needs assistance, and say where you are. Keep it clear otherwise. In a life-threatening emergency, call 911 first; the mesh is a backup, not a replacement.'
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
		},
		{
			icon: 'M11 5L6 9H2v6h4l5 4zM15.5 8.5a5 5 0 010 7M19 5a10 10 0 010 14',
			title: 'A sound and a notification, too',
			body: 'While Ridgeline is open, it can also play a bell and show a system notification when someone calls for help on #sos. Turn either on under the Alerts pill beside #sos (on a phone, also under More → Settings), and press Test to hear it. They are off unless you turn them on, and the choice is saved in this browser only.'
		}
	];
</script>

{#snippet heading(text: string)}
	<div class="border-line/60 mt-5 mb-4 flex items-center gap-3 border-t pt-4">
		<span class="label" data-caps>{text}</span>
	</div>
{/snippet}

{#snippet list(items: { icon: string; title: string; body: string }[], red: boolean)}
	<ul class="flex flex-col gap-4">
		{#each items as item (item.title)}
			<li class="flex items-start gap-3">
				<span
					class="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full {red ? 'text-coral' : 'bg-signal/10 text-signal'}"
					style={red ? 'background:color-mix(in srgb, var(--color-coral) 14%, transparent)' : undefined}
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
{/snippet}

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
			{#if season}
			<ul class="flex flex-col gap-4">
				{#each season.announce as item (item.title)}
					<li class="flex items-start gap-3">
						<span class="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full text-lg leading-none" style="background:color-mix(in srgb, {season.accent} 16%, transparent)" aria-hidden="true"><img src={seasonal.src(item.emoji)} alt="" class="h-5 w-5" /></span>
						<div class="min-w-0">
							<div class="text-fg text-sm font-600">{item.title}</div>
							<div class="text-fg-dim mt-0.5 text-sm leading-relaxed">{item.body}</div>
						</div>
					</li>
				{/each}
			</ul>

			{@render heading('Also new: #sos monitoring')}
			{/if}

			{@render list(latest, true)}
			{@render heading('Earlier: recommended settings')}
			{@render list(settings, false)}
			{@render heading('Earlier: #sos')}
			{@render list(sos, true)}
		</div>

		<div class="border-line/70 flex flex-wrap items-center justify-end gap-x-3 gap-y-2 border-t px-6 py-4">
			{#if season}
				<button
					onclick={trySeason}
					class="rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-[filter] hover:brightness-125"
					style="border-color:color-mix(in srgb, {season.accent} 50%, transparent);color:{season.accent}"
					><img src={seasonal.src(season.emoji)} alt="" class="mr-1.5 inline h-4 w-4 align-[-0.15em]" />{season.tryLabel}</button
				>
			{/if}
			<a
				href="/channels?channel=sos"
				onclick={openMonitoring}
				class="bg-coral text-ink rounded-[var(--radius)] px-4 py-2 text-sm font-600 transition-[filter] hover:brightness-110"
				>Open the #sos list</a
			>
			<button
				onclick={() => announce.close()}
				class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors"
				>Got it</button
			>
		</div>
	</Modal>
{/if}
