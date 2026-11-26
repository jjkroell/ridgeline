<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import MaydaySurvey from '$lib/components/MaydaySurvey.svelte';
</script>

<Seo title="Emergency channel survey" description="Tell us how you used the #sos emergency channel, and what you would want from an emergency channel on the mesh." path="/survey" />

<article class="mx-auto max-w-2xl px-6 py-12">
	<header class="mb-8">
		<p class="label text-coral mb-2">Survey</p>
		<h1 class="font-display text-fg text-3xl font-900 tracking-tight sm:text-4xl">The #sos emergency channel</h1>
		<p class="text-fg-dim mt-3 leading-relaxed">Ridgeline no longer monitors #sos. Before deciding whether to run an emergency channel again, we would like to hear how you used it and what you would want. Six quick questions, all optional; it takes about two minutes.</p>
	</header>
	<MaydaySurvey />
	<p class="text-fg-faint mt-8 text-xs leading-relaxed">
		Answers are read only by Ridgeline’s admins. Your email, if you give one, is used only to reply to you. See
		<a href="/privacy" class="text-signal hover:underline">Privacy</a>.
	</p>
</article>
