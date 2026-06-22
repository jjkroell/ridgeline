<script lang="ts">
	import { page } from '$app/state';
	import NodeDetail from '$lib/components/NodeDetail.svelte';

	const pubkey = $derived(page.params.pubkey ?? '');
</script>

<div class="px-4 py-4">
	{#key pubkey}
		<NodeDetail {pubkey} heading />
	{/key}
</div>
