<script lang="ts">
	// /about became /settings. The server redirects a direct visit; this covers
	// in-app navigation from a client still running an older build. Old
	// #bridging / #traffic links go on to the FAQ (see routes/settings).
	import { onMount } from 'svelte';
	onMount(() => location.replace('/settings' + location.hash));
</script>
