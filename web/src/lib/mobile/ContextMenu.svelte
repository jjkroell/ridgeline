<script lang="ts">
	// The one context menu (see context-menu.svelte.ts). A press and hold
	// echoes the row as a lifted card in place over a blurred page, with the
	// actions beside it — below when there's room, above when the row is low.
	// A right-click gets a plain menu at the pointer, as on a Mac.
	import { contextMenu } from './context-menu.svelte';

	const m = $derived(contextMenu.current);
	let menuH = $state(0);
	let menuEl = $state<HTMLElement>();
	let downHere = false;
	const pos = $derived.by(() => {
		if (!m) return '';
		const vw = window.innerWidth;
		const vh = window.innerHeight;
		if (m.at) {
			const w = 220;
			const left = Math.min(m.at.x, vw - w - 8);
			const top = m.at.y + menuH > vh - 8 ? Math.max(8, m.at.y - menuH) : m.at.y;
			return `top:${top}px;left:${left}px;width:${w}px`;
		}
		const r = m.rect;
		const below = r.bottom + 8 + menuH < vh - 16;
		const top = below ? r.bottom + 8 : Math.max(16, r.top - 8 - menuH);
		return `top:${top}px;left:${Math.max(12, r.left)}px;width:min(260px, calc(100vw - 24px))`;
	});

	async function run(i: { run: () => void | Promise<void> }) {
		contextMenu.close();
		await i.run();
	}
	function key(e: KeyboardEvent) {
		if (e.key === 'Escape') contextMenu.close();
		if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
		e.preventDefault();
		const items = [...(menuEl?.querySelectorAll<HTMLElement>('[role=menuitem]') ?? [])];
		const i = items.indexOf(document.activeElement as HTMLElement);
		items[(i + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length]?.focus();
	}
</script>

<svelte:window onkeydown={m ? key : undefined} onscroll={() => contextMenu.close()} onresize={() => contextMenu.close()} />

{#if m?.at}
	<!-- Any press outside closes it, a right-click elsewhere included. -->
	<div
		class="fixed inset-0 z-[60]"
		role="presentation"
		onpointerdown={() => contextMenu.close()}
		oncontextmenu={(e) => {
			e.preventDefault();
			contextMenu.close();
		}}
	></div>
	<div
		data-cm-menu
		data-cm-desktop
		role="menu"
		aria-label={m.title}
		bind:this={menuEl}
		bind:offsetHeight={menuH}
		class="border-line/60 bg-ink-2/95 fixed z-[62] overflow-hidden rounded-xl border p-1 shadow-2xl backdrop-blur-xl"
		style={pos}
	>
		<div class="text-fg-faint truncate px-2.5 pt-1 pb-1.5 text-[0.7rem] font-600">{m.title}</div>
		{#each m.items as i, n (i.label)}
			<!-- svelte-ignore a11y_autofocus -->
			<button role="menuitem" autofocus={n === 0} onclick={() => run(i)} class="text-fg hover:bg-signal focus-visible:bg-signal flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-[0.82rem] outline-none hover:text-[var(--on-signal,var(--color-ink))] focus-visible:text-[var(--on-signal,var(--color-ink))]">
				<svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0 opacity-75" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d={i.icon} /></svg>
				{i.label}
			</button>
		{/each}
	</div>
{:else if m}
	<!-- Close only on a tap that STARTED here: lifting the finger that opened
	     the menu lands on this backdrop too, and must not dismiss it. -->
	<button
		data-cm-backdrop
		class="fixed inset-0 z-[60] backdrop-blur-md"
		aria-label="Close menu"
		onpointerdown={() => (downHere = true)}
		onclick={() => {
			if (downHere) contextMenu.close();
			downHere = false;
		}}
	></button>
	<div
		data-cm-card
		class="border-line/60 bg-panel pointer-events-none fixed z-[61] rounded-2xl border px-4 py-3 shadow-2xl"
		style="top:{m.rect.top}px;left:{m.rect.left}px;width:{m.rect.width}px"
		aria-hidden="true"
	>
		<div class="text-fg truncate text-sm font-600">{m.title}</div>
		{#if m.subtitle}<div class="text-fg-faint mt-0.5 truncate font-mono text-[0.62rem]">{m.subtitle}</div>{/if}
	</div>
	<div
		data-cm-menu
		role="menu"
		aria-label={m.title}
		bind:this={menuEl}
		bind:offsetHeight={menuH}
		class="border-line/60 bg-ink-2/95 divide-line/50 fixed z-[62] divide-y overflow-hidden rounded-2xl border shadow-2xl backdrop-blur-xl"
		style={pos}
	>
		{#each m.items as i, n (i.label)}
			<!-- svelte-ignore a11y_autofocus -->
			<button role="menuitem" autofocus={n === 0} onclick={() => run(i)} class="text-fg active:bg-line/50 hover:bg-line/30 flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-[0.95rem]">
				{i.label}
				<svg viewBox="0 0 24 24" class="text-fg-dim h-[18px] w-[18px] shrink-0" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d={i.icon} /></svg>
			</button>
		{/each}
	</div>
{/if}

<style>
	[data-cm-backdrop] {
		background: color-mix(in srgb, var(--color-ink) 40%, transparent);
		animation: cm-fade 0.18s ease-out;
	}
	[data-cm-card] {
		animation: cm-lift 0.22s cubic-bezier(0.2, 0.8, 0.2, 1) both;
	}
	[data-cm-menu] {
		animation: cm-pop 0.24s cubic-bezier(0.2, 0.8, 0.2, 1) both;
		transform-origin: top left;
	}
	@keyframes cm-fade {
		from {
			opacity: 0;
		}
	}
	@keyframes cm-lift {
		to {
			transform: scale(1.03);
		}
	}
	[data-cm-desktop] {
		animation-duration: 0.12s;
	}
	@keyframes cm-pop {
		from {
			opacity: 0;
			transform: scale(0.85);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		[data-cm-backdrop],
		[data-cm-card],
		[data-cm-menu] {
			animation: none;
		}
	}
</style>
