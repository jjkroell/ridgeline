<script lang="ts">
	import Logo from '$lib/components/Logo.svelte';
	let { name, desc = '' }: { name: string; desc?: string } = $props();
</script>

<div class="flex min-h-[60vh] flex-col items-center justify-center px-8 text-center">
	<Logo class="mb-5 h-12 w-12 opacity-80" />
	<h2 class="font-display text-fg text-lg font-700">{name}</h2>
	<p class="text-fg-faint mt-2 max-w-xs text-sm">{desc || 'This screen is being built for mobile — coming in the next pass.'}</p>
</div>
