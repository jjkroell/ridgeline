<!--
  Marks a node that also runs an observer (an observer's id is its node's public
  key). Observer violet, matching the Observer role colour.

    variant="icon"  a small antenna on node lists — the row is already a link to
                    the node, so this is a tooltip, not a second link
    variant="link"  a labelled pill on the node page that opens the observer page
-->
<script lang="ts">
	import Tooltip from './Tooltip.svelte';

	let {
		id,
		variant = 'icon',
		size = 'sm',
		mobile = false
	}: { id: string; variant?: 'icon' | 'link'; size?: 'sm' | 'md'; mobile?: boolean } = $props();

	const sz = $derived(size === 'md' ? 'h-4 w-4' : 'h-3.5 w-3.5');
	const href = $derived(`${mobile ? '/m' : ''}/observers/${encodeURIComponent(id)}`);
</script>

{#snippet antenna(cls: string)}
	<svg viewBox="0 0 24 24" class={cls} fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"
		><path d="M12 13v8M8.5 21h7M7.8 16.2a6 6 0 0 1 0-8.4M16.2 7.8a6 6 0 0 1 0 8.4M4.9 19.1a10 10 0 0 1 0-14.2M19.1 4.9a10 10 0 0 1 0 14.2" /><circle cx="12" cy="12" r="1.2" /></svg
	>
{/snippet}

{#if variant === 'icon'}
	<Tooltip text="Also an observer" class="shrink-0">
		<span class="text-role-observer inline-flex" role="img" aria-label="Observer">{@render antenna(sz)}</span>
	</Tooltip>
{:else}
	<a
		{href}
		title="Open this node's observer page"
		class="text-role-observer font-mono inline-flex shrink-0 items-center gap-1.5 rounded-[var(--radius)] px-2 py-0.5 text-[0.7rem] tracking-wide transition-colors hover:brightness-125"
		style="background:color-mix(in srgb, var(--color-role-observer) 12%, transparent); border:1px solid color-mix(in srgb, var(--color-role-observer) 30%, transparent)"
	>
		{@render antenna('h-3.5 w-3.5')}
		Observer page
	</a>
{/if}
