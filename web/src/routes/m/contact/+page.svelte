<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import ContactForm from '$lib/components/ContactForm.svelte';
</script>

<Seo title="Contact us" description="Questions, problems or feedback about Ridgeline? Send us a message." path="/contact" />

<div class="px-4 py-4">
	<p class="text-fg-dim mb-4 text-sm leading-relaxed">
		Something not working, a question about the mesh, or an idea for Ridgeline? Send a message and
		we will get back to you by email.
	</p>
	<ContactForm />
	<p class="text-fg-faint mt-6 text-xs leading-relaxed">
		Your email is used only to reply to you. See <a href="/privacy" class="text-signal">Privacy</a>.
	</p>
</div>
