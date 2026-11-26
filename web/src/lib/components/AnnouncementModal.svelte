<script lang="ts">
	import Modal from './Modal.svelte';
	import { announce, RELEASE_ON } from '$lib/announce.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { seasonal } from '$lib/seasonal.svelte';
	import { sos as sosState } from '$lib/sos.svelte';

	// Each entry is one new capability, in user-facing terms. While a season is
	// in force its own items lead (see $lib/seasonal.svelte; they vanish when it
	// ends), then the newest release's (`latest`, red: the emergency channel);
	// `settings` is an earlier one, under an "Earlier" label.
	onMount(() => seasonal.load());
	const season = $derived(seasonal.season);
	function trySeason() {
		if (!seasonal.on) seasonal.toggle();
		announce.close();
		goto('/live-map');
	}
	// Land on #mayday with its Monitoring card (the Yes / No) open.
	function openMonitoring() {
		sosState.setCard('monitors', true);
		announce.close();
	}
	type Item = { icon: string; title: string; body: string; href?: string; cta?: string };
	// The admin's emergency switch picks the note (announce.current): off, #sos
	// is no longer monitored (no word of #mayday: that name isn't public yet);
	// on, #mayday has taken its place.
	const off: Item[] = [
		{
			icon: 'M18.4 5.6a9 9 0 11-12.8 0 9 9 0 0112.8 0zM5.6 5.6l12.8 12.8',
			title: 'Ridgeline no longer monitors #sos',
			body: 'Ridgeline has stopped watching the #sos channel. It no longer sends #sos alert emails, sounds the alarm, marks callers on the maps or keeps a list of who is monitoring. #sos still works on the mesh as an ordinary channel, but nobody is watching it through Ridgeline.'
		},
		{
			icon: 'M12 9v4M12 17h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z',
			title: 'In an emergency, call 911',
			body: 'If you have any phone or cell service, call 911 first. The mesh is not an emergency service, and nothing on it reaches emergency services by itself.'
		},
		{
			icon: 'M5 12l5 5L20 7',
			title: 'Nothing for you to do',
			body: 'Your alert-email choice and your monitoring entry are kept, unused. You can leave #sos in your channel list or remove it.'
		},
		{
			icon: 'M9 11l3 3 8-8M20 12v7a2 2 0 01-2 2H6a2 2 0 01-2-2V5a2 2 0 012-2h9',
			title: 'Tell us what you think',
			body: 'Should Ridgeline monitor an emergency channel again? Six quick questions, all optional.',
			href: '/survey',
			cta: 'Take the survey'
		}
	];
	const on: Item[] = [
		{
			icon: 'M12 9v4M12 17h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z',
			title: '#mayday is now the mesh’s emergency channel',
			body: 'Ridgeline watches #mayday instead of #sos. Add #mayday to your MeshCore app or radio (a hashtag channel, no key needed): everyone has it under Channels here. Post there if you or someone near you needs help, and say where you are. In a life-threatening emergency, call 911 first if you can.'
		},
		{
			icon: 'M9 8a3.2 3.2 0 100-.01M3 20a6 6 0 0112 0M16 11l2 2 4-4',
			title: 'Monitoring? Add yourself to the #mayday list',
			body: 'The #mayday list starts empty. Once your companion radio has joined #mayday, sign in, open #mayday under Channels, tap the monitoring count and answer Yes with your radio’s name.'
		},
		{
			icon: 'M3 6h18v12H3zM3 7l9 6 9-6',
			title: 'Alert emails now cover #mayday',
			body: 'If you turned on alert emails, they now come from #mayday. Change it under Account → Account settings.'
		},
		{
			icon: 'M18.4 5.6a9 9 0 11-12.8 0 9 9 0 0112.8 0zM5.6 5.6l12.8 12.8',
			title: 'Ridgeline no longer monitors #sos',
			body: 'Messages on #sos no longer trigger alerts or emails. You can remove #sos from your channel list.'
		}
	];
	const latest = $derived(announce.current === RELEASE_ON ? on : off);
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
</script>

{#snippet heading(text: string)}
	<div class="border-line/60 mt-5 mb-4 flex items-center gap-3 border-t pt-4">
		<span class="label" data-caps>{text}</span>
	</div>
{/snippet}

{#snippet list(items: Item[], red: boolean)}
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
					{#if item.href}
						<a
							href={item.href}
							onclick={() => announce.close()}
							class="bg-coral text-ink mt-2.5 inline-block rounded-[var(--radius)] px-4 py-2 text-sm font-600 transition-[filter] hover:brightness-110"
							>{item.cta}</a
						>
					{/if}
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

			{@render heading(announce.current === RELEASE_ON ? 'Also new: #mayday' : 'Also new: #sos')}
			{/if}

			{@render list(latest, true)}
			{@render heading('Earlier: recommended settings')}
			{@render list(settings, false)}
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
			{#if announce.current === RELEASE_ON}
				<a
					href="/channels?channel=mayday"
					onclick={openMonitoring}
					class="bg-coral text-ink rounded-[var(--radius)] px-4 py-2 text-sm font-600 transition-[filter] hover:brightness-110"
					>Open #mayday</a
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
