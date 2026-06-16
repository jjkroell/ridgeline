<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Observer } from '$lib/api';
	import { ago, fmtNum } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';

	let observers = $state<Observer[]>([]);
	let loading = $state(true);

	async function refresh() {
		try {
			observers = await api.observers();
		} finally {
			loading = false;
		}
	}
	onMount(() => {
		refresh();
		const t = setInterval(refresh, 8000);
		return () => clearInterval(t);
	});

	function fresh(lastSeen: string): boolean {
		return Date.now() - new Date(lastSeen).getTime() < 5 * 60 * 1000;
	}
</script>

<PageHeader eyebrow="Listening Posts" title="Observers">
	<div class="font-mono text-fg-dim text-xs">
		<span class="text-signal tnum">{observers.length}</span> <span class="text-fg-faint">posts</span>
	</div>
</PageHeader>

<div class="px-6 py-6 md:px-10">
	{#if loading}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">Loading…</div>
	{:else if observers.length === 0}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">
			No observers reporting yet.
		</div>
	{:else}
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each observers as o, i (o.id)}
				<div class="panel panel-hover rise px-5 py-4" style="animation-delay:{i * 40}ms">
					<div class="flex items-start justify-between">
						<div class="min-w-0">
							<div class="font-mono text-fg truncate text-sm font-bold">{o.id}</div>
							{#if o.region}
								<div class="label mt-1">{o.region}</div>
							{/if}
						</div>
						<div class="flex items-center gap-1.5">
							{#if fresh(o.lastSeen)}
								<span class="live-dot"></span>
							{:else}
								<span class="bg-fg-faint/60 h-2 w-2 rounded-full"></span>
							{/if}
						</div>
					</div>
					<div class="border-line/60 mt-4 flex items-end justify-between border-t pt-3">
						<div>
							<div class="label">Packets</div>
							<div class="font-display tnum text-fg mt-1 text-2xl font-700">
								{fmtNum(o.packetCount)}
							</div>
						</div>
						<div class="text-right">
							<div class="label">Last heard</div>
							<div class="font-mono tnum text-fg-dim mt-1 text-sm">{ago(o.lastSeen)} ago</div>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
