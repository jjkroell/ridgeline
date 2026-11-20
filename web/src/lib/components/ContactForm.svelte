<script lang="ts">
	// The site's one contact form (on /contact; the /faq page links there).
	// POST /api/contact delivers to the operator's mailbox; the server
	// validates, rate-limits and drops anything that fills the hidden "website"
	// field (bots), so the client only checks the two required fields.
	import { auth } from '$lib/auth.svelte';

	const MAX = 4000;
	let name = $state('');
	let email = $state('');
	let message = $state('');
	let website = $state(''); // honeypot
	let busy = $state(false);
	let sent = $state(false);
	let error = $state('');

	// Signed-in visitors: start from their account details (still editable).
	$effect(() => {
		if (auth.user && !email) email = auth.user.email;
		if (auth.user && !name) name = auth.user.displayName ?? '';
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (!email.trim() || !message.trim()) {
			error = 'An email address and a message are both needed.';
			return;
		}
		busy = true;
		try {
			const r = await fetch('/api/contact', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, email, message, website })
			});
			const j = await r.json().catch(() => ({}));
			if (r.ok) {
				sent = true;
				message = '';
			} else {
				error = j.error || `Could not send (HTTP ${r.status}).`;
			}
		} catch {
			error = 'Could not reach the server. Check your connection and try again.';
		} finally {
			busy = false;
		}
	}

	const field =
		'border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none';
</script>

{#if sent}
	<div class="border-signal/40 bg-signal/10 rounded-[var(--radius)] border px-4 py-3">
		<p class="text-signal text-sm font-600">Thanks, your message is on its way.</p>
		<p class="text-fg-dim mt-1 text-sm">We will reply to {email}.</p>
		<button type="button" onclick={() => (sent = false)} class="text-signal mt-2 text-xs hover:underline">Send another message</button>
	</div>
{:else}
	<form onsubmit={submit} class="flex flex-col gap-4" novalidate>
		<label class="block">
			<span class="label normal-case text-fg-dim mb-1.5 block">Your name <span class="text-fg-faint">(optional)</span></span>
			<input bind:value={name} maxlength="100" autocomplete="name" class={field} />
		</label>
		<label class="block">
			<span class="label normal-case text-fg-dim mb-1.5 block">Your email</span>
			<input type="email" bind:value={email} required autocomplete="email" placeholder="so we can reply" class={field} />
		</label>
		<label class="block">
			<span class="label normal-case text-fg-dim mb-1.5 flex items-baseline justify-between">
				<span>Message</span><span class="text-fg-faint tnum">{message.length} / {MAX}</span>
			</span>
			<textarea bind:value={message} required maxlength={MAX} rows="7" class="{field} min-h-[9rem] resize-y"
				placeholder="If something isn't working, tell us which page you were on and roughly when it happened."></textarea>
		</label>
		<!-- Honeypot: hidden from people, filled in by bots. -->
		<div class="absolute -left-[9999px] h-px w-px overflow-hidden" aria-hidden="true">
			<label>Website<input bind:value={website} tabindex="-1" autocomplete="off" /></label>
		</div>
		{#if error}<p class="text-coral text-xs">{error}</p>{/if}
		<button
			type="submit"
			disabled={busy}
			class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 w-full rounded-[var(--radius)] border px-4 py-2.5 text-sm font-600 transition-colors disabled:opacity-50 sm:w-auto sm:self-end"
		>
			{busy ? 'Sending…' : 'Send'}
		</button>
	</form>
{/if}
