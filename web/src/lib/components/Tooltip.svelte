<script lang="ts">
	import type { Snippet } from 'svelte';

	let { text, children }: { text: string; children: Snippet } = $props();

	let show = $state(false);
	let pos = $state({ x: 0, y: 0 });

	function enter(e: MouseEvent) {
		const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
		pos = { x: r.left + r.width / 2, y: r.top };
		show = true;
	}
</script>

<span class="inline-flex" onmouseenter={enter} onmouseleave={() => (show = false)} role="note">
	{@render children()}
</span>

{#if show}
	<div
		class="border-line-bright bg-ink-2 text-fg-dim pointer-events-none fixed z-[100] max-w-[250px] rounded-[var(--radius)] border px-2.5 py-1.5 text-xs leading-snug shadow-xl"
		style="left:{pos.x}px;top:{pos.y}px;transform:translate(-50%,calc(-100% - 9px))"
	>
		{text}
		<span
			class="bg-ink-2 border-line-bright absolute top-full left-1/2 -mt-[5px] h-2 w-2 -translate-x-1/2 rotate-45 border-r border-b"
		></span>
	</div>
{/if}
