<script lang="ts">
	import Seo from '$lib/components/Seo.svelte';

	// The LoRa parameters every radio on this mesh must match — the MeshCore
	// default North America preset, with the frequency set to 909.000 MHz. The
	// mesh moved here from 910.425 MHz on 1 October 2026; there is one preset
	// now, so there is one table.
	const radio: { k: string; v: string }[] = [
		{ k: 'Frequency', v: '909.000 MHz' },
		{ k: 'Bandwidth', v: '62.5 kHz' },
		{ k: 'Spreading factor', v: 'SF 7' },
		{ k: 'Coding rate', v: 'CR 5' }
	];

	// One full day of traffic as Ridgeline stored it: 19 November 2026,
	// 00:00–24:00 UTC. A static snapshot (counted from the observations table),
	// not live data — update the date with the numbers if this is ever refreshed.
	// "obs" = observations: one observer hearing one copy of a packet.
	const day = { date: '19 November 2026', packets: 11063, obs: 514759 };
	const traffic: { name: string; what: string; packets: number; obs: number; flood?: boolean }[] = [
		{ name: 'Requests', what: 'A node asking another for status, telemetry or a login. Encrypted.', packets: 2622, obs: 185796, flood: true },
		{ name: 'Channel messages', what: 'Text on Public and hashtag channels.', packets: 2159, obs: 146168, flood: true },
		{ name: 'Path returns', what: 'The route back, sent after a flood reaches the node it was for.', packets: 755, obs: 50268, flood: true },
		{ name: 'Channel data', what: 'Non-text data sent on a channel.', packets: 695, obs: 43889, flood: true },
		{ name: 'Direct messages', what: 'Private text between two people.', packets: 621, obs: 32450, flood: true },
		{ name: 'Adverts', what: 'A node announcing its name, type and location.', packets: 1356, obs: 32407 },
		{ name: 'Responses', what: 'Replies to requests, usually sent along a known path.', packets: 1360, obs: 8258 },
		{ name: 'Anonymous requests', what: 'Guest logins to rooms and repeaters.', packets: 177, obs: 6620, flood: true },
		{ name: 'Acks', what: 'Acknowledgements: delivery receipts for direct messages.', packets: 97, obs: 6010, flood: true },
		{ name: 'Discovery', what: 'Nodes asking who is nearby, and the replies.', packets: 1095, obs: 2599 },
		{ name: 'Other', what: 'Traces, multipart, firmware updates and unrecognised packets.', packets: 139, obs: 294 }
	];
	const fmt = (n: number) => n.toLocaleString('en-CA');
	const pct = (n: number, of: number) => ((n / of) * 100).toFixed(1) + '%';
</script>

<Seo
	title="About Ridgeline — MeshCore mesh for Vancouver Island & the Lower Mainland"
	description="Ridgeline is a live observatory for the MeshCore LoRa mesh network across coastal British Columbia — nodes, repeaters, coverage and packets on 909.000 MHz."
	path="/about"
/>

