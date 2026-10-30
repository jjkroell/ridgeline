<!--
  Hands a card its Cupertino widget size. WidgetShell reads it from context, so
  the individual widgets never need to know which size they're drawn at.
-->
<script lang="ts">
	import { setContext, type Snippet } from 'svelte';
	import type { WidgetShape } from '$lib/overview.svelte';

	let { shape, children }: { shape: WidgetShape; children: Snippet } = $props();
	setContext('widget-shape', () => shape);
</script>

{@render children()}
