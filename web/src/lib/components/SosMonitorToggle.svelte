<!--
  "Are you monitoring #sos?" Yes / No: whether the signed-in account is on the
  public #sos monitoring list, and with which radio: the server lists only a
  companion it has heard, as that radio advertises its name. In the monitor list card,
  the #sos guide and Account settings. Nothing is sent on the mesh.
-->
<script lang="ts">
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

	async function save(enabled: boolean) {
		if (busy) return;
		failed = false;
		msg = '';
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
				oninput={(e) => (typed = e.currentTarget.value)}
				maxlength="64"
				placeholder="As in your MeshCore app"
				aria-describedby="{uid}-hint"
				autocomplete="off"
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
		<p class="text-fg-faint mt-1.5 text-xs leading-relaxed" id="{uid}-hint">
			Your companion radio’s name. It’s shown on the public list, and Ridgeline checks that it has heard it.
		</p>
	{/if}
	<p class="mt-1 text-xs {failed ? 'text-amber' : 'text-signal'} empty:hidden" role="status" aria-live="polite">{msg}</p>
</div>
