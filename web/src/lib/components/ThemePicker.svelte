<script lang="ts">
	// Shared theme picker — desktop sidebar and the mobile More sheet render the
	// same swatch row. Each swatch previews the theme it selects: the square is
	// that theme's page ground, the dot its accent, so the choice reads without
	// applying it. Harbour is ONE swatch (split light/dark); once chosen, a
	// Light / Dark / Auto switch and an accent row appear beneath it.
	import { theme, THEMES, ACCENTS, type ThemeChoice } from '$lib/theme.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	let { compact = false }: { compact?: boolean } = $props();

	const box = $derived(compact ? 'h-7 w-7' : 'h-5 w-5');
	const dot = $derived(compact ? 'h-2.5 w-2.5' : 'h-1.5 w-1.5');
	const accentDot = $derived(compact ? 'h-6 w-6' : 'h-4 w-4');

	const HARBOUR = { id: 'harbour', label: 'Harbour', swatch: ['#e4e4e9', '#1c1c1e', '#0a84ff'] };
	const swatches = [
		...THEMES.filter((t) => !t.id.startsWith('apple')).map((t) => ({ id: t.id as string, label: t.label, swatch: t.swatch })),
		HARBOUR
	];
	const MODES: { id: ThemeChoice; label: string }[] = [
		{ id: 'apple', label: 'Light' },
		{ id: 'apple-dark', label: 'Dark' },
		{ id: 'apple-auto', label: 'Auto' }
	];

	const isHarbour = $derived(theme.choice.startsWith('apple'));
	const selected = (id: string) => (id === 'harbour' ? isHarbour : theme.choice === id);

	function pick(id: string) {
		// Harbour starts in Auto (follows the system), as Apple's own apps do.
		if (id === 'harbour') {
			if (!isHarbour) theme.set('apple-auto');
		} else theme.set(id as ThemeChoice);
	}
</script>

<div class="flex flex-col gap-1.5">
	<!-- .label is tracked out 0.14em, which leaves a trailing gap after the last
	     glyph; the matching indent cancels it so the text optically centres. -->
	<span class="label {compact ? '' : 'text-center [text-indent:0.14em]'}">Theme</span>
	<!-- Centred in the desktop sidebar, where the row is narrower than the column.
	     Left-aligned when compact: the mobile sheet is full-width, so centring
	     would strand the swatches away from their label. -->
	<div
		class="flex flex-wrap items-center gap-1.5 {compact ? '' : 'justify-center'}"
		role="radiogroup"
		aria-label="Colour theme"
	>
		{#each swatches as t (t.id)}
			{@const split = t.id === 'harbour'}
			<Tooltip text={t.label}>
				<button
					type="button"
					role="radio"
					aria-checked={selected(t.id)}
					aria-label={t.label}
					onclick={() => pick(t.id)}
					class="{box} grid place-items-center rounded-[var(--radius)] border transition-transform hover:scale-110"
					style="background:{split
						? `linear-gradient(135deg, ${t.swatch[0]} 50%, ${t.swatch[1]} 50%)`
						: t.swatch[0]}; border-color:{split ? '#8e8e93' : t.swatch[1]}; {selected(t.id)
						? 'outline:2px solid var(--color-signal); outline-offset:1px;'
						: ''}"
				>
					<span class="{dot} rounded-full" style="background:{split && theme.isApple ? 'var(--color-signal)' : t.swatch[2]}"></span>
				</button>
			</Tooltip>
		{/each}
	</div>

	{#if isHarbour}
		<div
			data-seg
			class="border-line mt-1.5 flex overflow-hidden rounded-[var(--radius)] border text-xs {compact ? 'max-w-xs' : ''}"
			role="radiogroup"
			aria-label="Appearance"
		>
			{#each MODES as m (m.id)}
				<button
					type="button"
					role="radio"
					aria-checked={theme.choice === m.id}
					onclick={() => theme.set(m.id)}
					class="flex-1 py-1 {compact ? 'py-1.5' : ''}">{m.label}</button
				>
			{/each}
		</div>
		<div
			class="mt-1 flex flex-wrap items-center gap-2 {compact ? '' : 'justify-center'}"
			role="radiogroup"
			aria-label="Accent colour"
		>
			{#each ACCENTS as a (a.id)}
				<Tooltip text={a.label}>
					<button
						type="button"
						role="radio"
						aria-checked={theme.accent === a.id}
						aria-label="{a.label} accent"
						onclick={() => theme.setAccent(a.id)}
						class="{accentDot} grid place-items-center rounded-full transition-transform hover:scale-110"
						style="background:{theme.id === 'apple' ? a.light : a.dark}"
					>
						{#if theme.accent === a.id}
							<span class="h-1.5 w-1.5 rounded-full bg-white"></span>
						{/if}
					</button>
				</Tooltip>
			{/each}
		</div>
	{/if}
	<label class="text-fg-dim mt-1.5 flex cursor-pointer items-center gap-2 text-xs {compact ? 'justify-between text-sm' : 'justify-center'}">
		<span>Reduce transparency</span>
		<input type="checkbox" checked={theme.solid} onchange={(e) => theme.setSolid(e.currentTarget.checked)} class="accent-[var(--color-signal)]" />
	</label>
</div>
