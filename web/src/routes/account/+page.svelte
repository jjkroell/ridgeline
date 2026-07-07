<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { claims, shares, type ClaimWithNode, type SharedWithMe } from '$lib/api';
	import { ago, shortKey } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';

	// The caller's node claims (pending + owned).
	let myNodes = $state<ClaimWithNode[]>([]);
	async function loadMyNodes() {
		try {
			myNodes = await claims.mine();
		} catch {
			/* leave empty */
		}
	}

	// Nodes shared WITH me (private locations others have granted me access to).
	let sharedWithMe = $state<SharedWithMe[]>([]);
	async function loadSharedWithMe() {
		try {
			sharedWithMe = await shares.mine();
			// Viewing the list clears the "new shares" badge.
			await auth.markSharesSeen();
		} catch {
			/* leave empty */
		}
	}

	// Redirect to sign-in if not authenticated (once the /me probe resolves).
	$effect(() => {
		if (auth.ready && !auth.loggedIn) goto('/login');
	});

	onMount(() => {
		loadMyNodes();
		loadSharedWithMe();
	});

	async function signOut() {
		await auth.logout();
		goto('/');
	}
</script>

<svelte:head><title>Account · Ridgeline</title></svelte:head>

<PageHeader eyebrow="Accounts" title="Your account">
	<button
		onclick={signOut}
		class="border-line text-fg-dim hover:border-coral/50 hover:text-coral rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors"
	>
		Sign out
	</button>
</PageHeader>

{#if auth.loggedIn}
	{@const u = auth.user}
	<div class="flex flex-col gap-6 px-6 pb-16 md:px-10">
		<!-- Profile -->
		<div class="panel px-6 py-6">
			<div class="flex items-center gap-4">
				<span
					class="bg-signal/15 text-signal flex h-14 w-14 shrink-0 items-center justify-center rounded-full text-xl font-700"
				>
					{(u?.displayName || u?.email || '?').charAt(0).toUpperCase()}
				</span>
				<div class="min-w-0">
					<div class="font-display text-fg truncate text-xl font-700">
						{u?.displayName || u?.email}
					</div>
					<div class="text-fg-dim truncate text-sm">{u?.email}</div>
				</div>
				<div class="ml-auto flex flex-wrap justify-end gap-2">
					{#if u?.isOwner}
						<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600">Owner</span>
					{:else if u?.isAdmin}
						<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600"
							>Admin</span
						>
					{/if}
				</div>
			</div>
			<div class="border-line/70 mt-5 grid grid-cols-2 gap-4 border-t pt-5 text-sm sm:grid-cols-3">
				<div>
					<div class="label mb-1">Joined</div>
					<div class="text-fg-dim">{ago(u?.createdAt)}</div>
				</div>
				<div>
					<div class="label mb-1">Last login</div>
					<div class="text-fg-dim">{u?.lastLogin ? ago(u.lastLogin) : '—'}</div>
				</div>
			</div>
		</div>

		<!-- My nodes -->
		{#if myNodes.length}
			<div class="panel overflow-hidden">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
					<span class="font-display text-fg text-sm font-700">My nodes</span>
					<span class="text-fg-faint text-xs">{myNodes.length}</span>
				</div>
				<div class="divide-line/60 divide-y">
					{#each myNodes as c (c.id)}
						<a
							href="/nodes/{c.nodePubkey}"
							class="panel-hover flex items-center gap-3 px-5 py-3"
						>
							<span class="min-w-0 flex-1">
								<span class="text-fg block truncate text-sm font-600"
									>{c.nodeName || shortKey(c.nodePubkey)}</span
								>
								<span class="text-fg-faint block truncate font-mono text-xs">{shortKey(c.nodePubkey, 6, 4)}</span>
							</span>
							{#if c.status === 'verified'}
								<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600"
									>Owned</span
								>
							{:else}
								<span class="bg-amber/15 text-amber rounded-full px-2.5 py-1 text-xs font-600"
									>Pending</span
								>
							{/if}
						</a>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Shared with me -->
		{#if sharedWithMe.length}
			<div class="panel overflow-hidden">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
					<span class="font-display text-fg text-sm font-700">Shared with me</span>
					<span class="text-fg-faint text-xs">{sharedWithMe.length}</span>
				</div>
				<div class="divide-line/60 divide-y">
					{#each sharedWithMe as sh (sh.nodePubkey)}
						<a href="/nodes/{sh.nodePubkey}" class="panel-hover flex items-center gap-3 px-5 py-3">
							<span
								class="bg-signal/15 text-signal grid h-8 w-8 shrink-0 place-items-center rounded-full"
							>
								<svg
									viewBox="0 0 24 24"
									class="h-4 w-4"
									fill="none"
									stroke="currentColor"
									stroke-width="1.6"
									stroke-linecap="round"
									stroke-linejoin="round"
									><path d="M12 21s-7-4.35-7-11a7 7 0 0 1 14 0c0 6.65-7 11-7 11z" /><circle
										cx="12"
										cy="10"
										r="2.5"
									/></svg
								>
							</span>
							<span class="min-w-0 flex-1">
								<span class="text-fg block truncate text-sm font-600"
									>{sh.nodeName || shortKey(sh.nodePubkey)}</span
								>
								<span class="text-fg-faint block truncate text-xs"
									>Shared by {sh.sharedByName} · {ago(sh.createdAt)}</span
								>
							</span>
							{#if !sh.seen}
								<span class="bg-signal text-ink rounded-full px-2.5 py-1 text-xs font-700">New</span>
							{/if}
						</a>
					{/each}
				</div>
			</div>
		{/if}

	</div>
{/if}
