<script lang="ts">
	// Search field for a phone list page. In Cupertino it sits under the
	// large title and starts tucked away, as in Mail or Contacts: pull the
	// list down and it slides out; once it is out, the next pull refreshes.
	// Cancel clears it and tucks it away again. Other themes show it always.
	import { onMount, type Snippet } from 'svelte';
	import { theme } from '$lib/theme.svelte';

	let {
		value = $bindable(''),
		placeholder = 'Search',
		trailing
	}: { value?: string; placeholder?: string; trailing?: Snippet } = $props();

	const FIELD = 46; // field + its gap below
	let wrap = $state<HTMLElement>();
	let input = $state<HTMLInputElement>();
	let open = $state(false);
	let focused = $state(false);
	let reveal = $state(0);
	let pulling = $state(false);

	const tuck = $derived(theme.isApple);
	const collapsed = $derived(tuck && !open && !value);
	const height = $derived(!tuck ? null : collapsed ? reveal : FIELD);

	onMount(() => {
		const main = wrap?.closest<HTMLElement>('[data-m-main]');
		if (!main) return;
		let y0: number | null = null;
		const start = (e: TouchEvent) => {
			y0 = collapsed && main.scrollTop <= 0 ? e.touches[0].clientY : null;
		};
		const move = (e: TouchEvent) => {
			if (y0 == null) return;
			pulling = true;
			reveal = Math.max(0, Math.min(FIELD, (e.touches[0].clientY - y0) * 0.6));
		};
		const end = () => {
			if (y0 == null) return;
			y0 = null;
			pulling = false;
			if (reveal > FIELD / 2) open = true;
			reveal = 0;
		};
		main.addEventListener('touchstart', start, { passive: true });
		main.addEventListener('touchmove', move, { passive: true });
		main.addEventListener('touchend', end);
		main.addEventListener('touchcancel', end);
		return () => {
			main.removeEventListener('touchstart', start);
			main.removeEventListener('touchmove', move);
			main.removeEventListener('touchend', end);
			main.removeEventListener('touchcancel', end);
		};
	});

	function cancel() {
		value = '';
		input?.blur();
		open = false;
	}
</script>

<div
	bind:this={wrap}
	data-pull-search
	data-collapsed={collapsed}
	class={tuck ? 'overflow-hidden' : 'mb-2'}
	style:height={height == null ? undefined : `${height}px`}
	style:transition={pulling ? 'none' : 'height 0.3s cubic-bezier(0.32, 0.72, 0, 1)'}
>
	<div class="flex items-center gap-2" style:opacity={tuck && collapsed ? Math.min(1, reveal / FIELD) : undefined}>
		<div data-search class="border-line bg-panel flex min-w-0 flex-1 items-center gap-2 rounded-xl border px-3 py-2.5">
			<svg viewBox="0 0 24 24" class="text-fg-faint h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="M21 21l-4-4" /></svg>
			<input
				bind:this={input}
				bind:value
				{placeholder}
				type="search"
				enterkeyhint="search"
				tabindex={collapsed ? -1 : undefined}
				onfocus={() => (focused = true)}
				onblur={() => (focused = false)}
				class="text-fg placeholder:text-fg-faint min-w-0 flex-1 bg-transparent text-sm outline-none [&::-webkit-search-cancel-button]:hidden"
			/>
			{#if value}<button onclick={() => (value = '')} aria-label="Clear search" class="text-fg-faint -my-1 text-lg leading-none">×</button>{/if}
			{@render trailing?.()}
		</div>
		{#if tuck && (focused || value)}
			<button onclick={cancel} class="text-signal shrink-0 text-[0.95rem]">Cancel</button>
		{/if}
	</div>
</div>
