<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';
</script>

<Seo
	title="About Ridgeline — MeshCore mesh for Vancouver Island & the Lower Mainland"
	description="Ridgeline is a live observatory for the MeshCore LoRa mesh network across coastal British Columbia — nodes, repeaters, coverage and packets on the alternate frequency (currently 910.425 MHz)."
	path="/about"
/>

<article class="mx-auto max-w-3xl px-6 py-12 leading-relaxed">
	<header class="mb-10">
		<p class="label text-signal mb-2">MeshCore Observatory</p>
		<h1 class="font-display text-fg text-3xl font-900 tracking-tight sm:text-4xl">
			Ridgeline: a live observatory for the coastal BC MeshCore mesh
		</h1>
		<p class="text-fg-dim mt-4 text-lg">
			Ridgeline maps and measures the <strong>MeshCore</strong> LoRa radio mesh
			as it spreads across <strong>Vancouver Island</strong> and the
			<strong>Lower Mainland</strong> of British Columbia — every node, every
			repeater, and every packet, in real time.
		</p>
	</header>

	<section class="text-fg-dim space-y-4">
		<h2 class="text-fg text-xl font-700">What is Ridgeline?</h2>
		<p>
			Ridgeline is a free, open observatory for a community-run
			<strong>MeshCore</strong> mesh network. MeshCore is a long-range, low-power
			<strong>LoRa</strong> radio protocol that lets inexpensive devices relay
			messages hop-by-hop with no cell towers, no internet, and no monthly fees —
			ideal for off-grid communication across the mountains, inlets, and islands
			of coastal British Columbia.
		</p>
		<p>
			Passive receivers (called <em>observers</em>) listen to the mesh and feed
			what they hear into Ridgeline, which decodes the traffic and turns it into
			a searchable <a href="/nodes" class="text-signal hover:underline">node directory</a>,
			a <a href="/live" class="text-signal hover:underline">live packet feed</a>,
			interactive <a href="/map" class="text-signal hover:underline">coverage</a> and
			<a href="/live-map" class="text-signal hover:underline">signal</a> maps, and
			network <a href="/analytics" class="text-signal hover:underline">analytics</a>.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Where the mesh reaches</h2>
		<p>
			The network is centred on the Salish Sea region of southwestern British
			Columbia. Coverage grows as operators add nodes, but today it spans
			<strong>Vancouver Island</strong> — including the Greater Victoria area,
			the Cowichan Valley, Nanaimo and up-island — and the
			<strong>Lower Mainland</strong> around Metro Vancouver, linked by
			mountaintop and shoreline repeaters across the water.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">The alternate frequency (currently 910.425 MHz)</h2>
		<p>
			The mesh runs in the 900 MHz ISM band. Ridgeline tracks its
			<strong>alternate frequency</strong> — currently
			<strong>910.425 MHz</strong> — used for regional links across coastal
			British Columbia. Watching <strong>910.425</strong> activity in Ridgeline
			shows which nodes and repeaters are active on the alt frequency and how
			traffic flows across the mesh.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Nodes, repeaters, and observers</h2>
		<p>
			A <strong>node</strong> is any MeshCore device on the network — a handheld
			companion radio, a fixed station, or a solar repeater on a ridge. A
			<strong>repeater</strong> extends range by rebroadcasting packets it hears.
			An <strong>observer</strong> is a receive-only station that reports what it
			hears to Ridgeline, giving the map its coverage picture. Browse them all in
			the <a href="/nodes" class="text-signal hover:underline">node directory</a> and
			<a href="/observers" class="text-signal hover:underline">observer list</a>.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Join the network</h2>
		<p>
			Anyone in range can join with a compatible MeshCore LoRa radio. Set your
			device to the mesh's alternate frequency (currently 910.425 MHz), and
			you'll start relaying and receiving mesh traffic. Once you're on the air,
			watch yourself appear on the
			<a href="/live-map" class="text-signal hover:underline">live map</a> and in the
			<a href="/live" class="text-signal hover:underline">packet feed</a>.
		</p>
	</section>

	<footer class="border-line text-fg-faint mt-12 border-t pt-6 text-sm">
		<p>
			Ridgeline is an independent, community project for the coastal British
			Columbia MeshCore mesh. <a href="/" class="text-signal hover:underline">Open the live dashboard →</a>
		</p>
	</footer>
</article>
