<script lang="ts">
	let { type }: { type: string } = $props();

	const colors: Record<string, string> = {
		Advert: 'var(--color-signal)',
		TextMessage: 'var(--color-sky)',
		GroupText: 'var(--color-violet)',
		Trace: 'var(--color-amber)',
		Ack: 'var(--color-fg-dim)',
		Request: 'var(--color-lime)',
		Response: 'var(--color-lime)',
		Path: 'var(--color-fg-dim)'
	};
	const color = $derived(colors[type] ?? 'var(--color-fg-faint)');
</script>

<span
	class="font-mono rounded-[var(--radius)] px-1.5 py-0.5 text-[0.66rem] tracking-wide"
	style="color:{color}; background:color-mix(in srgb, {color} 10%, transparent)"
>
	{type}
</span>
