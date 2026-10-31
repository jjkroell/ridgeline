<script lang="ts">
	import PageHeader from '$lib/components/PageHeader.svelte';
	import SosGuide from '$lib/components/SosGuide.svelte';
	import Seo from '$lib/components/Seo.svelte';
</script>

<Seo
	title="The #sos emergency channel"
	description="How the MeshCore #sos channel works in coastal BC: how to join and monitor it, the trigger words, and what happens to a call for help — Ridgeline alerts, emails and the auto reply."
	path="/sos"
/>

<PageHeader eyebrow="Guide" title="The #sos channel">
	{#snippet children()}
		<a
			href="/channels?channel=sos"
			class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
			>Open #sos →</a
		>
	{/snippet}
</PageHeader>

<SosGuide />
