<script lang="ts">
	// ⌘K / Ctrl+K quick-jump (Spotlight-style): type to find a page or any node
	// by name or key prefix; arrows move, Enter opens, Esc closes. Nodes are
	// fetched on first open and kept for the session.
	import { goto } from '$app/navigation';
	import { tick } from 'svelte';
	import { api, type Node } from '$lib/api';
	import { roleColor, roleLabel, shortKey } from '$lib/format';

	let { pages }: { pages: { href: string; label: string }[] } = $props();

	let open = $state(false);
	let q = $state('');
	let sel = $state(0);
	let nodes = $state<Node[] | null>(null);
	let input = $state<HTMLInputElement>();

	export async function show() {
		open = true;
		q = '';
		sel = 0;
		await tick();
		input?.focus();
		if (!nodes) nodes = await api.nodes().catch(() => []);
	}

	type Hit = { kind: 'page' | 'node'; href: string; label: string; sub: string; color?: string };
	const hits = $derived.by((): Hit[] => {
		const s = q.trim().toLowerCase();
		const pg: Hit[] = pages
			.filter((p) => !s || p.label.toLowerCase().includes(s))
			.map((p) => ({ kind: 'page', href: p.href, label: p.label, sub: 'Page' }));
		if (!s) return pg;
		const nd: Hit[] = (nodes ?? [])
			.filter((n) => n.name?.toLowerCase().includes(s) || n.publicKey.toLowerCase().startsWith(s))
			.sort((a, b) => Number(!a.name?.toLowerCase().startsWith(s)) - Number(!b.name?.toLowerCase().startsWith(s)))
			.slice(0, 8)
			.map((n) => ({
				kind: 'node',
				href: '/nodes/' + n.publicKey,
				label: n.name || shortKey(n.publicKey),
				sub: `${roleLabel(n.role)} · ${shortKey(n.publicKey)}`,
				color: roleColor(n.role)
			}));
		return [...pg, ...nd];
	});
	$effect(() => {
		q;
		sel = 0;
	});

	function go(h: Hit | undefined) {
		if (!h) return;
		open = false;
		goto(h.href);
	}

	function windowKey(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			if (open) open = false;
			else show();
		}
	}
	function listKey(e: KeyboardEvent) {
		if (e.key === 'Escape') open = false;
		else if (e.key === 'ArrowDown') {
			e.preventDefault();
			sel = Math.min(hits.length - 1, sel + 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			sel = Math.max(0, sel - 1);
		} else if (e.key === 'Enter') go(hits[sel]);
	}
</script>

<svelte:window onkeydown={windowKey} />

{#if open}
	<div class="fixed inset-0 z-[90] flex items-start justify-center bg-black/40 px-4 pt-[14vh] backdrop-blur-[2px]">
		<button class="absolute inset-0 cursor-default" aria-label="Close search" onclick={() => (open = false)}></button>
		<div
			data-palette
			class="border-line bg-panel relative w-full max-w-xl overflow-hidden rounded-[var(--radius)] border shadow-2xl"
			role="dialog"
			aria-modal="true"
			aria-label="Search"
		>
			<div class="border-line/70 flex items-center gap-3 border-b px-4">
				<svg viewBox="0 0 24 24" class="text-fg-faint h-5 w-5 shrink-0" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="M21 21l-4-4" /></svg>
				<input
					bind:this={input}
					bind:value={q}
					onkeydown={listKey}
					placeholder="Search nodes and pages…"
					class="text-fg placeholder:text-fg-faint min-w-0 flex-1 bg-transparent py-4 text-base outline-none"
					role="combobox"
					aria-expanded="true"
					aria-controls="palette-list"
					aria-activedescendant={hits.length ? `palette-${sel}` : undefined}
				/>
				<kbd class="border-line text-fg-faint rounded border px-1.5 py-0.5 text-[0.65rem]">esc</kbd>
			</div>
			<ul id="palette-list" class="max-h-[50vh] overflow-y-auto p-2" role="listbox">
				{#each hits as h, i (h.href)}
					<li id="palette-{i}" role="option" aria-selected={i === sel}>
						<button
							onclick={() => go(h)}
							onmousemove={() => (sel = i)}
							class="flex w-full items-center gap-3 rounded-[var(--radius)] px-3 py-2 text-left {i === sel ? 'bg-signal/12' : ''}"
						>
							{#if h.kind === 'node'}
								<span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background:{h.color}"></span>
							{:else}
								<svg viewBox="0 0 24 24" class="text-fg-faint h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
							{/if}
							<span class="text-fg min-w-0 flex-1 truncate text-sm {i === sel ? 'font-600' : ''}">{h.label}</span>
							<span class="text-fg-faint shrink-0 font-mono text-[0.68rem]">{h.sub}</span>
						</button>
					</li>
				{:else}
					<li class="text-fg-faint px-3 py-6 text-center text-sm">
						{nodes ? 'No matches.' : 'Loading nodes…'}
					</li>
				{/each}
			</ul>
		</div>
	</div>
{/if}
