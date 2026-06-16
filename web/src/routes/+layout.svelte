<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { live } from '$lib/live.svelte';

	let { children } = $props();

	onMount(() => live.start());

	const nav = [
		{ href: '/', label: 'Overview', exact: true, icon: 'grid' },
		{ href: '/nodes', label: 'Nodes', icon: 'node' },
		{ href: '/live', label: 'Live Feed', icon: 'pulse' },
		{ href: '/map', label: 'Map', icon: 'map' },
		{ href: '/observers', label: 'Observers', icon: 'eye' }
	];

	function active(href: string, exact = false): boolean {
		const p = page.url.pathname;
		return exact ? p === href : p === href || p.startsWith(href + '/');
	}

	const icons: Record<string, string> = {
		grid: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
		node: 'M12 2v6m0 8v6M2 12h6m8 0h6M12 8a4 4 0 100 8 4 4 0 000-8z',
		pulse: 'M2 12h4l3 8 4-16 3 8h6',
		map: 'M9 4 3 6v14l6-2 6 2 6-2V4l-6 2-6-2zM9 4v14M15 6v14',
		eye: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7-10-7-10-7zM12 9a3 3 0 100 6 3 3 0 000-6z'
	};
</script>

<div class="flex min-h-screen">
	<!-- Sidebar -->
	<aside
		class="border-line/80 bg-ink-2/70 sticky top-0 hidden h-screen w-[220px] shrink-0 flex-col border-r backdrop-blur-sm md:flex"
	>
		<a href="/" class="group flex items-center gap-3 px-5 pt-6 pb-5">
			<svg viewBox="0 0 32 32" class="h-8 w-8" aria-hidden="true">
				<path
					d="M3 23 L11 11 L16 17 L22 7 L29 23"
					fill="none"
					stroke="var(--color-signal)"
					stroke-width="2.2"
					stroke-linejoin="round"
					stroke-linecap="round"
				/>
				<path
					d="M3 27 L11 17 L16 22 L22 14 L29 27"
					fill="none"
					stroke="var(--color-amber)"
					stroke-width="1.4"
					stroke-linejoin="round"
					stroke-linecap="round"
					opacity="0.6"
				/>
				<circle cx="22" cy="7" r="2.4" fill="var(--color-signal)" />
			</svg>
			<div class="leading-none">
				<div class="font-display text-fg text-[1.05rem] font-900 tracking-tight">RIDGELINE</div>
				<div class="label mt-1">MeshCore Observatory</div>
			</div>
		</a>

		<nav class="mt-3 flex flex-col gap-0.5 px-3">
			{#each nav as item (item.href)}
				{@const on = active(item.href, item.exact)}
				<a
					href={item.href}
					class="group relative flex items-center gap-3 rounded-[var(--radius)] px-3 py-2.5 text-sm transition-colors
						{on ? 'text-fg' : 'text-fg-dim hover:text-fg'}"
				>
					{#if on}
						<span class="bg-signal absolute top-1/2 left-0 h-5 w-[2px] -translate-y-1/2 rounded-full"
						></span>
					{/if}
					<svg
						viewBox="0 0 24 24"
						class="h-[18px] w-[18px] shrink-0 transition-colors {on
							? 'text-signal'
							: 'text-fg-faint group-hover:text-fg-dim'}"
						fill="none"
						stroke="currentColor"
						stroke-width="1.6"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<path d={icons[item.icon]} />
					</svg>
					<span class="font-medium">{item.label}</span>
				</a>
			{/each}
		</nav>

		<div class="mt-auto px-5 pb-5">
			<div class="border-line flex items-center gap-2 border-t pt-4">
				{#if live.connected}
					<span class="live-dot"></span>
					<span class="label !text-signal">Live</span>
				{:else}
					<span class="bg-coral/70 inline-block h-2 w-2 rounded-full"></span>
					<span class="label !text-coral">Offline</span>
				{/if}
				<span class="label ml-auto tnum">{live.total}</span>
			</div>
		</div>
	</aside>

	<!-- Main -->
	<div class="flex min-w-0 flex-1 flex-col">
		<!-- Mobile top bar -->
		<header
			class="border-line bg-ink-2/80 flex items-center gap-4 border-b px-4 py-3 backdrop-blur-sm md:hidden"
		>
			<a href="/" class="font-display font-900 tracking-tight">RIDGELINE</a>
			<nav class="ml-auto flex gap-3 text-xs">
				{#each nav as item (item.href)}
					<a
						href={item.href}
						class={active(item.href, item.exact) ? 'text-signal' : 'text-fg-dim'}>{item.label}</a
					>
				{/each}
			</nav>
		</header>

		<main class="flex-1">
			{@render children()}
		</main>
	</div>
</div>
