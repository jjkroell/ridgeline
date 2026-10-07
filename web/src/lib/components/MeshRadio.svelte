<script lang="ts">
	// The radio settings every node and observer on this mesh shares, shown once
	// for the whole site instead of a label on each node. Renders nothing unless
	// the server reports exactly one preset (see /api/health meshRadio).
	//
	//   variant="status"  under the sidebar's LIVE row: what we're listening on
	//   variant="inline"  one line with an icon, for the mobile More sheet and
	//                     the narrow-screen menu (no LIVE row there)
	import { env } from '$lib/env.svelte';

	let { variant = 'status', class: cls = '' }: { variant?: 'status' | 'inline'; class?: string } = $props();

	const parts = $derived.by(() => {
		if (!env.meshRadio) return null;
		const [f, b, s, c] = env.meshRadio.split(',');
		const freq = f ? `${(+f).toFixed(3)} MHz` : '';
		const rest = [b ? `${b} kHz` : '', s ? `SF${s}` : '', c ? `CR${c}` : ''].filter(Boolean).join(' · ');
		return { freq, rest };
	});
	const title = $derived(parts ? `Every node and observer on this mesh uses ${parts.freq} · ${parts.rest}` : '');
</script>

{#if parts}
	{#if variant === 'status'}
		<div class="tnum font-mono leading-snug {cls}" {title}>
			<div class="text-fg-dim text-[0.72rem]">{parts.freq}</div>
			<div class="text-fg-faint text-[0.62rem]">{parts.rest}</div>
		</div>
	{:else}
		<div class="text-fg-dim flex items-center gap-2.5 {cls}" {title}>
			<svg viewBox="0 0 24 24" class="text-fg-faint h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"
				><path d="M4.9 19.1a10 10 0 0 1 0-14.2M19.1 4.9a10 10 0 0 1 0 14.2M7.8 16.2a6 6 0 0 1 0-8.4M16.2 7.8a6 6 0 0 1 0 8.4M12 13a1 1 0 100-2 1 1 0 000 2z" /></svg
			>
			<span class="tnum font-mono text-[0.72rem]">{parts.freq}</span>
			<span class="text-fg-faint tnum truncate font-mono text-[0.62rem]">{parts.rest}</span>
		</div>
	{/if}
{/if}
