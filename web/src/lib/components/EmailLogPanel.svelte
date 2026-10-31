<script lang="ts">
	// Every email Ridgeline handed to the SMTP relay (#sos alerts, verification,
	// resets, note notifications, contact form), newest first, with the relay's
	// refusal when it didn't take one. Shared by /admin and /m/admin like
	// MembersPanel; `compact` switches to the mobile card chrome.
	import { auth } from '$lib/auth.svelte';
	import { adminEmailLog, type EmailLogEntry } from '$lib/api';
	import { ago } from '$lib/format';

	let { compact = false }: { compact?: boolean } = $props();

	// Collapsed by default and remembered per browser, like Members.
	const OPEN_KEY = 'ridgeline-admin-email-log-open';
	let open = $state(false);
	$effect(() => {
		if (typeof localStorage === 'undefined') return;
		try {
			open = localStorage.getItem(OPEN_KEY) === '1';
		} catch {
			/* storage blocked: stay collapsed */
		}
	});
	function toggle() {
		open = !open;
		try {
			localStorage.setItem(OPEN_KEY, open ? '1' : '0');
		} catch {
			/* not remembered, still toggles */
		}
	}

	const shell = $derived(
		compact
			? 'border-line/60 bg-panel mb-3 rounded-2xl border overflow-hidden'
			: 'panel rise mt-6 overflow-hidden'
	);
	const head = $derived(compact ? 'px-4 py-3' : 'px-5 py-3.5');
	const pad = $derived(compact ? 'px-4' : 'px-5');

	const KINDS: { id: string; label: string }[] = [
		{ id: '', label: 'All' },
		{ id: 'sos-alert', label: '#sos alerts' },
		{ id: 'verification', label: 'Verification' },
		{ id: 'password-reset', label: 'Password reset' },
		{ id: 'note', label: 'Notes' },
		{ id: 'contact', label: 'Contact' }
	];
	const kindLabel = (id: string) => KINDS.find((k) => k.id === id)?.label ?? id;

	let kind = $state('');
	let q = $state('');
	let entries = $state<EmailLogEntry[]>([]);
	let more = $state(false);
	let loading = $state(false);
	let error = $state('');
	let loaded = $state(false);

	async function load(append = false) {
		if (!auth.isAdmin) return;
		loading = true;
		error = '';
		try {
			const before = append ? entries[entries.length - 1]?.id : undefined;
			const r = await adminEmailLog.list({ kind, q: q.trim(), before });
			entries = append ? [...entries, ...r.entries] : r.entries;
			more = r.more;
			loaded = true;
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			loading = false;
		}
	}

	// Fetch on first open, not on page load: most visits never look at it.
	$effect(() => {
		if (open && auth.isAdmin && !loaded && !loading) load();
	});

	function setKind(id: string) {
		kind = id;
		load();
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function onSearch() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => load(), 300);
	}

	/** "Fri Oct 30, 10:51 PM" in the viewer's own zone. */
	function when(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return d.toLocaleString(undefined, {
			weekday: 'short',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit'
		});
	}

	const refused = $derived(entries.filter((e) => e.error).length);
</script>

<section class={shell}>
	<div class="border-line/70 flex items-center gap-2.5 {open ? 'border-b' : ''} {head}">
		<button
			onclick={toggle}
			aria-expanded={open}
			class="hover:text-signal -m-1 flex min-w-0 flex-1 items-center gap-2.5 p-1 text-left transition-colors"
		>
			<span
				class="text-fg-faint inline-block shrink-0 text-[0.7rem] transition-transform duration-150"
				style="transform: rotate({open ? 90 : 0}deg)">▶</span
			>
			<h2 class="font-display text-fg font-700 tracking-wide {compact ? 'text-xs' : 'text-sm'}">
				EMAIL LOG
			</h2>
			{#if loaded}
				<span class="label normal-case text-fg-faint"
					>{entries.length}{more ? '+' : ''} shown</span
				>
				{#if refused}
					<span class="label text-coral! normal-case">· {refused} refused</span>
				{/if}
			{/if}
		</button>
		{#if open}
			<button
				onclick={() => load()}
				class="label hover:text-signal shrink-0 transition-colors"
				disabled={loading}>{loading ? 'Loading…' : 'Refresh'}</button
			>
		{/if}
	</div>
	{#if open}
		<div class="border-line/60 flex flex-wrap items-center gap-2 border-b py-3 {pad}">
			<div class="flex flex-wrap gap-1.5">
				{#each KINDS as k (k.id)}
					<button
						onclick={() => setKind(k.id)}
						class="rounded-full border px-2.5 py-1 text-xs transition-colors {kind === k.id
							? 'border-signal/50 bg-signal/15 text-signal'
							: 'border-line/70 text-fg-dim hover:text-fg'}">{k.label}</button
					>
				{/each}
			</div>
			<input
				type="search"
				bind:value={q}
				oninput={onSearch}
				placeholder="Search recipient"
				aria-label="Search by recipient"
				class="border-line bg-ink-2 text-fg placeholder:text-fg-faint focus:border-signal min-w-0 flex-1 rounded-[var(--radius)] border px-3 py-1.5 text-sm outline-none sm:max-w-64"
			/>
		</div>
		{#if error}
			<div class="text-coral py-3 text-xs {pad}">{error}</div>
		{/if}
		<div class="divide-line/60 divide-y">
			{#each entries as e (e.id)}
				<div class="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 py-2.5 {pad}">
					<span class="text-fg-dim w-40 shrink-0 text-xs tabular-nums" title={new Date(e.sentAt).toLocaleString()}
						>{when(e.sentAt)} <span class="text-fg-faint">· {ago(e.sentAt)}</span></span
					>
					<span
						class="shrink-0 rounded-full px-2 py-0.5 text-[0.62rem] font-600 {e.kind === 'sos-alert'
							? 'bg-coral/15 text-coral'
							: 'bg-line/60 text-fg-dim'}">{kindLabel(e.kind)}</span
					>
					<span class="text-fg min-w-0 truncate text-sm">{e.recipient}</span>
					{#if e.error}
						<span class="bg-coral/15 text-coral rounded-full px-2 py-0.5 text-[0.62rem] font-600"
							>Refused</span
						>
					{/if}
					<span class="text-fg-faint basis-full truncate text-xs sm:basis-auto sm:flex-1">{e.subject}</span>
					{#if e.error}
						<span class="text-coral basis-full text-xs break-words">{e.error}</span>
					{/if}
				</div>
			{/each}
			{#if loaded && entries.length === 0 && !loading}
				<div class="text-fg-faint py-6 text-center text-sm {pad}">
					{kind || q.trim() ? 'No emails match.' : 'No emails sent yet. The log started on Oct 31, 2026.'}
				</div>
			{/if}
		</div>
		{#if more}
			<div class="border-line/60 border-t py-2.5 text-center {pad}">
				<button
					onclick={() => load(true)}
					disabled={loading}
					class="label hover:text-signal transition-colors">{loading ? 'Loading…' : 'Load older'}</button
				>
			</div>
		{/if}
	{/if}
</section>
