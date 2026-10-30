<script lang="ts">
	// Ridgeline mark, "Ridge to Signal": a ridgeline climbs to a summit node and
	// leaves it as a radio wave — terrain turning into signal. The ridge takes
	// the theme accent, the wave --logo-b (amber, unless the accent is too close
	// to it; see app.css).
	let { class: cls = 'h-8 w-8' }: { class?: string } = $props();
</script>

<svg viewBox="0 0 32 32" class={cls} aria-hidden="true">
	<path
		d="M2.5 24 8.5 15l3.5 4 4-8"
		fill="none"
		stroke="var(--color-signal)"
		stroke-width="2.4"
		stroke-linecap="round"
		stroke-linejoin="round"
	/>
	<path
		d="M16 11c2 0 2.2 11 4.8 11s2.6-8 4.8-8c2 0 2 5 3.9 5"
		fill="none"
		stroke="var(--logo-b, var(--color-amber))"
		stroke-width="2.4"
		stroke-linecap="round"
	/>
	<circle cx="16" cy="11" r="2.3" fill="var(--color-signal)" />
</svg>
