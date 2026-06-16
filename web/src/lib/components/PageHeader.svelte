<script lang="ts">
	interface Props {
		eyebrow: string;
		title: string;
		children?: import('svelte').Snippet;
	}
	let { eyebrow, title, children }: Props = $props();
</script>

<header class="border-line/70 flex flex-wrap items-end justify-between gap-4 border-b px-6 py-6 md:px-10">
	<div class="rise">
		<div class="label mb-2 flex items-center gap-2">
			<span class="bg-signal/70 inline-block h-px w-6"></span>
			{eyebrow}
		</div>
		<h1 class="font-display text-fg text-3xl font-700 tracking-tight md:text-4xl">{title}</h1>
	</div>
	{#if children}
		<div class="rise flex items-center gap-3" style="animation-delay:60ms">
			{@render children()}
		</div>
	{/if}
</header>
