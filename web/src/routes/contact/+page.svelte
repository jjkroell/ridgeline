<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
	import ContactForm from '$lib/components/ContactForm.svelte';
</script>

<Seo
	title="Contact us"
	description="Questions, problems or feedback about Ridgeline or the coastal BC MeshCore mesh? Send us a message."
	path="/contact"
/>

<article class="mx-auto max-w-2xl px-6 py-12">
	<header class="mb-6">
		<p class="label text-signal mb-2">Help</p>
		<h1 class="font-display text-fg text-3xl font-900 tracking-tight sm:text-4xl">Contact us</h1>
		<p class="text-fg-dim mt-3 leading-relaxed">
			Something not working, a question about the mesh, or an idea for Ridgeline? Send a message
			and we will get back to you by email.
		</p>
	</header>
	<ContactForm />
	<p class="text-fg-faint mt-8 text-xs leading-relaxed">
		Your email is used only to reply to you. See <a href="/privacy" class="text-signal hover:underline">Privacy</a>.
	</p>
</article>
