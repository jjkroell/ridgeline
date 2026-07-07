<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import {
		adminUsers,
		claims,
		shares,
		type AuthUser,
		type ClaimWithNode,
		type SharedWithMe
	} from '$lib/api';
	import { ago, shortKey } from '$lib/format';

	let myNodes = $state<ClaimWithNode[]>([]);
	async function loadMyNodes() {
		try {
			myNodes = await claims.mine();
		} catch {
			/* leave empty */
		}
	}

	let sharedWithMe = $state<SharedWithMe[]>([]);
	async function loadSharedWithMe() {
		try {
			sharedWithMe = await shares.mine();
			await auth.markSharesSeen();
		} catch {
			/* leave empty */
		}
	}

	// --- Admin: member management (parity with desktop) ---
	let members = $state<AuthUser[]>([]);
	let loadingMembers = $state(false);
	let membersError = $state('');
	let busyId = $state<number | null>(null);

	async function loadMembers() {
		if (!auth.isAdmin) return;
		loadingMembers = true;
		membersError = '';
		try {
			members = await adminUsers.list();
		} catch (e) {
			membersError = String((e as Error).message ?? e);
		} finally {
			loadingMembers = false;
		}
	}

	let confirmDeleteId = $state<number | null>(null);

	async function setFlags(u: AuthUser, isAdmin: boolean, canClaim: boolean) {
		busyId = u.id;
		membersError = '';
		try {
			await adminUsers.setFlags(auth.csrf, u.id, isAdmin, canClaim);
			await loadMembers();
		} catch (e) {
			membersError = String((e as Error).message ?? e);
		} finally {
			busyId = null;
		}
	}

	async function setBlocked(u: AuthUser, blocked: boolean) {
		busyId = u.id;
		membersError = '';
		try {
			await adminUsers.setBlocked(auth.csrf, u.id, blocked);
			await loadMembers();
		} catch (e) {
			membersError = String((e as Error).message ?? e);
		} finally {
			busyId = null;
		}
	}

	async function removeUser(u: AuthUser) {
		busyId = u.id;
		membersError = '';
		try {
			await adminUsers.remove(auth.csrf, u.id);
			confirmDeleteId = null;
			await loadMembers();
		} catch (e) {
			membersError = String((e as Error).message ?? e);
		} finally {
			busyId = null;
		}
	}

	onMount(() => {
		loadMembers();
		loadMyNodes();
		loadSharedWithMe();
	});
	$effect(() => {
		if (auth.isAdmin && members.length === 0 && !loadingMembers) loadMembers();
	});

	async function signOut() {
		await auth.logout();
		goto('/m');
	}
</script>

<svelte:head><title>Account · Ridgeline</title></svelte:head>

