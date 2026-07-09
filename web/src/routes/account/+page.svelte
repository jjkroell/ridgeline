<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { authApi, claims, shares, type ClaimWithNode, type SharedWithMe } from '$lib/api';
	import { ago, shortKey } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';

	// --- Account settings (self-service edit) ---
	let dn = $state('');
	let dnMsg = $state('');
	let dnBusy = $state(false);

	let newEmail = $state('');
	let emailPw = $state('');
	let emailMsg = $state('');
	let emailErr = $state('');
	let emailBusy = $state(false);

	let curPw = $state('');
	let newPw = $state('');
	let newPw2 = $state('');
	let pwMsg = $state('');
	let pwErr = $state('');
	let pwBusy = $state(false);
	const MIN_PW = 8;
	// New password validity: long enough and the two entries agree.
	const pwLongEnough = $derived(newPw.length >= MIN_PW);
	const pwMatch = $derived(newPw2.length > 0 && newPw === newPw2);
	const pwReady = $derived(!!curPw && pwLongEnough && pwMatch);

	let resendMsg = $state('');

	// Collapsible sections — persisted per browser so each section remembers
	// whether it was left open. Default collapsed on first visit.
	const openKeys = {
		settings: 'ridgeline-acct-open-settings',
		nodes: 'ridgeline-acct-open-nodes',
		shared: 'ridgeline-acct-open-shared'
	};
	function readOpen(key: string): boolean {
		try {
			return localStorage.getItem(key) === '1';
		} catch {
			return false;
		}
	}
	let openSettings = $state(readOpen(openKeys.settings));
	let openNodes = $state(readOpen(openKeys.nodes));
	let openShared = $state(readOpen(openKeys.shared));
	// Persist whenever any section is toggled.
	$effect(() => {
		try {
			localStorage.setItem(openKeys.settings, openSettings ? '1' : '0');
			localStorage.setItem(openKeys.nodes, openNodes ? '1' : '0');
			localStorage.setItem(openKeys.shared, openShared ? '1' : '0');
		} catch {
			/* storage unavailable — sections just won't persist */
		}
	});

	async function saveName() {
		dnBusy = true;
		dnMsg = '';
		try {
			await auth.updateDisplayName(dn.trim());
			dnMsg = 'Saved.';
		} catch (e) {
			dnMsg = String((e as Error).message ?? e);
		} finally {
			dnBusy = false;
		}
	}

	async function saveEmail() {
		emailBusy = true;
		emailErr = '';
		emailMsg = '';
		try {
			await auth.changeEmail(emailPw, newEmail.trim());
			newEmail = '';
			emailPw = '';
			emailMsg = auth.user?.emailVerified
				? 'Email updated.'
				: 'Confirmation link sent — check your new inbox to finish the change.';
		} catch (e) {
			emailErr = String((e as Error).message ?? e);
		} finally {
			emailBusy = false;
		}
	}

	async function savePassword() {
		pwErr = '';
		pwMsg = '';
		if (newPw !== newPw2) {
			pwErr = 'New passwords do not match.';
			return;
		}
		pwBusy = true;
		try {
			await auth.changePassword(curPw, newPw);
			curPw = '';
			newPw = '';
			newPw2 = '';
			pwMsg = 'Password changed.';
		} catch (e) {
			pwErr = String((e as Error).message ?? e);
		} finally {
			pwBusy = false;
		}
	}

	async function resendVerification() {
		if (!auth.user) return;
		resendMsg = '';
		await authApi.resendVerification(auth.user.email);
		resendMsg = 'Sent — check your inbox.';
	}

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
		dn = auth.user?.displayName ?? '';
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

