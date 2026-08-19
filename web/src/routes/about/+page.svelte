<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';

	// The LoRa parameters every radio on this mesh must match — the MeshCore
	// default North America preset, with the frequency moved to 910.425 MHz.
	const radio: { k: string; v: string }[] = [
		{ k: 'Frequency', v: '910.425 MHz' },
		{ k: 'Bandwidth', v: '62.5 kHz' },
		{ k: 'Spreading factor', v: 'SF 7' },
		{ k: 'Coding rate', v: 'CR 5' }
	];

	// The second network, joined to the one above by the wired pair at Mt Cokley.
	// Note the slower spreading factor — it is not just a different frequency, so
	// a radio set up for one will not hear the other even after retuning.
	const radio909: { k: string; v: string }[] = [
		{ k: 'Frequency', v: '909.000 MHz' },
		{ k: 'Bandwidth', v: '62.5 kHz' },
		{ k: 'Spreading factor', v: 'SF 8' },
		{ k: 'Coding rate', v: 'CR 5' }
	];
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
			Ridgeline watches the coastal-BC MeshCore mesh
		</h1>
		<p class="text-fg-dim mt-4 text-lg">
			Ridgeline is a window onto the <strong>MeshCore</strong> radio mesh as it
			grows across <strong>Vancouver Island</strong> and the
			<strong>Lower Mainland</strong>. Point it at the network and you can see
			which radios are awake, which repeaters are carrying traffic over water and
			around mountains, and where a message can still get through today.
		</p>
	</header>

	<section class="text-fg-dim space-y-4">
		<h2 class="text-fg text-xl font-700">What Ridgeline does</h2>
		<p>
			<strong>MeshCore</strong> is a protocol for cheap, low-power
			<strong>LoRa</strong> radios that pass messages hop to hop — no towers, no
			internet, no monthly bill. That suits a coastline like this one, where cell
			coverage thins out the moment you leave the highway. The trouble is that a
			mesh is mostly invisible while it runs: the traffic is in the air, not on
			any screen.
		</p>
		<p>
			Ridgeline gives it one. Receive-only stations we call <em>observers</em>
			sit and listen, then pass what they hear back to be decoded. What comes out
			is a searchable
			<a href="/nodes" class="text-signal hover:underline">node directory</a>, a
			<a href="/live" class="text-signal hover:underline">live packet feed</a>,
			interactive <a href="/map" class="text-signal hover:underline">coverage</a>
			and <a href="/live-map" class="text-signal hover:underline">signal</a> maps,
			and the <a href="/analytics" class="text-signal hover:underline">analytics</a>
			that describe the network as a whole.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Where the mesh reaches</h2>
		<p>
			The network centres on the Salish Sea. Today it runs down through Greater
			Victoria, up <strong>Vancouver Island</strong> past the Cowichan Valley and
			Nanaimo, and across the strait to Metro Vancouver and the
			<strong>Lower Mainland</strong>, held together by repeaters perched on
			ridgelines and shorelines. None of that is fixed. The footprint widens
			every time an operator raises a new node, and the maps redraw themselves to
			match.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">The alternate frequency (currently 910.425 MHz)</h2>
		<p>
			The mesh lives in the 900 MHz ISM band. Most of the regional linking
			happens on what the network treats as its
			<strong>alternate frequency</strong> — right now <strong>910.425 MHz</strong>,
			though that may change. Ridgeline follows the alt frequency, so everything
			you see here reflects it: who is transmitting, who is relaying, and how far
			each signal carries.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Radio settings (910.425 MHz)</h2>
		<p>
			Every radio on the mesh speaks the same LoRa dialect. Below are the exact
			parameters — the MeshCore <strong>default North America</strong> preset, with
			the frequency moved to <strong>910.425 MHz</strong>. A radio that doesn't
			match all four won't hear a thing, so if you're setting one up, copy them
			precisely.
		</p>
		<dl
			class="border-line/70 divide-line/60 not-prose my-2 divide-y overflow-hidden rounded-[var(--radius)] border"
		>
			{#each radio as row (row.k)}
				<div class="flex items-center justify-between gap-4 px-4 py-3">
					<dt class="text-fg-dim text-sm">{row.k}</dt>
					<dd class="text-signal font-mono text-sm font-600 tabular-nums">{row.v}</dd>
				</div>
			{/each}
		</dl>
		<p>
			It's a narrow-band profile. The 62.5&nbsp;kHz channel keeps the receiver
			sensitive enough to pull weak signals off distant ridges, while SF7 keeps
			each packet short on the air — and airtime is the scarce resource on a
			shared frequency. Short packets are what let the mesh keep growing without
			nodes talking over one another. The 4/5 coding rate adds just enough error
			correction to survive a noisy channel.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">The second network on 909 MHz</h2>
		<p>
			Not everything on the mesh is on the alternate frequency. A separate group
			of nodes runs on <strong>909.000 MHz</strong> at a slower spreading factor,
			and the two networks are joined by a pair of repeaters wired together at
			<strong>Mt Cokley</strong>, above Parksville. One listens on each frequency,
			and whatever either of them hears is handed across to the other.
		</p>
		<p>
			Every receiver feeding Ridgeline sits on the alternate frequency, so nothing
			here can hear 909 directly. A node on that side shows up only once its
			traffic has crossed the Mt Cokley link — and that crossing is exactly how we
			work out which nodes live over there. They're marked in violet everywhere
			they appear, with their frequency shown under the name.
		</p>
		<p>
			One thing worth noting if you're setting a radio up: the 909 side also uses a
			different <strong>spreading factor</strong>, so retuning the frequency alone
			isn't enough to hear it.
		</p>
		<dl
			class="border-line/70 divide-line/60 not-prose my-2 divide-y overflow-hidden rounded-[var(--radius)] border"
		>
			{#each radio909 as row (row.k)}
				<div class="flex items-center justify-between gap-4 px-4 py-3">
					<dt class="text-fg-dim text-sm">{row.k}</dt>
					<dd class="font-mono text-fg text-sm tnum">{row.v}</dd>
				</div>
			{/each}
		</dl>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Nodes, repeaters, and observers</h2>
		<p>
			A few terms worth pinning down. A <strong>node</strong> is any MeshCore
			device on the network — a handheld you carry, a base station sitting in a
			window, a solar-powered box bolted to a ridge. A <strong>repeater</strong>
			is a node built to listen and rebroadcast, extending the mesh's reach with
			every hop. An <strong>observer</strong> is the odd one out: it only
			receives, and it reports what it hears to Ridgeline, which is why the maps
			have anything to show at all. You can browse them in the
			<a href="/nodes" class="text-signal hover:underline">node directory</a> and
			the <a href="/observers" class="text-signal hover:underline">observer list</a>.
		</p>
	</section>


	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Join the network</h2>
		<p>
			Anyone within range of a compatible MeshCore radio can join. Match the four
			settings above — all of them — and the radio will begin hearing its
			neighbours and relaying their traffic. Then keep an eye on the
			<a href="/live-map" class="text-signal hover:underline">live map</a> and the
			<a href="/live" class="text-signal hover:underline">packet feed</a>: the
			first time your own node announces itself, you'll see it take its place
			among the others.
		</p>
	</section>

	<footer class="border-line text-fg-faint mt-12 border-t pt-6 text-sm">
		<p>
			Ridgeline is an independent, community-run project for the coastal British
			Columbia MeshCore mesh. <a href="/" class="text-signal hover:underline">Open the live dashboard →</a>
		</p>
	</footer>
</article>
