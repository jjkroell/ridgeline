<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import MaydaySurvey from '$lib/components/MaydaySurvey.svelte';
</script>

<Seo title="Emergency channel survey" description="Tell us how you used the #sos emergency channel, and what you would want from an emergency channel on the mesh." path="/survey" />

<div class="px-4 py-4">
	<p class="text-fg-dim mb-5 text-sm leading-relaxed">Ridgeline no longer monitors #sos. Before deciding whether to run an emergency channel again, we would like to hear how you used it and what you would want. Six quick questions, all optional; it takes about two minutes.</p>
	<MaydaySurvey compact />
	<p class="text-fg-faint mt-6 text-xs leading-relaxed">
		Answers are read only by Ridgeline’s admins. Your email, if you give one, is used only to reply to you. See
		<a href="/privacy" class="text-signal">Privacy</a>.
	</p>
</div>
