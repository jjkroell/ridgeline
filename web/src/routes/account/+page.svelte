<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { adminUsers, claims, type AuthUser, type ClaimWithNode } from '$lib/api';
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

	// Redirect to sign-in if not authenticated (once the /me probe resolves).
	$effect(() => {
		if (auth.ready && !auth.loggedIn) goto('/login');
	});

	// --- Admin: member management ---
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
	});
	// Re-load if admin status settles after the initial probe.
	$effect(() => {
		if (auth.isAdmin && members.length === 0 && !loadingMembers) loadMembers();
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
					{#if u?.canClaim}
						<span class="bg-amber/15 text-amber rounded-full px-2.5 py-1 text-xs font-600"
							>Can claim nodes</span
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

		<!-- Claim status -->
		<div class="panel px-6 py-5">
			<div class="label normal-case text-fg-dim mb-2">Node claiming & private locations</div>
			{#if auth.canClaim}
				<p class="text-fg-dim text-sm leading-relaxed">
					You're approved to claim nodes. Open any node's page and use <strong class="text-fg"
						>Claim this node</strong
					> to prove control of it and add it to your account. Public/private notes and private exact
					locations are coming next.
				</p>
			{:else}
				<p class="text-fg-dim text-sm leading-relaxed">
					Claiming nodes and storing private locations requires admin approval. An admin can grant
					your account access; until then you can browse everything on the site.
				</p>
			{/if}
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

		<!-- Admin: members -->
		{#if auth.isAdmin}
			<div class="panel overflow-hidden">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
					<span class="font-display text-fg text-sm font-700">Members</span>
					<span class="text-fg-faint text-xs">{members.length} registered</span>
					<button
						onclick={loadMembers}
						class="label hover:text-signal ml-auto transition-colors"
						disabled={loadingMembers}>{loadingMembers ? 'Loading…' : 'Refresh'}</button
					>
				</div>
				{#if membersError}
					<div class="text-coral px-5 py-3 text-xs">{membersError}</div>
				{/if}
				<div class="divide-line/60 divide-y">
					{#each members as m (m.id)}
						{@const self = m.id === auth.user?.id}
						<div class="flex flex-wrap items-center gap-3 px-5 py-3 {m.blocked ? 'opacity-60' : ''}">
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2">
									<span class="text-fg truncate text-sm font-600">{m.displayName || m.email}</span>
									{#if m.isOwner}
										<span class="bg-signal/15 text-signal rounded-full px-2 py-0.5 text-[0.62rem] font-600"
											>Owner</span
										>
									{/if}
									{#if m.blocked}
										<span class="bg-coral/15 text-coral rounded-full px-2 py-0.5 text-[0.62rem] font-600"
											>Blocked</span
										>
									{/if}
								</div>
								<div class="text-fg-faint truncate text-xs">{m.email} · joined {ago(m.createdAt)}</div>
							</div>
							<label class="text-fg-dim flex items-center gap-1.5 text-xs">
								<input
									type="checkbox"
									checked={m.canClaim}
									disabled={busyId === m.id}
									onchange={(e) => setFlags(m, m.isAdmin, e.currentTarget.checked)}
									class="accent-signal"
								/>
								Can claim
							</label>
							<label class="text-fg-dim flex items-center gap-1.5 text-xs">
								<input
									type="checkbox"
									checked={m.isAdmin}
									disabled={busyId === m.id || self || m.isOwner}
									onchange={(e) => setFlags(m, e.currentTarget.checked, m.canClaim)}
									class="accent-signal"
								/>
								Admin
							</label>
							<!-- Moderation: never available for the owner or your own account. -->
							{#if !m.isOwner && !self}
								<div class="flex items-center gap-3">
									<button
										onclick={() => setBlocked(m, !m.blocked)}
										disabled={busyId === m.id}
										class="text-xs font-600 transition-colors disabled:opacity-50 {m.blocked
											? 'text-signal hover:text-signal/80'
											: 'text-amber hover:text-amber/80'}"
									>
										{m.blocked ? 'Unblock' : 'Block'}
									</button>
									{#if confirmDeleteId === m.id}
										<button
											onclick={() => removeUser(m)}
											disabled={busyId === m.id}
											class="text-coral text-xs font-700 disabled:opacity-50">Confirm</button
										>
										<button
											onclick={() => (confirmDeleteId = null)}
											class="text-fg-faint hover:text-fg-dim text-xs">Cancel</button
										>
									{:else}
										<button
											onclick={() => (confirmDeleteId = m.id)}
											class="text-coral/80 hover:text-coral text-xs font-600">Remove</button
										>
									{/if}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>
{/if}
