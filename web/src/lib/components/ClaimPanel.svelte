<script lang="ts">
	import { onDestroy } from 'svelte';
	import { claims, type ClaimStatus } from '$lib/api';
	import { auth } from '$lib/auth.svelte';

	interface Props {
		pubkey: string;
		/** Tighter spacing for the mobile node page. */
		compact?: boolean;
	}
	let { pubkey, compact = false }: Props = $props();

	let status = $state<ClaimStatus | null>(null);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let copied = $state(false);
	let poll: ReturnType<typeof setInterval> | null = null;

	async function load() {
		try {
			status = await claims.status(pubkey);
			error = '';
			// Keep polling while there's something to detect live: a pending claim
			// awaiting its verifying advert, or a just-verified node whose name still
			// carries the code (waiting for the owner to restore it). Stop otherwise.
			const watching =
				status.mine?.status === 'pending' || (status.ownedByMe && status.nameNeedsReset);
			if (watching && !poll) {
				poll = setInterval(load, 15000);
			} else if (!watching && poll) {
				clearInterval(poll);
				poll = null;
			}
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			loading = false;
		}
	}

	// (Re)load whenever the target node changes.
	let loadedFor = $state('');
	$effect(() => {
		if (pubkey && pubkey !== loadedFor) {
			loadedFor = pubkey;
			loading = true;
			load();
		}
	});
	onDestroy(() => poll && clearInterval(poll));

	async function claim() {
		busy = true;
		error = '';
		try {
			await claims.create(auth.csrf, pubkey);
			await load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}

	async function release() {
		busy = true;
		error = '';
		try {
			await claims.release(auth.csrf, pubkey);
			await load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}

	async function copyCode() {
		const code = status?.mine?.code;
		if (!code) return;
		try {
			await navigator.clipboard.writeText(code);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch {
			/* clipboard blocked — the code is visible to type manually */
		}
	}

	function expiresIn(iso?: string): string {
		if (!iso) return '';
		const ms = new Date(iso).getTime() - Date.now();
		if (ms <= 0) return 'expired';
		const m = Math.round(ms / 60000);
		return m >= 1 ? `~${m} min` : '<1 min';
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
			><path d="M12 2 4 5v6c0 5 3.4 8.5 8 11 4.6-2.5 8-6 8-11V5l-8-3z" /><path d="m9 12 2 2 4-4" /></svg
		>
		Ownership
	</div>

	{#if loading}
		<p class="text-fg-faint text-sm">Checking…</p>
	{:else if status}
		{#if status.ownedByMe}
			<!-- You own it -->
			<div class="flex flex-wrap items-center gap-3">
				<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600"
					>You own this node</span
				>
				<button
					onclick={release}
					disabled={busy}
					class="text-fg-faint hover:text-coral ml-auto text-xs transition-colors disabled:opacity-50"
					>Release ownership</button
				>
			</div>
			{#if status.nameNeedsReset}
				<div class="border-amber/40 bg-amber/10 mt-3 rounded-[var(--radius)] border px-3 py-2.5">
					<p class="text-fg-dim text-xs leading-relaxed">
						<strong class="text-amber">Set the name back:</strong> this node's advertised name still
						contains the verification code. Restore its normal name and send another advert — this
						note clears once Ridgeline sees the change.
					</p>
				</div>
			{/if}
			<p class="text-fg-faint mt-3 text-xs leading-relaxed">
				You can add public and private notes and set this node's private exact location (coming in
				the next update).
			</p>
		{:else if status.owner}
			<!-- Owned by someone else -->
			<div class="flex items-center gap-2">
				<span class="bg-signal/15 text-signal rounded-full px-2.5 py-1 text-xs font-600">Claimed</span
				>
				<span class="text-fg-dim text-sm">by {status.owner.displayName}</span>
			</div>
		{:else if status.mine?.status === 'pending'}
			<!-- Your pending claim: show the code + instructions, poll for verification -->
			<p class="text-fg-dim text-sm leading-relaxed">
				To prove you control this node, set its advertised <strong class="text-fg">name</strong> to
				include this code, then send an advert (or wait for the next one):
			</p>
			<div class="border-line bg-ink-2 mt-3 flex items-center gap-3 rounded-[var(--radius)] border px-4 py-3">
				<code class="text-signal font-mono text-lg font-700 tracking-[0.2em]">{status.mine.code}</code>
				<button
					onclick={copyCode}
					class="border-line text-fg-dim hover:text-fg ml-auto rounded-[var(--radius)] border px-2.5 py-1 text-xs transition-colors"
					>{copied ? 'Copied' : 'Copy'}</button
				>
			</div>
			<ul class="text-fg-faint mt-3 space-y-1 text-xs leading-relaxed">
				<li>
					• <strong class="text-fg-dim">Repeater / room server:</strong> change the name via its BLE app
					or serial console, then trigger an advert.
				</li>
				<li>• <strong class="text-fg-dim">Companion:</strong> change the name in the MeshCore app.</li>
				<li>• We verify the advert's signature, so only your node can complete this.</li>
				<li>
					• Once it's verified, <strong class="text-fg-dim">change the name back</strong> and send another
					advert so Ridgeline shows the node's correct name.
				</li>
			</ul>
			<div class="mt-3 flex items-center gap-3">
				<span class="flex items-center gap-1.5 text-xs text-amber">
					<span class="inline-block h-2 w-2 animate-pulse rounded-full bg-amber"></span>
					Waiting for advert · code valid {expiresIn(status.mine.expiresAt)}
				</span>
				<button
					onclick={release}
					disabled={busy}
					class="text-fg-faint hover:text-coral ml-auto text-xs transition-colors disabled:opacity-50"
					>Cancel</button
				>
			</div>
		{:else if !status.loggedIn}
			<p class="text-fg-dim text-sm">
				<a href="/login" class="text-signal hover:underline">Sign in</a> to claim your nodes.
			</p>
		{:else if !status.canClaim}
			<p class="text-fg-dim text-sm leading-relaxed">
				Claiming nodes requires admin approval. Once approved you can prove control of this node and
				manage it.
			</p>
		{:else}
			<div class="flex flex-wrap items-center gap-3">
				<p class="text-fg-dim text-sm">Is this your node?</p>
				<button
					onclick={claim}
					disabled={busy}
					class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 ml-auto rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors disabled:opacity-50"
					>{busy ? 'Starting…' : 'Claim this node'}</button
				>
			</div>
		{/if}

		{#if error}
			<p class="text-coral mt-3 text-xs">{error}</p>
		{/if}
	{/if}
</div>
