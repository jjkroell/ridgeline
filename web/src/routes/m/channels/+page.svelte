<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import { onMount, tick } from 'svelte';
	import { channels, type Channel } from '$lib/channels.svelte';
	import { decryptGroupText } from '$lib/channel-crypto';
	import { live } from '$lib/live.svelte';
	import { api, type LiveEvent } from '$lib/api';
	import { ago } from '$lib/format';
	import Linkify from '$lib/components/Linkify.svelte';

	const HISTORY_SEC = 86400;
	let history = $state<LiveEvent[]>([]);
	let loadingHistory = $state(true);
	let selectedId = $state<string | null>(null);
	let pane = $state<'list' | 'chat'>('list');

	onMount(async () => {
		channels.init();
		try {
			history = await api.channelHistory(HISTORY_SEC);
		} finally {
			loadingHistory = false;
		}
	});

	interface ChatMsg {
		hash: string;
		sender: string;
		text: string;
		receivedAt: string;
		t: number;
		observers: number;
	}

	const byChannel = $derived.by(() => {
		type Acc = Omit<ChatMsg, 'observers'> & { obs: Set<string> };
		const buckets = new Map<string, Map<string, Acc>>();
		for (const ev of [...history, ...live.events]) {
			if (ev.payloadType !== 'GroupText' || !ev.payloadRaw) continue;
			const hb = ev.payloadRaw.slice(0, 2).toUpperCase();
			for (const c of channels.list) {
				if (c.hashByte !== hb) continue;
				const d = decryptGroupText(ev.payloadRaw, c.keyHex);
				if (!d) continue;
				let m = buckets.get(c.id);
				if (!m) buckets.set(c.id, (m = new Map()));
				const t = +new Date(ev.receivedAt);
				const ex = m.get(ev.messageHash);
				if (!ex) {
					m.set(ev.messageHash, { hash: ev.messageHash, sender: d.sender, text: d.text, receivedAt: ev.receivedAt, t, obs: new Set(ev.observerId ? [ev.observerId] : []) });
				} else {
					if (t < ex.t) { ex.t = t; ex.receivedAt = ev.receivedAt; }
					if (ev.observerId) ex.obs.add(ev.observerId);
				}
				break;
			}
		}
		const out = new Map<string, ChatMsg[]>();
		for (const [id, m] of buckets) {
			out.set(id, [...m.values()].map(({ obs, ...r }) => ({ ...r, observers: obs.size })).sort((a, b) => a.t - b.t));
		}
		return out;
	});

	const selected = $derived(channels.list.find((c) => c.id === selectedId) ?? null);
	const messages = $derived(selectedId ? (byChannel.get(selectedId) ?? []) : []);
	const lastOf = (id: string) => { const a = byChannel.get(id); return a && a.length ? a[a.length - 1] : undefined; };

	function open(id: string) {
		selectedId = id;
		pane = 'chat';
		pinned = true;
	}

	let scroller = $state<HTMLDivElement>();
	let pinned = $state(true);
	function onScroll() {
		if (!scroller) return;
		pinned = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 80;
	}
	$effect(() => {
		void selectedId;
		void messages.length;
		if (pinned && scroller && pane === 'chat') tick().then(() => { if (scroller) scroller.scrollTop = scroller.scrollHeight; });
	});

	let adding = $state(false);
	// Which add form shows — one channel type at a time.
	let addKind = $state<'hashtag' | 'private'>('hashtag');
	const addKinds = [
		{ id: 'hashtag', label: '# Hashtag' },
		{ id: 'private', label: 'Private' }
	] as const;
	// Hashtag channels the server has confirmed on the mesh by decryption — a
	// one-tap way to add a real channel without knowing its name in advance.
	let discovered = $state<import('$lib/api').DiscoveredChannel[]>([]);
	let discoveredLoaded = $state(false);
	async function loadDiscovered() {
		try {
			const d = await api.discoveredChannels();
			discovered = d.channels ?? [];
		} catch {
			/* leave empty — the forms above still work */
		} finally {
			discoveredLoaded = true;
		}
	}
	// Names the user hasn't already added locally (compare on the '#'+name form).
	const discoverableNew = $derived(
		discovered.filter((d) => !channels.list.some((c) => c.name.toLowerCase() === ('#' + d.name).toLowerCase()))
	);
	function addDiscovered(name: string) {
		if (!channels.addHashtag(name)) { adding = false; open(channels.list[channels.list.length - 1].id); }
	}
	// Refresh the discovered list each time the Add panel opens, so a channel
	// confirmed while the page is open shows up (the endpoint is served from
	// memory, so this is free).
	$effect(() => {
		if (adding) loadDiscovered();
	});
	let hashtagName = $state('');
	let hashtagErr = $state<string | null>(null);
	// Default on: adding a hashtag channel also offers it to the shared discovery
	// list so the server tries it for everyone. Unticked keeps the add local.
	let shareToPool = $state(true);
	let privateName = $state('');
	let privateKey = $state('');
	let privateErr = $state<string | null>(null);
	function addHashtag() {
		const name = hashtagName;
		hashtagErr = channels.addHashtag(name);
		if (!hashtagErr) {
			// Best-effort: a failure never blocks the local add.
			if (shareToPool) api.submitChannelCandidate(name).catch(() => {});
			hashtagName = ''; adding = false; open(channels.list[channels.list.length - 1].id);
		}
	}
	function addPrivate() {
		privateErr = channels.addPrivate(privateName, privateKey);
		if (!privateErr) { privateName = ''; privateKey = ''; adding = false; open(channels.list[channels.list.length - 1].id); }
	}
	function removeChannel(id: string) {
		channels.remove(id);
		pane = 'list';
	}

	const typeColor: Record<Channel['type'], string> = { public: 'var(--color-signal)', hashtag: 'var(--color-amber)', private: 'var(--color-coral)' };
	const palette = ['var(--color-signal)', 'var(--color-amber)', '#7aa2f7', '#bb9af7', '#9ece6a', 'var(--color-coral)'];
	function senderColor(name: string): string {
		let h = 0;
		for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
		return palette[h % palette.length];
	}
	const input = 'w-full rounded-xl border border-line bg-panel px-3 py-2.5 font-mono text-sm text-fg placeholder:text-fg-faint focus:border-signal/60 focus:outline-none';
