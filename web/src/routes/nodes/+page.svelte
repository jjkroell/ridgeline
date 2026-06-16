<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Node } from '$lib/api';
	import { ago, shortKey, fmtCoord, fmtNum } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import RoleBadge from '$lib/components/RoleBadge.svelte';

	let nodes = $state<Node[]>([]);
	let loading = $state(true);
	let query = $state('');
	let roleFilter = $state<string>('all');

	async function refresh() {
		try {
			nodes = await api.nodes();
		} finally {
			loading = false;
		}
	}
	onMount(() => {
		refresh();
		const t = setInterval(refresh, 8000);
		return () => clearInterval(t);
	});

	const roles = ['all', 'Repeater', 'ChatNode', 'RoomServer', 'Sensor'];
	const roleLabels: Record<string, string> = {
		all: 'All',
		Repeater: 'Repeaters',
		ChatNode: 'Companions',
		RoomServer: 'Rooms',
		Sensor: 'Sensors'
	};

	const filtered = $derived(
		nodes.filter((n) => {
			if (roleFilter !== 'all' && n.role !== roleFilter) return false;
			if (!query) return true;
			const q = query.toLowerCase();
			return n.name.toLowerCase().includes(q) || n.publicKey.toLowerCase().includes(q);
		})
	);
</script>

<PageHeader eyebrow="Mesh Inventory" title="Nodes">
	<div class="font-mono text-fg-dim text-xs">
		<span class="text-signal tnum">{fmtNum(filtered.length)}</span>
		<span class="text-fg-faint"> / {fmtNum(nodes.length)} shown</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	<!-- Controls -->
	<div class="mb-5 flex flex-wrap items-center gap-3">
		<div class="panel flex items-center gap-2 px-3 py-2">
			<svg
				viewBox="0 0 24 24"
				class="text-fg-faint h-4 w-4"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg
			>
			<input
				bind:value={query}
				placeholder="Search name or key…"
				class="font-mono placeholder:text-fg-faint w-48 bg-transparent text-sm outline-none"
			/>
		</div>
		<div class="flex flex-wrap gap-1.5">
			{#each roles as r (r)}
				<button
					onclick={() => (roleFilter = r)}
					class="rounded-[var(--radius)] border px-3 py-1.5 text-xs transition-colors
						{roleFilter === r
						? 'border-signal/50 text-signal bg-signal/10'
						: 'border-line text-fg-dim hover:border-line-bright hover:text-fg'}"
				>
					{roleLabels[r]}
				</button>
			{/each}
		</div>
	</div>

	<!-- Table -->
	<div class="panel overflow-hidden">
		<div
			class="label border-line/70 grid grid-cols-[1fr_auto_auto_auto] gap-4 border-b px-5 py-3 md:grid-cols-[1.4fr_120px_1fr_80px_70px]"
		>
			<span>Node</span>
			<span class="hidden md:block">Role</span>
			<span class="hidden md:block">Location</span>
			<span class="text-right">Adverts</span>
			<span class="text-right">Seen</span>
		</div>

		{#if loading}
			<div class="text-fg-faint px-5 py-12 text-center text-sm">Loading…</div>
		{:else if filtered.length === 0}
			<div class="text-fg-faint px-5 py-12 text-center text-sm">No matching nodes.</div>
		{:else}
			<div class="divide-line/50 divide-y">
				{#each filtered as n (n.publicKey)}
					<a
						href="/nodes/{n.publicKey}"
						class="panel-hover grid grid-cols-[1fr_auto_auto_auto] items-center gap-4 px-5 py-3 md:grid-cols-[1.4fr_120px_1fr_80px_70px]"
					>
						<div class="min-w-0">
							<div class="text-fg truncate font-medium">{n.name || shortKey(n.publicKey)}</div>
							<div class="font-mono text-fg-faint mt-0.5 text-[0.68rem]">
								{shortKey(n.publicKey, 10, 4)}
							</div>
						</div>
						<div class="hidden md:block"><RoleBadge role={n.role} /></div>
						<div class="font-mono text-fg-dim hidden text-xs md:block tnum">
							{fmtCoord(n.latitude, n.longitude)}
						</div>
						<div class="font-mono tnum text-fg-dim text-right text-sm">{n.advertCount}</div>
						<div class="font-mono tnum text-fg-faint text-right text-xs">{ago(n.lastSeen)}</div>
					</a>
				{/each}
			</div>
		{/if}
	</div>
</div>
