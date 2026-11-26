<script lang="ts">
	// Admin → Emergency channel: the one switch for the #mayday integration —
	// the guide (/mayday), the built-in channel with its cards and monitoring
	// list, the red pulse, map rings, browser alarms and alert emails. Off, all
	// of it is hidden and nothing on the channel triggers anything; the code,
	// the monitoring list and everyone's alert settings are kept for when it's
	// turned back on. Shared by /admin and /m/admin like SeasonsPanel.
	import { auth } from '$lib/auth.svelte';
	import { env } from '$lib/env.svelte';
	import { sos } from '$lib/sos.svelte';
	import { adminEmergency } from '$lib/api';

	let { compact = false }: { compact?: boolean } = $props();

	const shell = $derived(
		compact ? 'border-line/60 bg-panel mb-3 rounded-2xl border overflow-hidden' : 'panel rise mt-6 overflow-hidden'
	);
	const pad = $derived(compact ? 'px-4' : 'px-5');

	let busy = $state(false);
	let error = $state('');
	const on = $derived(env.emergency);

	async function set(v: boolean) {
		if (busy) return;
		if (!v && !confirm('Turn off #mayday? Alerts, alert emails, map rings, the monitoring list and the guide stop for everyone. Settings are kept.'))
			return;
		busy = true;
		error = '';
		try {
			await adminEmergency.set(auth.csrf, v);
			env.emergency = v;
			sos.load();
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}
</script>

<section class={shell}>
	<div class="border-line/70 border-b {pad} {compact ? 'py-3' : 'py-3.5'}">
		<h2 class="font-display text-fg font-700 tracking-wide {compact ? 'text-xs' : 'text-sm'}">EMERGENCY CHANNEL</h2>
		<p class="text-fg-dim mt-1 text-xs">
			Ridgeline’s monitoring of <span class="font-mono">#mayday</span>, for everyone. #sos is no longer monitored.
		</p>
	</div>
	<div class="flex items-center gap-3 {pad} py-3">
		<div class="min-w-0 flex-1">
			<div class="text-fg text-sm font-600">
				#mayday integration <span class="font-400 {on ? 'text-coral' : 'text-fg-faint'}">· {on ? 'on' : 'off'}</span>
			</div>
			<div class="text-fg-faint text-xs leading-relaxed">
				{#if on}
					Live: the guide, the built-in channel and its monitoring list, the red pulse, map rings, browser alarms
					and alert emails to opted-in accounts.
				{:else}
					Off: all of it is hidden and nothing on #mayday triggers anything. The monitoring list and everyone’s
					alert settings are kept.
				{/if}
			</div>
		</div>
		<button
			role="switch"
			aria-checked={on}
			aria-label="#mayday integration"
			disabled={busy || !env.loaded}
			onclick={() => set(!on)}
			class="relative h-6 w-11 shrink-0 rounded-full border transition-colors disabled:opacity-50 {on
				? 'bg-coral border-transparent'
				: 'border-line bg-panel-2'}"
		>
			<span
				class="absolute top-0.5 left-0.5 h-[18px] w-[18px] rounded-full bg-white shadow transition-transform"
				style="transform: translateX({on ? 20 : 0}px)"
			></span>
		</button>
	</div>
	{#if error}<p class="text-coral {pad} pb-3 text-xs">{error}</p>{/if}
</section>
