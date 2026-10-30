<script lang="ts">
	// The pop-up for hud.show() — see lib/hud.svelte.ts. Doesn't take focus
	// or block taps; screen readers hear it through the status role.
	import { hud } from '$lib/hud.svelte';

	const STAR = 'M12 2.5l2.9 5.9 6.5.95-4.7 4.6 1.1 6.45L12 17.9l-5.8 3.05 1.1-6.45-4.7-4.6 6.5-.95z';
</script>

<div class="pointer-events-none fixed inset-0 z-[80] grid place-items-center" role="status" aria-live="polite">
	{#if hud.current}
		{#key hud.current.id}
			<div data-hud class="flex w-36 flex-col items-center gap-2 rounded-[20px] px-4 pt-5 pb-4 text-center">
				<svg viewBox="0 0 24 24" class="h-11 w-11" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
					{#if hud.current.icon === 'check'}
						<path d="M5 12.5l4.5 4.5L19 7.5" />
					{:else if hud.current.icon === 'error'}
						<path d="M12 7v6M12 17h.01" /><circle cx="12" cy="12" r="9.5" />
					{:else}
						<path d={STAR} fill={hud.current.icon === 'star' ? 'currentColor' : 'none'} />
					{/if}
				</svg>
				<span class="text-[0.85rem] leading-snug font-600">{hud.current.msg}</span>
			</div>
		{/key}
	{/if}
</div>

<style>
	[data-hud] {
		color: var(--color-fg);
		background: color-mix(in srgb, var(--color-ink-2) 82%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-line) 60%, transparent);
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.28);
		backdrop-filter: saturate(180%) blur(24px);
		animation:
			hud-in 0.22s cubic-bezier(0.2, 0.8, 0.2, 1) both,
			hud-out 0.3s ease-in 1.1s forwards;
	}
	@keyframes hud-in {
		from {
			opacity: 0;
			transform: scale(0.88);
		}
	}
	@keyframes hud-out {
		to {
			opacity: 0;
			transform: scale(0.96);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		[data-hud] {
			animation: hud-out 0.3s ease-in 1.1s forwards;
		}
	}
</style>
