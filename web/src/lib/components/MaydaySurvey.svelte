<script lang="ts">
	// The emergency-channel feedback survey (#sos, and whether to run one again),
	// on /survey and /m/survey. It doesn't mention #mayday: that name isn't public.
	// POST /api/survey/mayday-2026 stores it for admins (Admin → Survey); the
	// server keeps only known choices, caps the text and rate-limits by IP.
	// Every question is optional, but at least one must be answered.
	import { auth } from '$lib/auth.svelte';
	import { USED, MONITOR, FEATURES, SURVEY_ID, SURVEY_DONE_KEY } from '$lib/survey';
	import { onMount } from 'svelte';

	let { compact = false }: { compact?: boolean } = $props();

	const MAX = 2000;
	let used = $state<string[]>([]);
	let monitor = $state('');
	let nameOther = $state('');
	let features = $state<string[]>([]);
	let concerns = $state('');
	let other = $state('');
	let email = $state('');
	let website = $state(''); // honeypot
	let busy = $state(false);
	let sent = $state(false);
	let before = $state(false);
	let error = $state('');

	onMount(() => {
		try {
			before = !!localStorage.getItem(SURVEY_DONE_KEY);
		} catch {
			/* storage unavailable */
		}
	});

	function toggle(list: string[], id: string): string[] {
		// "I never used it" and the others don't go together.
		if (list === used && id === 'never') return list.includes('never') ? [] : ['never'];
		const l = list === used ? list.filter((x) => x !== 'never') : list;
		return l.includes(id) ? l.filter((x) => x !== id) : [...l, id];
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			const r = await fetch(`/api/survey/${SURVEY_ID}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ used, monitor, nameOther, features, concerns, other, email, website })
			});
			const j = await r.json().catch(() => ({}));
			if (r.ok) {
				sent = true;
				try {
					localStorage.setItem(SURVEY_DONE_KEY, new Date().toISOString());
				} catch {
					/* storage unavailable */
				}
			} else error = j.error || `Could not send (HTTP ${r.status}).`;
		} catch {
			error = 'Could not reach the server. Check your connection and try again.';
		} finally {
			busy = false;
		}
	}

	const field =
		'border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-base outline-none md:text-sm';
	const q = 'text-fg text-sm font-600 mb-2 block';
	const opt = (on: boolean) =>
		`flex cursor-pointer items-center gap-2.5 rounded-[var(--radius)] border px-3 py-2 text-sm transition-colors ${on ? 'border-coral/60 bg-coral/10 text-fg' : 'border-line text-fg-dim hover:border-line-bright'}`;
</script>

{#if sent}
	<div class="border-signal/40 bg-signal/10 rounded-[var(--radius)] border px-4 py-3">
		<p class="text-signal text-sm font-600">Thank you, your answers are in.</p>
		<p class="text-fg-dim mt-1 text-sm">
			{email.trim() ? `We may reply to ${email.trim()}.` : 'They help decide what Ridgeline does next.'}
		</p>
	</div>
{:else}
	{#if before}
		<p class="border-line text-fg-dim mb-5 rounded-[var(--radius)] border px-3 py-2 text-xs">
			You’ve answered this survey from this browser already. You can send it again if something has changed.
		</p>
	{/if}
	<form onsubmit={submit} class="flex flex-col {compact ? 'gap-6' : 'gap-7'}" novalidate>
		<fieldset>
			<legend class={q}>1. How did you use #sos? <span class="text-fg-faint font-400">Pick any.</span></legend>
			<div class="grid gap-2 sm:grid-cols-2">
				{#each USED as o (o.id)}
					{@const on = used.includes(o.id)}
					<label class={opt(on)}>
						<input type="checkbox" class="accent-coral h-4 w-4 shrink-0" checked={on} onchange={() => (used = toggle(used, o.id))} />
						{o.label}
					</label>
				{/each}
			</div>
		</fieldset>

		<fieldset>
			<legend class={q}>2. Should Ridgeline monitor an emergency channel again?</legend>
			<div class="flex flex-wrap gap-2">
				{#each MONITOR as o (o.id)}
					<label class={opt(monitor === o.id)}>
						<input type="radio" name="monitor" class="accent-coral h-4 w-4" value={o.id} bind:group={monitor} />
						{o.label}
					</label>
				{/each}
			</div>
		</fieldset>

		<fieldset>
			<legend class={q}>
				<label for="survey-name">3. If it does, what should the channel be called?</label>
				<span class="text-fg-faint font-400">Write in your choice.</span>
			</legend>
			<input
				id="survey-name"
				bind:value={nameOther}
				maxlength="60"
				placeholder="e.g. #help"
				autocapitalize="off"
				spellcheck="false"
				class="{field} sm:max-w-xs"
			/>
		</fieldset>

		<fieldset>
			<legend class={q}>4. Which of these would you want? <span class="text-fg-faint font-400">Pick any.</span></legend>
			<div class="grid gap-2 sm:grid-cols-2">
				{#each FEATURES as o (o.id)}
					{@const on = features.includes(o.id)}
					<label class={opt(on)}>
						<input type="checkbox" class="accent-coral h-4 w-4 shrink-0" checked={on} onchange={() => (features = toggle(features, o.id))} />
						{o.label}
					</label>
				{/each}
			</div>
		</fieldset>

		<label class="block">
			<span class={q}>5. Any concerns? <span class="text-fg-faint font-400">False alarms, privacy, people relying on it…</span></span>
			<textarea bind:value={concerns} maxlength={MAX} rows="3" class="{field} resize-y"></textarea>
		</label>

		<label class="block">
			<span class={q}>6. Anything else?</span>
			<textarea bind:value={other} maxlength={MAX} rows="3" class="{field} resize-y"></textarea>
		</label>

		<label class="block">
			<span class={q}>Your email <span class="text-fg-faint font-400">(optional, only if you’d like a reply)</span></span>
			<input type="email" bind:value={email} autocomplete="email" placeholder={auth.user?.email ?? ''} class="{field} sm:max-w-sm" />
		</label>

		<!-- Honeypot: hidden from people, filled in by bots. -->
		<div class="absolute -left-[9999px] h-px w-px overflow-hidden" aria-hidden="true">
			<label>Website<input bind:value={website} tabindex="-1" autocomplete="off" /></label>
		</div>
		{#if error}<p class="text-coral text-sm">{error}</p>{/if}
		<button
			type="submit"
			disabled={busy}
			class="bg-coral text-ink w-full rounded-[var(--radius)] px-4 py-2.5 text-sm font-600 transition-[filter] hover:brightness-110 disabled:opacity-50 sm:w-auto sm:self-start"
		>
			{busy ? 'Sending…' : 'Send answers'}
		</button>
	</form>
{/if}