{#if !auth.ready}
	<div class="text-fg-faint px-4 py-16 text-center text-sm">Loading…</div>
{:else if !auth.loggedIn}
	<!-- Signed-out prompt -->
	<div class="px-4 py-10 text-center">
		<span
			class="bg-signal/15 text-signal mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full"
		>
			<svg
				viewBox="0 0 24 24"
				class="h-7 w-7"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"
				stroke-linejoin="round"><path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg
			>
		</span>
		<h2 class="font-display text-fg text-lg font-700">You're signed out</h2>
		<p class="text-fg-dim mx-auto mt-2 max-w-xs text-sm leading-relaxed">
			Sign in to claim your nodes, add notes, and manage private locations.
		</p>
		<a
			href="/m/login"
			class="bg-signal/15 text-signal border-signal/40 active:bg-signal/25 mt-6 inline-block rounded-xl border px-6 py-3 text-sm font-700"
			>Sign in or register</a
		>
	</div>
{:else}
	{@const u = auth.user}
	<div class="flex flex-col gap-4 px-4 py-5">
		<!-- Profile -->
		<div class="panel px-4 py-4">
			<div class="flex items-center gap-3">
				<span
					class="bg-signal/15 text-signal flex h-12 w-12 shrink-0 items-center justify-center rounded-full text-lg font-700"
					>{(u?.displayName || u?.email || '?').charAt(0).toUpperCase()}</span
				>
				<div class="min-w-0">
					<div class="font-display text-fg truncate text-base font-700">
						{u?.displayName || u?.email}
					</div>
					<div class="text-fg-dim truncate text-xs">{u?.email}</div>
				</div>
			</div>
			<div class="mt-3 flex flex-wrap gap-2">
				{#if u?.isOwner}
					<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600">Owner</span>
				{:else if u?.isAdmin}
					<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600">Admin</span>
				{/if}
				{#if u?.canClaim}
					<span class="bg-amber/15 text-amber rounded-full px-2.5 py-1 text-xs font-600"
						>Can claim nodes</span
					>
				{/if}
				<span class="text-fg-faint self-center text-xs">joined {ago(u?.createdAt)}</span>
			</div>
		</div>

		<!-- Claim status -->
		<div class="panel px-4 py-4">
			<div class="label normal-case text-fg-dim mb-2">Node claiming & private locations</div>
			<p class="text-fg-dim text-sm leading-relaxed">
				{#if auth.canClaim}
					You're approved to claim nodes — open any node and tap <strong class="text-fg"
						>Claim this node</strong
					>. Notes and private locations are coming next.
				{:else}
					Claiming nodes and storing private locations requires admin approval. Until then you can
					browse everything.
				{/if}
			</p>
		</div>

		<!-- My nodes -->
		{#if myNodes.length}
			<div class="panel overflow-hidden">
				<div class="border-line/70 flex items-center gap-2 border-b px-4 py-3">
					<span class="font-display text-fg text-sm font-700">My nodes</span>
					<span class="text-fg-faint text-xs">{myNodes.length}</span>
				</div>
				<div class="divide-line/60 divide-y">
					{#each myNodes as c (c.id)}
						<a href="/m/nodes/{c.nodePubkey}" class="active:bg-line/40 flex items-center gap-3 px-4 py-3">
							<span class="min-w-0 flex-1">
								<span class="text-fg block truncate text-sm font-600"
									>{c.nodeName || shortKey(c.nodePubkey)}</span
								>
								<span class="text-fg-faint block truncate font-mono text-xs"
									>{shortKey(c.nodePubkey, 6, 4)}</span
								>
							</span>
							{#if c.status === 'verified'}
								<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600">Owned</span>
							{:else}
								<span class="bg-amber/15 text-amber rounded-full px-2.5 py-1 text-xs font-600">Pending</span>
							{/if}
						</a>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Shared with me -->
		{#if sharedWithMe.length}
			<div class="panel overflow-hidden">
				<div class="border-line/70 flex items-center gap-2 border-b px-4 py-3">
					<span class="font-display text-fg text-sm font-700">Shared with me</span>
					<span class="text-fg-faint text-xs">{sharedWithMe.length}</span>
				</div>
				<div class="divide-line/60 divide-y">
					{#each sharedWithMe as sh (sh.nodePubkey)}
						<a
							href="/m/nodes/{sh.nodePubkey}"
							class="active:bg-line/40 flex items-center gap-3 px-4 py-3"
						>
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

		<!-- Admin: members -->
		{#if auth.isAdmin}
			<div class="panel overflow-hidden">
				<div class="border-line/70 flex items-center gap-2 border-b px-4 py-3">
					<span class="font-display text-fg text-sm font-700">Members</span>
					<span class="text-fg-faint text-xs">{members.length}</span>
					<button onclick={loadMembers} class="label active:text-signal ml-auto">Refresh</button>
				</div>
				{#if membersError}<div class="text-coral px-4 py-2 text-xs">{membersError}</div>{/if}
				<div class="divide-line/60 divide-y">
					{#each members as m (m.id)}
						{@const self = m.id === auth.user?.id}
						<div class="px-4 py-3 {m.blocked ? 'opacity-60' : ''}">
							<div class="flex items-center gap-2">
								<span class="text-fg truncate text-sm font-600">{m.displayName || m.email}</span>
								{#if m.isOwner}
									<span class="bg-signal/15 text-signal rounded-full px-2 py-0.5 text-[0.6rem] font-600"
										>Owner</span
									>
								{/if}
								{#if m.blocked}
									<span class="bg-coral/15 text-coral rounded-full px-2 py-0.5 text-[0.6rem] font-600"
										>Blocked</span
									>
								{/if}
							</div>
							<div class="text-fg-faint mb-2 truncate text-xs">{m.email}</div>
							<div class="flex flex-wrap items-center gap-4">
								<label class="text-fg-dim flex items-center gap-1.5 text-xs">
									<input
										type="checkbox"
										checked={m.canClaim}
										disabled={busyId === m.id}
										onchange={(e) => setFlags(m, m.isAdmin, e.currentTarget.checked)}
										class="accent-signal h-4 w-4"
									/> Can claim
								</label>
								<label class="text-fg-dim flex items-center gap-1.5 text-xs">
									<input
										type="checkbox"
										checked={m.isAdmin}
										disabled={busyId === m.id || self || m.isOwner}
										onchange={(e) => setFlags(m, e.currentTarget.checked, m.canClaim)}
										class="accent-signal h-4 w-4"
									/> Admin
								</label>
								{#if !m.isOwner && !self}
									<button
										onclick={() => setBlocked(m, !m.blocked)}
										disabled={busyId === m.id}
										class="text-xs font-600 disabled:opacity-50 {m.blocked ? 'text-signal' : 'text-amber'}"
										>{m.blocked ? 'Unblock' : 'Block'}</button
									>
									{#if confirmDeleteId === m.id}
										<button onclick={() => removeUser(m)} class="text-coral text-xs font-700"
											>Confirm</button
										>
										<button onclick={() => (confirmDeleteId = null)} class="text-fg-faint text-xs"
											>Cancel</button
										>
									{:else}
										<button onclick={() => (confirmDeleteId = m.id)} class="text-coral/80 text-xs font-600"
											>Remove</button
										>
									{/if}
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<button
			onclick={signOut}
			class="border-line text-fg-dim active:border-coral/50 active:text-coral w-full rounded-xl border px-4 py-3 text-sm font-600"
			>Sign out</button
		>
	</div>
{/if}