<article class="mx-auto max-w-3xl px-6 py-12 leading-relaxed">
	<header class="mb-10">
		<p class="label text-signal mb-2">MeshCore Observatory</p>
		<h1 class="font-display text-fg text-3xl font-900 tracking-tight sm:text-4xl">
			Ridgeline watches the coastal BC MeshCore mesh
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
			<strong>MeshCore</strong> is a protocol for cheap
			<strong>LoRa</strong> radios that sip power and pass messages hop to hop — no towers, no
			internet, no monthly bill. That suits a coastline like this one, where cell
			coverage thins out the moment you leave the highway. The trouble is that a
			mesh is mostly invisible while it runs: the traffic is in the air, not on
			any screen.
		</p>
		<p>
			Ridgeline gives it one. Stations that only ever listen, which we call
			<em>observers</em>, sit and pass what they hear back to be decoded. What comes out
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
		<h2 class="text-fg text-xl font-700">Radio settings (909.000 MHz)</h2>
		<p>
			The mesh lives in the 900&nbsp;MHz ISM band, and every radio on it speaks
			the same LoRa dialect. Below are the exact parameters — the MeshCore
			<strong>default North America</strong> preset, with the frequency set to
			<strong>909.000&thinsp;MHz</strong>. A radio that doesn't match all four
			won't hear a thing, so if you're setting one up, copy them precisely.
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
			It's a narrowband profile. The 62.5&nbsp;kHz channel keeps the receiver
			sensitive enough to pull weak signals off distant ridges, while SF7 keeps
			each packet short on the air — and airtime is the scarce resource on a
			shared frequency. Short packets are what let the mesh keep growing without
			nodes talking over one another. The 4/5 coding rate adds just enough error
			correction to survive a noisy channel.
		</p>
		<p>
			The mesh moved here from 910.425&thinsp;MHz on <strong>1 October 2026</strong>.
			<a href="/faq" class="text-signal font-600 hover:underline"
				>Why we moved to 909.000&thinsp;MHz, and how to retune a node &rarr;</a
			>
		</p>
	</section>

	<section id="bridging" class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Bridging etiquette</h2>
		<p>
			This section is about good manners on a shared channel, not a list of rules. Bridging has its
			place. It just affects everyone on the mesh, so it is worth knowing what it does before you try
			it.
		</p>

		<h3 class="text-fg text-base font-600 pt-2">What a bridge is</h3>
		<p>
			A <strong>bridge</strong> joins two meshes that sit on different frequencies or radio settings.
			Every packet it hears on one side is transmitted again on the other, so to the people using them
			the two meshes become one. The firmware known to do this:
		</p>
		<ul class="list-disc space-y-2 pl-5">
			<li>
				<strong>Official MeshCore repeater builds with a bridge.</strong> Two repeaters, one per
				frequency, joined by a serial cable (RS-232) or by ESP-NOW (Wi-Fi between two ESP32 boards).
				These ship as ready-made builds, so anyone can flash one.
			</li>
			<li>
				<strong>Halo Keymind Cascade</strong>, Mike Carper's MeshCore firmware (used by
				<a href="https://cascadiamesh.org" target="_blank" rel="noopener" class="text-signal hover:underline">CascadiaMesh</a>). One
				radio can time-share two frequencies and pass traffic between them, which it calls
				<em>crossing</em>, so a bridge no longer needs two radios. It also has MQTT, ESP-NOW and
				RS-232 bridges. Crossing is off by default.
			</li>
			<li>
				<strong>Internet and MQTT gateways</strong> that take packets from a network connection and
				transmit them on the air. They are not always called bridges, but they have the same effect.
			</li>
		</ul>

		<h3 class="text-fg text-base font-600 pt-2">What it does to the mesh</h3>
		<p>
			Most MeshCore radios in North America ship on the default preset,
			<strong>910.525&thinsp;MHz</strong>, and that channel is shared by a very large population of
			stations. We left it because of that load: collisions were costing us channel messages, direct
			messages and remote administration of our repeaters (the
			<a href="/faq" class="text-signal hover:underline">frequency FAQ</a> has the full story). That is why a bridge to 910.525 is
			the clearest example of what to avoid:
		</p>
		<ul class="list-disc space-y-2 pl-5">
			<li>
				<strong>The traffic comes with it.</strong> Everything the bridge hears on the other side is
				repeated onto 909.000 and flooded across our repeaters. The airtime we gained by moving would
				be spent carrying another network's messages.
			</li>
			<li>
				<strong>It works both ways.</strong> Our own floods get pushed onto the other side as well,
				which is not a good-neighbour thing to do to a channel that is already busy.
			</li>
			<li>
				<strong>Floods get bigger, not just busier.</strong> A flood spreads until it has crossed the
				whole joined network. Every message travels further, through more repeaters, and takes up
				airtime in more places at once.
			</li>
			<li>
				<strong>Routes get less reliable.</strong> MeshCore names each repeater in a route by a short
				fragment of its key. Join two large meshes and many more repeaters share the same fragment, so
				direct routes become easier to confuse.
			</li>
			<li>
				<strong>Channel settings don't filter it.</strong> A plain bridge forwards every packet it
				hears and cannot read channels, so removing channels from the bridge radio changes nothing.
				Region scoping doesn't help either: it decides what a repeater forwards, not what is on the
				air.
			</li>
		</ul>

		<h3 class="text-fg text-base font-600 pt-2">Where bridging has a place</h3>
		<p>
			Testing and experimenting, and short-term jobs. During the move we ran a bridge on
			Mt&nbsp;Cokley between 910.425 and 909.000 so nobody was cut off while their node was retuned,
			and it came down once the move was complete. It joined two halves of <em>our</em> mesh, for a
			set time.
		</p>

		<h3 class="text-fg text-base font-600 pt-2">Doing it safely</h3>
		<ul class="list-disc space-y-2 pl-5">
			<li>
				<strong>Tell us first.</strong> A quick note through the
				<a href="/contact" class="text-signal hover:underline">contact page</a> means nobody chases a mystery traffic spike.
			</li>
			<li>
				<strong>Keep it short.</strong> An hour or two, then switch bridging off
				(<code>set bridge.enabled off</code>, or turn crossing off) rather than leaving it running.
			</li>
			<li><strong>Pick a quiet time</strong>, not an evening or a net.</li>
			<li>
				<strong>Bridge to something nobody else uses</strong>, such as a test frequency or a second
				radio of your own, not a live mesh.
			</li>
			<li>
				<strong>Filter if your firmware can.</strong> Halo Keymind Cascade can drop or rate-limit
				bridged traffic by packet type and channel, so a test can carry only a dedicated channel such
				as <code>#bridge</code>. A plain bridge can't do this: it passes everything.
			</li>
			<li>
				<strong>Watch the mesh while it runs.</strong> The <a href="/live" class="text-signal hover:underline">live feed</a> and
				Analytics show quickly if traffic on 909.000 jumps, and that's the time to switch it off.
			</li>
		</ul>
		<p>
			If a bridge starts pushing traffic onto 909.000, we'll get in touch and point the owner to this
			page. Most of the time nobody has told them the etiquette yet.
		</p>
	</section>

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Nodes, repeaters, and observers</h2>
		<p>
			A few terms worth pinning down. A <strong>node</strong> is any MeshCore
			device on the network — a handheld you carry, a base station sitting in a
			window, a box bolted to a ridge and running on solar. A <strong>repeater</strong>
			is a node built to listen and rebroadcast, extending the mesh's reach with
			every hop. An <strong>observer</strong> is the odd one out: it only
			receives, and it reports what it hears to Ridgeline, which is why the maps
			have anything to show at all. You can browse them in the
			<a href="/nodes" class="text-signal hover:underline">node directory</a> and
			the <a href="/observers" class="text-signal hover:underline">observer list</a>.
		</p>
	</section>


	<section id="traffic" class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">What the mesh carries</h2>
		<p>
			Here is one full day of traffic as Ridgeline recorded it, from
			<strong>{day.date}</strong> (00:00–24:00&nbsp;UTC). That day the observers picked up
			<strong>{fmt(day.packets)}</strong> distinct packets. Because a packet is heard many times over —
			by several observers, and by each of them again as neighbouring repeaters pass it on — those
			packets add up to <strong>{fmt(day.obs)}</strong> observations, about
			{Math.round(day.obs / day.packets)} per packet.
		</p>

		<div class="border-line/70 not-prose my-2 overflow-hidden rounded-[var(--radius)] border">
			<div class="text-fg-faint border-line/60 flex items-end gap-3 border-b px-4 py-2 font-mono text-[0.65rem] tracking-wide uppercase">
				<span class="min-w-0 flex-1">Packet type</span>
				<span class="w-16 shrink-0 text-right">Packets</span>
				<span class="w-24 shrink-0 text-right sm:w-28">Observations</span>
			</div>
			<div class="divide-line/60 divide-y">
				{#each traffic as t (t.name)}
					<div class="flex items-start gap-3 px-4 py-2.5">
						<div class="min-w-0 flex-1">
							<div class="text-fg text-sm font-600 break-words">{t.name}</div>
							{#if t.flood}<div class="text-fg-faint font-mono text-[0.58rem] tracking-wide uppercase">mostly flood</div>{/if}
							<div class="text-fg-faint text-xs leading-snug">{t.what}</div>
						</div>
						<div class="w-16 shrink-0 text-right font-mono text-sm tabular-nums">
							<div class="text-fg">{fmt(t.packets)}</div>
							<div class="text-fg-faint text-[0.65rem]">{pct(t.packets, day.packets)}</div>
						</div>
						<div class="w-24 shrink-0 text-right font-mono text-sm tabular-nums sm:w-28">
							<div class="text-signal font-600">{fmt(t.obs)}</div>
							<div class="bg-line/50 mt-1 ml-auto h-1 w-full overflow-hidden rounded-full">
								<div class="bg-signal h-full rounded-full" style="width:{(t.obs / traffic[0].obs) * 100}%"></div>
							</div>
							<div class="text-fg-faint text-[0.65rem]">{pct(t.obs, day.obs)}</div>
						</div>
					</div>
				{/each}
			</div>
		</div>

		<h3 class="text-fg text-base font-600 pt-2">Floods are what fill the air</h3>
		<p>
			A <strong>flood</strong> packet is passed on by every repeater that hears it, so a single one
			reaches the whole mesh and is on the air many times over. Flood types were heard 50 to 70 times
			each. Packets sent <strong>direct</strong>, along a path the sender already knows, only touch the
			repeaters on that path: discovery made up about 10% of the day's packets but only 0.5% of what
			was heard.
		</p>

		<h3 class="text-fg text-base font-600 pt-2">Requests were the biggest load</h3>
		<p>
			Requests were about a quarter of the packets but over a third of everything heard, and nearly
			all of them were floods. They arrived at a steady rate around the clock, which points to software
			polling repeaters on a schedule rather than people. The path returns they trigger add roughly
			another tenth.
		</p>
		<p>
			If you run anything that checks on a repeater automatically, poll as rarely as you can, and make
			sure your companion already has a path to that repeater so its requests go direct instead of
			flooding the whole mesh. Every flood uses airtime that everyone on 909.000&thinsp;MHz shares.
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

	<section class="text-fg-dim mt-10 space-y-4">
		<h2 class="text-fg text-xl font-700">Questions or problems?</h2>
		<p>
			If something on Ridgeline isn't working, or you have a question about the mesh,
			<a href="/contact" class="text-signal hover:underline">contact us</a>. It helps to say which
			page you were on and roughly when it happened.
		</p>
	</section>

	<footer class="border-line text-fg-faint mt-12 border-t pt-6 text-sm">
		<p>
			Ridgeline is an independent project, run by the community, for the coastal
			British Columbia MeshCore mesh. <a href="/" class="text-signal hover:underline">Open the live dashboard →</a>
		</p>
	</footer>
</article>
