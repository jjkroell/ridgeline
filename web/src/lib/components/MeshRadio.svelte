<script lang="ts">
	// The radio settings every node and observer on this mesh shares, shown once
	// for the whole site instead of a label on each node. Renders nothing unless
	// the server reports exactly one preset (see /api/health meshRadio).
	import { env } from '$lib/env.svelte';
	import { fmtMeshRadio } from '$lib/format';

	let { class: cls = '' }: { class?: string } = $props();
	const text = $derived(fmtMeshRadio(env.meshRadio));
</script>

{#if text}
	<div class={cls} title="Every node and observer on this mesh uses these radio settings">
		<div class="label">Mesh radio</div>
		<div class="text-fg-dim tnum mt-1 font-mono text-[0.7rem] leading-snug">{text}</div>
	</div>
{/if}
