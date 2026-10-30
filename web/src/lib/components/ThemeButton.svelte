<script lang="ts">
	// One-line stand-in for the theme picker: a swatch of the active theme and
	// its name. Opens ThemeModal.
	import { theme } from '$lib/theme.svelte';
	import ThemeModal from './ThemeModal.svelte';

	let { variant = 'row' }: { variant?: 'row' | 'icon' } = $props();
	let open = $state(false);

	function portal(node: HTMLElement) {
		document.body.appendChild(node);
		return { destroy: () => node.remove() };
	}

	const mode = $derived(theme.choice === 'apple-auto' ? 'Auto' : theme.id === 'apple-dark' ? 'Dark' : 'Light');
	const name = $derived(theme.isApple ? `Harbour · ${mode}` : theme.def.label);
</script>

{#snippet swatch(size: string, dot: string)}
	<span class="grid {size} shrink-0 place-items-center rounded-[var(--radius)] border" style="background:{theme.def.swatch[0]}; border-color:{theme.def.swatch[1]}">
		<span class="{dot} rounded-full" style="background:{theme.isApple ? 'var(--color-signal)' : theme.def.swatch[2]}"></span>
	</span>
{/snippet}

{#if variant === 'icon'}
	<button onclick={() => (open = true)} aria-label="Theme: {name}" aria-haspopup="dialog" class="p-1">
		{@render swatch('h-5 w-5', 'h-2 w-2')}
	</button>
{:else}
	<button
		onclick={() => (open = true)}
		aria-haspopup="dialog"
		class="text-fg-dim hover:text-fg hover:border-line-bright border-line flex w-full items-center gap-2.5 rounded-[var(--radius)] border px-3 py-2 text-xs transition-colors"
	>
		{@render swatch('h-4 w-4', 'h-1.5 w-1.5')}
		<span class="min-w-0 flex-1 truncate text-left font-medium">{name}</span>
		<span class="label !text-[0.6rem]">Theme</span>
	</button>
{/if}

{#if open}
	<!-- Mounted under <body>: the sidebar and app bar are backdrop-blurred, and
	     a blur makes them the containing block for position:fixed, which would
	     trap the modal inside them. -->
	<div use:portal><ThemeModal onclose={() => (open = false)} /></div>
{/if}
