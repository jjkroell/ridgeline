<!--
  "Are you monitoring #sos?" Yes / No: whether the signed-in account is on the
  public #sos monitoring list, and with which radio: the server lists only a
  companion it has heard, as that radio advertises its name. Typing filters
  the companions heard, to pick from. In the monitor list card,
  the #sos guide and Account settings. Nothing is sent on the mesh.
-->
<script lang="ts">
	import { tick } from 'svelte';
	import { api } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { sos } from '$lib/sos.svelte';

	let { mobile = false }: { mobile?: boolean } = $props();
	const m = $derived(mobile ? '/m' : '');
	const uid = $props.id();

	const on = $derived(auth.user?.sosMonitor ?? false);
	const saved = $derived(auth.user?.sosMonitorName ?? '');
	// What's being typed; null while it's just the saved radio.
	let typed = $state<string | null>(null);
	const name = $derived(typed ?? saved);
	const renamed = $derived(on && name.trim() !== '' && name.trim() !== auth.user?.sosMonitorName);
	let busy = $state(false);
	let msg = $state('');
	let failed = $state(false);

	// The companions heard, to filter as the name is typed. Fetched when the box
	// is entered, and again if that was a while ago, so a radio that has just
	// sent its advert turns up.
	let companions = $state<string[] | null>(null);
	let fetchedAt = 0;
	let open = $state(false);
	let active = $state(-1);
	async function loadCompanions() {
		if (Date.now() - fetchedAt < 15_000) return;
		fetchedAt = Date.now();
		try {
			const names = (await api.nodes())
				.filter((n) => n.role === 'ChatNode' && !n.retiredAt)
				.map((n) => (n.name ?? '').trim());
			companions = [...new Set(names)]
				.filter(Boolean)
				.sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
		} catch {
			fetchedAt = 0; // typing still works; the server checks the name
		}
	}
	// Letters and digits only, as the server compares names (store.plainName).
	const plain = (t: string) => t.toLowerCase().replace(/[^\p{L}\p{Nd}]/gu, '');
	const matches = $derived.by(() => {
		const q = (typed ?? '').trim().toLowerCase();
		if (!q || !companions) return [];
		const pq = plain(q);
		const hit = companions.filter((c) => c.toLowerCase().includes(q) || (pq !== '' && plain(c).includes(pq)));
		const starts = (c: string) => c.toLowerCase().startsWith(q) || (pq !== '' && plain(c).startsWith(pq));
		return [...hit.filter(starts), ...hit.filter((c) => !starts(c))];
	});
	const listed = $derived(open && companions !== null && (typed ?? '').trim() !== '');
	function pick(c: string) {
		typed = c;
		open = false;
		active = -1;
		msg = '';
	}
	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && listed) {
			e.stopPropagation();
			open = false;
			active = -1;
		} else if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && matches.length) {
			e.preventDefault();
			open = true;
			const n = matches.length;
			active = e.key === 'ArrowDown' ? (active + 1) % n : (active <= 0 ? n : active) - 1;
			tick().then(() => document.getElementById(`${uid}-o${active}`)?.scrollIntoView({ block: 'nearest' }));
		} else if (e.key === 'Enter' && listed && active >= 0 && matches[active]) {
			e.preventDefault();
			pick(matches[active]);
		}
	}

	async function save(enabled: boolean) {
		if (busy) return;
		failed = false;
		msg = '';
		open = false;
		active = -1;
		if (enabled && !name.trim()) {
			failed = true;
			msg = 'Enter your radio’s name first.';
			document.getElementById(`${uid}-name`)?.focus();
			return;
		}
		busy = true;
		try {
			await auth.setSOSMonitor(enabled, name.trim());
			typed = null;
			msg = enabled
				? `You’re on the list as ${auth.user?.sosMonitorName}.`
				: 'You’re not on the list.';
			sos.load();
		} catch (e) {
			failed = true;
			msg = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}
</script>

{#snippet choice(value: boolean, label: string)}
	<button
		type="button"
		role="radio"
		aria-checked={on === value}
		disabled={busy || !auth.loggedIn}
		onclick={() => (on === value ? undefined : save(value))}
		class="min-w-14 rounded-[calc(var(--radius)-2px)] px-3 text-sm font-600 transition-colors disabled:cursor-not-allowed {mobile
			? 'py-2'
			: 'py-1'} {on === value && auth.loggedIn
			? value
				? 'bg-coral text-ink'
				: 'bg-panel-2 text-fg'
			: 'text-fg-dim hover:text-fg'}">{label}</button
	>
{/snippet}

<div data-sos-monitor-toggle>
	<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
		<span class="text-fg text-sm font-600" id="{uid}-q">Are you monitoring #sos?</span>
		<div
			role="radiogroup"
			aria-labelledby="{uid}-q"
			class="border-line inline-flex gap-0.5 rounded-[var(--radius)] border p-0.5 {auth.loggedIn ? '' : 'opacity-50'}"
		>
			{@render choice(true, 'Yes')}
			{@render choice(false, 'No')}
		</div>
		{#if auth.ready && !auth.loggedIn}
			<span class="text-fg-dim text-xs"
				><a href="{m}/login" class="text-coral hover:underline">Sign in</a> to answer.</span
			>
		{/if}
	</div>
	{#if auth.loggedIn}
		<form
			class="mt-2 flex flex-wrap items-center gap-2"
			onsubmit={(e) => {
				e.preventDefault();
				if (!on || renamed) save(true);
			}}
		>
			<label class="text-fg-dim text-xs" for="{uid}-name">Radio name</label>
			<input
				id="{uid}-name"
				value={name}
				oninput={(e) => {
					typed = e.currentTarget.value;
					open = true;
					active = -1;
					loadCompanions();
				}}
				onfocus={(e) => {
					// Typing replaces the name already there.
					e.currentTarget.select();
					loadCompanions();
				}}
				onblur={() => (open = false)}
				{onkeydown}
				role="combobox"
				aria-expanded={listed}
				aria-controls="{uid}-list"
				aria-autocomplete="list"
				aria-activedescendant={listed && active >= 0 ? `${uid}-o${active}` : undefined}
				maxlength="64"
				placeholder="As in your MeshCore app"
				aria-describedby="{uid}-hint"
				autocomplete="off"
				autocapitalize="off"
				spellcheck="false"
				class="border-line bg-ink-2 text-fg focus:border-coral w-48 max-w-full min-w-0 rounded-[var(--radius)] border px-2.5 text-sm outline-none {mobile
					? 'py-2'
					: 'py-1'}"
			/>
			{#if renamed}
				<button
					type="submit"
					disabled={busy}
					class="text-coral text-xs font-600 hover:underline disabled:opacity-50">Save</button
				>
			{/if}
		</form>
		{#if listed && matches.length}
			<!-- In the flow, not floating: the cards around it clip what overflows. -->
			<ul
				id="{uid}-list"
				role="listbox"
				aria-label="Companions heard"
				class="border-line bg-ink-2 mt-1.5 max-h-48 max-w-sm overflow-y-auto rounded-[var(--radius)] border py-1"
			>
				{#each matches as c, i (c)}
					<!-- Arrow keys and Enter work from the input, which keeps focus. -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<li
						id="{uid}-o{i}"
						role="option"
						aria-selected={i === active}
						onmousedown={(e) => e.preventDefault()}
						onclick={() => pick(c)}
						class="cursor-pointer truncate px-2.5 text-sm {mobile ? 'py-2' : 'py-1'} {i === active
							? 'bg-coral/15 text-fg'
							: 'text-fg-dim hover:bg-panel-2 hover:text-fg'}"
					>
						{c}
					</li>
				{/each}
			</ul>
		{:else if listed}
			<p class="text-amber mt-1.5 text-xs leading-relaxed">
				Ridgeline hasn’t heard a companion matching that. Send a flood advert from your radio, wait a minute and try
				again.
			</p>
		{/if}
		<p class="text-fg-faint mt-1.5 text-xs leading-relaxed" id="{uid}-hint">
			Start typing your companion radio’s name and pick it. <span class="text-fg-dim font-600"
				>You only need to enter one companion</span
			>, even if you have several. It’s shown on the public list. Being listed isn’t a commitment to respond.
		</p>
	{/if}
	<p class="mt-1 text-xs {failed ? 'text-amber' : 'text-signal'} empty:hidden" role="status" aria-live="polite">{msg}</p>
</div>
