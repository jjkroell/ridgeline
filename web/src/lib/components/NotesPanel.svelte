<script lang="ts">
	import { notes, type Note } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { ago } from '$lib/format';

	interface Props {
		pubkey: string;
		compact?: boolean;
	}
	let { pubkey, compact = false }: Props = $props();

	let list = $state<Note[]>([]);
	let loading = $state(true);
	let error = $state('');

	// New-note composer.
	let body = $state('');
	let visibility = $state<'public' | 'private'>('public');
	let posting = $state(false);

	// Inline edit state.
	let editingId = $state<number | null>(null);
	let editBody = $state('');
	let editVis = $state<'public' | 'private'>('public');

	let loadedFor = $state('');
	$effect(() => {
		if (pubkey && pubkey !== loadedFor) {
			loadedFor = pubkey;
			load();
		}
	});

	async function load() {
		loading = true;
		try {
			list = await notes.list(pubkey);
			error = '';
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			loading = false;
		}
	}

	async function post() {
		if (!body.trim()) return;
		posting = true;
		error = '';
		try {
			await notes.create(auth.csrf, pubkey, body.trim(), visibility);
			body = '';
			visibility = 'public';
			await load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			posting = false;
		}
	}

	function startEdit(n: Note) {
		editingId = n.id;
		editBody = n.body;
		editVis = n.visibility;
	}

	async function saveEdit(n: Note) {
		if (!editBody.trim()) return;
		try {
			await notes.update(auth.csrf, n.id, editBody.trim(), editVis);
			editingId = null;
			await load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		}
	}

	async function remove(n: Note) {
		try {
			await notes.remove(auth.csrf, n.id);
			await load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		}
	}
</script>

<div class="panel {compact ? 'px-4 py-4' : 'px-5 py-5'}">
	<div class="label normal-case text-fg-dim mb-3 flex items-center gap-2">
		<svg
			viewBox="0 0 24 24"
			class="text-fg-faint h-4 w-4"
			fill="none"
			stroke="currentColor"
			stroke-width="1.6"
			stroke-linecap="round"
			stroke-linejoin="round"
			><path d="M4 4h16v12H7l-3 3zM8 9h8M8 12h5" /></svg
		>
		Notes
		{#if list.length}<span class="text-fg-faint">{list.length}</span>{/if}
	</div>

	<!-- Composer -->
	{#if auth.loggedIn}
		<div class="border-line/70 mb-4 rounded-[var(--radius)] border p-3">
			<textarea
				bind:value={body}
				rows={compact ? 2 : 3}
				maxlength="4000"
				placeholder="Add a note about this node…"
				class="bg-ink-2 text-fg focus:border-signal w-full resize-y rounded-[var(--radius)] border border-transparent px-3 py-2 text-sm outline-none"
			></textarea>
			<div class="mt-2 flex items-center gap-2">
				<div class="border-line/70 flex gap-0.5 rounded-full border p-0.5 text-xs">
					<button
						onclick={() => (visibility = 'public')}
						class="rounded-full px-2.5 py-1 font-600 transition-colors {visibility === 'public'
							? 'bg-signal/15 text-signal'
							: 'text-fg-dim'}">Public</button
					>
					<button
						onclick={() => (visibility = 'private')}
						class="rounded-full px-2.5 py-1 font-600 transition-colors {visibility === 'private'
							? 'bg-amber/15 text-amber'
							: 'text-fg-dim'}">Private</button
					>
				</div>
				<span class="text-fg-faint text-xs"
					>{visibility === 'public' ? 'Visible to everyone' : 'Only you can see this'}</span
				>
				<button
					onclick={post}
					disabled={posting || !body.trim()}
					class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 ml-auto rounded-[var(--radius)] border px-3.5 py-1.5 text-sm font-600 transition-colors disabled:opacity-40"
					>{posting ? 'Posting…' : 'Post'}</button
				>
			</div>
		</div>
	{:else}
		<p class="text-fg-dim mb-4 text-sm">
			<a href="/login" class="text-signal hover:underline">Sign in</a> to add notes.
		</p>
	{/if}

	{#if error}<p class="text-coral mb-3 text-xs">{error}</p>{/if}

	<!-- List -->
	{#if loading}
		<p class="text-fg-faint text-sm">Loading…</p>
	{:else if list.length === 0}
		<p class="text-fg-faint text-sm">No notes yet.</p>
	{:else}
		<div class="space-y-3">
			{#each list as n (n.id)}
				<div class="border-line/60 rounded-[var(--radius)] border px-3.5 py-3">
					<div class="mb-1.5 flex items-center gap-2">
						<span class="text-fg text-xs font-600">{n.authorName}</span>
						{#if n.visibility === 'private'}
							<span class="bg-amber/15 text-amber rounded-full px-2 py-0.5 text-[0.6rem] font-600"
								>Private</span
							>
						{/if}
						<span class="text-fg-faint ml-auto text-[0.68rem]">{ago(n.createdAt)}</span>
					</div>
					{#if editingId === n.id}
						<textarea
							bind:value={editBody}
							rows="3"
							maxlength="4000"
							class="bg-ink-2 text-fg focus:border-signal w-full resize-y rounded-[var(--radius)] border border-transparent px-3 py-2 text-sm outline-none"
						></textarea>
						<div class="mt-2 flex items-center gap-2">
							<div class="border-line/70 flex gap-0.5 rounded-full border p-0.5 text-xs">
								<button
									onclick={() => (editVis = 'public')}
									class="rounded-full px-2 py-0.5 {editVis === 'public'
										? 'bg-signal/15 text-signal'
										: 'text-fg-dim'}">Public</button
								>
								<button
									onclick={() => (editVis = 'private')}
									class="rounded-full px-2 py-0.5 {editVis === 'private'
										? 'bg-amber/15 text-amber'
										: 'text-fg-dim'}">Private</button
								>
							</div>
							<button onclick={() => saveEdit(n)} class="text-signal ml-auto text-xs font-600"
								>Save</button
							>
							<button onclick={() => (editingId = null)} class="text-fg-faint text-xs">Cancel</button>
						</div>
					{:else}
						<p class="text-fg-dim text-sm leading-relaxed whitespace-pre-wrap">{n.body}</p>
						{#if n.mine}
							<div class="mt-2 flex gap-3">
								{#if n.userId === auth.user?.id}
									<button onclick={() => startEdit(n)} class="text-fg-faint hover:text-fg text-xs"
										>Edit</button
									>
								{/if}
								<button onclick={() => remove(n)} class="text-fg-faint hover:text-coral text-xs"
									>Delete</button
								>
							</div>
						{/if}
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