</script>

<Seo title="Channels" description="Decrypted MeshCore group chat." path="/m/channels" />

<div class="px-4 py-4">
	{#if pane === 'list'}
		<div class="mb-3 flex items-center justify-between">
			<span class="text-fg-faint font-mono text-[0.62rem]">{channels.list.length} channels · in this browser</span>
			<button onclick={() => (adding = !adding)} class="border-line text-fg-dim active:text-fg rounded-full border px-3 py-1 text-xs font-600">{adding ? 'Close' : '+ Add'}</button>
		</div>

		{#if adding}
			<div class="border-line/60 bg-panel mb-3 space-y-4 rounded-2xl border p-4">
				<div class="border-line flex rounded-full border p-0.5" role="tablist" aria-label="Channel type">
					{#each addKinds as k (k.id)}
						<button type="button" role="tab" aria-selected={addKind === k.id} onclick={() => (addKind = k.id)} class="flex-1 rounded-full py-1.5 text-xs font-600 {addKind === k.id ? 'bg-signal/15 text-signal' : 'text-fg-dim'}">{k.label}</button>
					{/each}
				</div>

				{#if addKind === 'hashtag'}
					<form class="space-y-2" onsubmit={(e) => { e.preventDefault(); addHashtag(); }}>
						<div class="flex items-center gap-2">
							<span class="text-fg-faint font-mono">#</span>
							<input bind:value={hashtagName} placeholder="name" class={input} oninput={() => (hashtagErr = null)} />
						</div>
						<button type="submit" class="border-signal/50 bg-signal/10 text-signal w-full rounded-xl border py-2.5 text-sm font-600 disabled:opacity-40" disabled={!hashtagName.trim()}>Add hashtag</button>
						<label class="text-fg-faint flex items-center gap-2 py-1 text-xs">
							<input type="checkbox" bind:checked={shareToPool} class="accent-signal h-4 w-4" />
							Share with the discovery list
						</label>
						{#if hashtagErr}<div class="text-coral text-xs">{hashtagErr}</div>{/if}
					</form>

					<div class="space-y-2">
						<div class="label">Discovered on the mesh</div>
						{#if !discoveredLoaded}
							<div class="text-fg-faint text-xs">Looking…</div>
						{:else if discoverableNew.length === 0}
							<div class="text-fg-faint text-xs">{discovered.length ? 'All discovered channels are already added.' : 'None confirmed yet.'}</div>
						{:else}
							<div class="flex flex-wrap gap-2">
								{#each discoverableNew as d (d.name)}
									<button type="button" onclick={() => addDiscovered(d.name)} class="border-line text-fg-dim active:border-signal/50 active:text-signal rounded-full border px-3 py-1.5 font-mono text-xs">+ #{d.name}</button>
								{/each}
							</div>
						{/if}
					</div>
				{:else}
					<form class="space-y-2" onsubmit={(e) => { e.preventDefault(); addPrivate(); }}>
						<input bind:value={privateName} placeholder="name" class={input} oninput={() => (privateErr = null)} />
						<input bind:value={privateKey} placeholder="32 hex key" spellcheck="false" autocomplete="off" class={input} oninput={() => (privateErr = null)} />
						<button type="submit" class="border-signal/50 bg-signal/10 text-signal w-full rounded-xl border py-2.5 text-sm font-600 disabled:opacity-40" disabled={!privateName.trim() || !privateKey.trim()}>Add private</button>
						{#if privateErr}<div class="text-coral text-xs">{privateErr}</div>{/if}
					</form>
				{/if}
			</div>
		{/if}

		<div class="border-line/60 bg-panel divide-line/50 divide-y overflow-hidden rounded-2xl border">
			{#if channels.list.length === 0}
				<div class="text-fg-faint px-4 py-8 text-center text-sm">No channels configured.</div>
			{:else}
				{#each channels.list as c (c.id)}
					{@const last = lastOf(c.id)}
					<button onclick={() => open(c.id)} class="active:bg-line/40 flex w-full flex-col gap-1 px-4 py-3 text-left">
						<div class="flex items-center gap-2">
							<span class="h-2 w-2 shrink-0 rounded-full" style="background:{typeColor[c.type]}"></span>
							<span class="font-display text-fg truncate text-sm font-700">{c.name}</span>
							{#if last}<span class="text-fg-faint ml-auto shrink-0 font-mono text-[0.6rem]">{ago(last.receivedAt)}</span>{/if}
						</div>
						<div class="text-fg-faint truncate pl-4 text-xs">
							{#if last}{last.sender ? `${last.sender}: ` : ''}{last.text}{:else}<span class="italic">no recent messages</span>{/if}
						</div>
					</button>
				{/each}
			{/if}
		</div>
		{#if !channels.hasPublic}
			<button onclick={() => channels.restorePublic()} class="text-fg-faint active:text-signal mt-3 w-full text-center text-xs">+ Restore Public channel</button>
		{/if}
	{:else if selected}
		<!-- chat header -->
		<div class="mb-3 flex items-center gap-2.5">
			<button onclick={() => (pane = 'list')} aria-label="Back" class="text-fg-dim active:text-fg -ml-1 p-1">
				<svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
			</button>
			<span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background:{typeColor[selected.type]}"></span>
			<h2 class="font-display text-fg min-w-0 flex-1 truncate text-lg font-700">{selected.name}</h2>
			<span class="text-fg-faint font-mono text-[0.62rem]"><span class="text-signal tnum">{messages.length}</span> msgs</span>
			{#if selected.type !== 'public' || !channels.hasPublic}
				<button onclick={() => removeChannel(selected.id)} class="text-fg-faint active:text-coral text-xs">remove</button>
			{/if}
		</div>

		<!-- messages -->
		<div bind:this={scroller} onscroll={onScroll} class="space-y-2.5 overflow-y-auto" style="height:calc(100dvh - 11rem)">
			{#if loadingHistory && messages.length === 0}
				<div class="text-fg-faint py-12 text-center text-sm">Loading messages…</div>
			{:else if messages.length === 0}
				<div class="text-fg-faint py-12 text-center text-sm">No messages on this channel in the last 6h.</div>
			{:else}
				{#each messages as m (m.hash)}
					<div class="border-line/60 bg-panel rounded-2xl border px-3.5 py-2.5">
						<div class="mb-0.5 flex items-baseline gap-2">
							<span class="font-display text-sm font-700" style="color:{m.sender ? senderColor(m.sender) : 'var(--color-fg-faint)'}">{m.sender || 'anon'}</span>
							<span class="text-fg-faint font-mono text-[0.6rem]">{ago(m.receivedAt)} ago</span>
							{#if m.observers > 1}<span class="text-fg-faint ml-auto font-mono text-[0.6rem]">×{m.observers}</span>{/if}
						</div>
						<div class="text-fg text-sm break-words whitespace-pre-wrap"><Linkify text={m.text} /></div>
					</div>
				{/each}
			{/if}
		</div>
	{/if}
</div>
