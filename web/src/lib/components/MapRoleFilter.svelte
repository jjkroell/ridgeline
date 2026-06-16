<script lang="ts">
	import { roleColor } from '$lib/format';

	// `selected` is the set of role keys currently shown.
	let { selected = $bindable() }: { selected: Set<string> } = $props();

	const ROLES: [string, string][] = [
		['Repeater', 'Repeaters'],
		['RoomServer', 'Rooms'],
		['ChatNode', 'Companions'],
		['Sensor', 'Sensors']
	];

	function toggle(key: string) {
		const s = new Set(selected);
		if (s.has(key)) s.delete(key);
		else s.add(key);
		selected = s;
	}
</script>

<div
	class="border-line bg-ink-2/85 absolute top-3 left-3 z-10 flex flex-wrap gap-1 rounded-[var(--radius)] border p-1 backdrop-blur-md"
>
	{#each ROLES as [key, label] (key)}
		{@const on = selected.has(key)}
		{@const c = roleColor(key)}
		<button
			onclick={() => toggle(key)}
			class="flex items-center gap-1.5 rounded-[var(--radius)] px-2 py-1 text-[0.68rem] font-medium transition-colors {on
				? ''
				: 'text-fg-faint hover:text-fg-dim'}"
			style={on ? `color:${c};background:color-mix(in srgb, ${c} 14%, transparent)` : ''}
		>
			<span
				class="inline-block h-1.5 w-1.5 rounded-full"
				style="background:{on ? c : 'var(--color-fg-faint)'}"
			></span>
			{label}
		</button>
	{/each}
</div>