{#snippet chevron(open: boolean)}
	<svg
		viewBox="0 0 24 24"
		class="text-fg-faint ml-auto h-4 w-4 shrink-0 transition-transform {open ? 'rotate-180' : ''}"
		fill="none"
		stroke="currentColor"
		stroke-width="1.8"
		stroke-linecap="round"
		stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg
	>
{/snippet}

{#if auth.loggedIn}
	{@const u = auth.user}
	<div class="flex flex-col gap-6 px-6 pb-16 md:px-10">
		{#if u && !u.emailVerified}
			<div class="border-amber/40 bg-amber/10 rounded-[var(--radius)] border px-4 py-3">
				<p class="text-fg-dim text-sm leading-relaxed">
					<strong class="text-amber">Confirm your email.</strong>
					We sent a link to <strong class="text-fg break-all">{u.email}</strong>. Until it's confirmed
					you won't be able to sign in again.
					<button onclick={resendVerification} class="text-signal hover:underline">Resend</button
					>{#if resendMsg}<span class="text-signal"> {resendMsg}</span>{/if}
				</p>
			</div>
		{/if}

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

		<!-- Account settings -->
		<div class="panel overflow-hidden">
			<button
				onclick={() => (openSettings = !openSettings)}
				aria-expanded={openSettings}
				class="panel-hover flex w-full items-center gap-2.5 px-5 py-3.5 text-left {openSettings
					? 'border-line/70 border-b'
					: ''}"
			>
				<span class="font-display text-fg text-sm font-700">Account settings</span>
				{#if u && !u.emailVerified}
					<span class="bg-amber/15 text-amber rounded-full px-2 py-0.5 text-xs font-600"
						>Email unconfirmed</span
					>
				{/if}
				{@render chevron(openSettings)}
			</button>
			{#if openSettings}
			<div class="divide-line/60 divide-y">
				<!-- Display name -->
				<form onsubmit={(e) => (e.preventDefault(), saveName())} class="px-5 py-4">
					<label class="label normal-case text-fg-dim mb-1.5 block" for="acct-dn"
						>Display name</label
					>
					<div class="flex flex-wrap items-center gap-2">
						<input
							id="acct-dn"
							bind:value={dn}
							maxlength="64"
							placeholder="Your name or handle"
							autocomplete="nickname"
							class="border-line bg-ink-2 text-fg focus:border-signal min-w-0 flex-1 rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
						/>
						<button
							type="submit"
							disabled={dnBusy || dn.trim() === (u?.displayName ?? '')}
							class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors disabled:opacity-50"
							>{dnBusy ? 'Saving…' : 'Save'}</button
						>
					</div>
					{#if dnMsg}<p class="text-fg-faint mt-2 text-xs">{dnMsg}</p>{/if}
				</form>

				<!-- Email -->
				<form onsubmit={(e) => (e.preventDefault(), saveEmail())} class="px-5 py-4">
					<label class="label normal-case text-fg-dim mb-1.5 block" for="acct-email">Email</label>
					<p class="text-fg-faint mb-2 text-xs">
						Current: <span class="text-fg-dim break-all">{u?.email}</span>. Changing it requires
						confirming the new address.
					</p>
					<div class="flex flex-col gap-2">
						<input
							id="acct-email"
							type="email"
							bind:value={newEmail}
							placeholder="new@email.com"
							autocomplete="email"
							class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
						/>
						<input
							type="password"
							bind:value={emailPw}
							placeholder="current password"
							autocomplete="current-password"
							class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
						/>
						<button
							type="submit"
							disabled={emailBusy || !newEmail.trim() || !emailPw}
							class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 self-start rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors disabled:opacity-50"
							>{emailBusy ? 'Updating…' : 'Change email'}</button
						>
					</div>
					{#if emailErr}<p class="text-coral mt-2 text-xs">{emailErr}</p>{/if}
					{#if emailMsg}<p class="text-signal mt-2 text-xs">{emailMsg}</p>{/if}
				</form>

				<!-- Password -->
				<form onsubmit={(e) => (e.preventDefault(), savePassword())} class="px-5 py-4">
					<label class="label normal-case text-fg-dim mb-1.5 block" for="acct-pw">Password</label>
					<div class="flex flex-col gap-2">
						<input
							id="acct-pw"
							type="password"
							bind:value={curPw}
							placeholder="current password"
							autocomplete="current-password"
							class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
						/>
						<input
							type="password"
							bind:value={newPw}
							placeholder="new password (at least 8 characters)"
							minlength={MIN_PW}
							autocomplete="new-password"
							class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
						/>
						<!-- Live character count / minimum hint -->
						{#if newPw.length > 0}
							<span class="text-fg-faint -mt-1 text-xs tnum">
								{newPw.length} character{newPw.length === 1 ? '' : 's'}{pwLongEnough
									? ''
									: ` — ${MIN_PW - newPw.length} more to reach ${MIN_PW}`}
							</span>
						{/if}
						<input
							type="password"
							bind:value={newPw2}
							placeholder="confirm new password"
							autocomplete="new-password"
							class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
						/>
						<!-- Match indicator -->
						{#if newPw2.length > 0}
							<span class="-mt-1 text-xs {pwMatch ? 'text-signal' : 'text-coral'}">
								{pwMatch ? '✓ Passwords match' : '✗ Passwords do not match'}
							</span>
						{/if}
						<button
							type="submit"
							disabled={pwBusy || !pwReady}
							class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 self-start rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors disabled:opacity-50"
							>{pwBusy ? 'Changing…' : 'Change password'}</button
						>
					</div>
					{#if pwErr}<p class="text-coral mt-2 text-xs">{pwErr}</p>{/if}
					{#if pwMsg}<p class="text-signal mt-2 text-xs">{pwMsg}</p>{/if}
				</form>
			</div>
			{/if}
		</div>

		<!-- My nodes -->
		{#if myNodes.length}
			<div class="panel overflow-hidden">
				<button
					onclick={() => (openNodes = !openNodes)}
					aria-expanded={openNodes}
					class="panel-hover flex w-full items-center gap-2.5 px-5 py-3.5 text-left {openNodes
						? 'border-line/70 border-b'
						: ''}"
				>
					<span class="font-display text-fg text-sm font-700">My nodes</span>
					<span class="text-fg-faint text-xs">{myNodes.length}</span>
					{@render chevron(openNodes)}
				</button>
				{#if openNodes}
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
				{/if}
			</div>
		{/if}

		<!-- Shared with me -->
		{#if sharedWithMe.length}
			<div class="panel overflow-hidden">
				<button
					onclick={() => (openShared = !openShared)}
					aria-expanded={openShared}
					class="panel-hover flex w-full items-center gap-2.5 px-5 py-3.5 text-left {openShared
						? 'border-line/70 border-b'
						: ''}"
				>
					<span class="font-display text-fg text-sm font-700">Shared with me</span>
					<span class="text-fg-faint text-xs">{sharedWithMe.length}</span>
					{#if sharedWithMe.some((s) => !s.seen)}
						<span class="bg-signal text-ink rounded-full px-2 py-0.5 text-xs font-700">New</span>
					{/if}
					{@render chevron(openShared)}
				</button>
				{#if openShared}
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
				{/if}
			</div>
		{/if}

	</div>
{/if}
