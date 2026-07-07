<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';

	let mode = $state<'login' | 'register'>('login');
	let email = $state('');
	let password = $state('');
	let displayName = $state('');
	let error = $state('');
	let busy = $state(false);

	// Already signed in → send them to their account.
	$effect(() => {
		if (auth.ready && auth.loggedIn) goto('/account');
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			if (mode === 'register') {
				await auth.register(email.trim(), password, displayName.trim());
			} else {
				await auth.login(email.trim(), password);
			}
			goto('/account');
		} catch (err) {
			error = String((err as Error).message ?? err);
		} finally {
			busy = false;
		}
	}

	function swap(to: 'login' | 'register') {
		mode = to;
		error = '';
	}
</script>

<svelte:head><title>Sign in · Ridgeline</title></svelte:head>

<PageHeader eyebrow="Accounts" title={mode === 'login' ? 'Sign in' : 'Create account'} />

<div class="px-6 pb-16 md:px-10">
	<div class="panel mx-auto max-w-md px-6 py-8">
		<!-- Mode toggle -->
		<div class="border-line/70 mb-6 flex gap-1 rounded-[var(--radius)] border p-1">
			<button
				onclick={() => swap('login')}
				class="flex-1 rounded-[calc(var(--radius)-2px)] px-3 py-1.5 text-sm font-600 transition-colors
					{mode === 'login' ? 'bg-signal/15 text-signal' : 'text-fg-dim hover:text-fg'}"
			>
				Sign in
			</button>
			<button
				onclick={() => swap('register')}
				class="flex-1 rounded-[calc(var(--radius)-2px)] px-3 py-1.5 text-sm font-600 transition-colors
					{mode === 'register' ? 'bg-signal/15 text-signal' : 'text-fg-dim hover:text-fg'}"
			>
				Create account
			</button>
		</div>

		<form onsubmit={submit} class="flex flex-col gap-4">
			{#if mode === 'register'}
				<label class="block">
					<span class="label normal-case text-fg-dim mb-1.5 block">Display name / callsign</span>
					<input
						type="text"
						bind:value={displayName}
						placeholder="VE7XYZ"
						maxlength="64"
						autocomplete="nickname"
						class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
					/>
					<span class="text-fg-faint mt-1 block text-xs">Shown on your public notes. Optional.</span>
				</label>
			{/if}
			<label class="block">
				<span class="label normal-case text-fg-dim mb-1.5 block">Email</span>
				<input
					type="email"
					bind:value={email}
					required
					autocomplete="email"
					placeholder="you@example.com"
					class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
				/>
			</label>
			<label class="block">
				<span class="label normal-case text-fg-dim mb-1.5 block">Password</span>
				<input
					type="password"
					bind:value={password}
					required
					minlength={mode === 'register' ? 8 : undefined}
					autocomplete={mode === 'register' ? 'new-password' : 'current-password'}
					placeholder={mode === 'register' ? 'at least 8 characters' : '••••••••'}
					class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
				/>
			</label>

			{#if error}
				<p class="text-coral text-xs">{error}</p>
			{/if}

			<button
				type="submit"
				disabled={busy}
				class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 mt-1 w-full rounded-[var(--radius)] border px-4 py-2.5 text-sm font-600 transition-colors disabled:opacity-50"
			>
				{busy ? 'Working…' : mode === 'login' ? 'Sign in' : 'Create account'}
			</button>
		</form>

		<p class="text-fg-faint mt-6 text-center text-xs leading-relaxed">
			{#if mode === 'register'}
				Anyone can register. Claiming nodes and setting private locations require approval by an
				admin.
			{:else}
				New here?
				<button onclick={() => swap('register')} class="text-signal hover:underline"
					>Create an account</button
				>.
			{/if}
		</p>
	</div>
</div>
