<script lang="ts">
	// Why the mesh moved to 909.000 MHz, and how to retune a node. Was a
	// standalone static page with its own fonts and palette; now a themed route
	// in the app shell so it follows whichever theme the visitor has chosen.
	import Seo from '$lib/components/Seo.svelte';

	const current = [
		{ k: 'Frequency', v: '909.000 MHz' },
		{ k: 'Bandwidth', v: '62.5 kHz' },
		{ k: 'Spreading', v: 'SF7' },
		{ k: 'Coding rate', v: 'CR5 (4/5)' }
	];
	const retired = [
		{ k: 'Frequency', v: '910.425 MHz' },
		{ k: 'Bandwidth', v: '62.5 kHz' },
		{ k: 'Spreading', v: 'SF7' },
		{ k: 'Coding rate', v: 'CR5 (4/5)' }
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
	const toc = [
		{ id: 'why-leave', t: 'Why leave the default frequency?' },
		{ id: 'emcomm', t: 'Where emergency communications fits' },
		{ id: 'regions', t: 'Why not MeshCore Regions?' },
		{ id: 'test-bed', t: 'Why 910.425 first' },
		{ id: 'choosing', t: 'How we landed on 909.000' },
		{ id: 'better', t: 'Is 909.000 actually better?' },
		{ id: 'retune', t: 'How do I change my node?' },
		{ id: 'bridging', t: 'Bridging etiquette' },
		{ id: 'traffic', t: 'What the mesh carries' }
	];
</script>

<Seo
	title="Why 909.000 MHz — the coastal BC MeshCore frequency move"
	description="Why the coastal BC MeshCore network moved to 909.000 MHz on October 1st 2026, how to retune repeaters and companions, bridging etiquette, and what traffic the mesh carries."
	path="/faq"
/>

<article class="faq mx-auto max-w-3xl px-6 py-12 leading-relaxed">
	<header class="mb-10">
		<p class="label text-signal mb-2">Network change · Completed October 1st 2026</p>
		<h1 class="font-display text-fg text-3xl font-900 tracking-tight sm:text-4xl">
			Why we moved to <span class="text-signal">909.000&thinsp;MHz</span>
		</h1>
		<p class="text-fg-dim mt-4 text-lg">
			On <strong class="text-fg">October 1st 2026</strong> the mesh moved to 909.000 MHz. This page
			explains what changed, how we arrived at the new settings, and, if your node is still on the
			old channel, what to do to it.
		</p>
	</header>

	<!-- Current vs retired -->
	<div class="grid gap-4 sm:grid-cols-2">
		<div class="panel border-signal/40 rounded-[var(--radius)] border p-5">
			<p class="label text-signal">Current · since October 1st 2026</p>
			<p class="text-signal mt-2 font-mono text-3xl font-600 tabular-nums">909.000</p>
			<p class="text-fg-dim mt-1 text-sm">Set this on your node.</p>
			<dl class="divide-line/60 mt-4 divide-y">
				{#each current as r (r.k)}
					<div class="flex justify-between gap-4 py-2">
						<dt class="text-fg-dim text-sm">{r.k}</dt>
						<dd class="text-fg font-mono text-sm font-600 tabular-nums">{r.v}</dd>
					</div>
				{/each}
			</dl>
		</div>
		<div class="panel border-line rounded-[var(--radius)] border p-5">
			<p class="label text-amber">Retired · October 1st 2026</p>
			<p class="text-amber mt-2 font-mono text-3xl font-600 tabular-nums line-through decoration-2 opacity-80">910.425</p>
			<p class="text-fg-dim mt-1 text-sm">Where the mesh lived for roughly four months. No longer in use.</p>
			<dl class="divide-line/60 mt-4 divide-y">
				{#each retired as r (r.k)}
					<div class="flex justify-between gap-4 py-2">
						<dt class="text-fg-faint text-sm">{r.k}</dt>
						<dd class="text-fg-dim font-mono text-sm tabular-nums">{r.v}</dd>
					</div>
				{/each}
			</dl>
		</div>
	</div>

	<!-- On this page -->
	<nav aria-label="On this page" class="border-line mt-10 rounded-[var(--radius)] border p-5">
		<p class="label text-fg-faint mb-3">On this page</p>
		<ol class="grid gap-x-6 gap-y-1.5 text-sm sm:grid-cols-2">
			{#each toc as x, i (x.id)}
				<li>
					<a href="#{x.id}" class="text-fg-dim hover:text-signal flex gap-2"
						><span class="text-fg-faint font-mono tabular-nums">{i + 1}</span>{x.t}</a
					>
				</li>
			{/each}
		</ol>
	</nav>

	<section id="why-leave">
		<h2>Why did we leave the default US/Canada frequency?</h2>
		<p>
			The default channel is shared with the entire North American MeshCore population. At that
			scale the channel carried far more traffic than our own region generates, and the practical
			result was that <strong>basic mesh functions stopped working for us</strong>:
		</p>
		<ul>
			<li>Channel messages frequently failed to arrive.</li>
			<li>Direct messages were unreliable.</li>
			<li>Remote administration of repeaters was effectively impossible.</li>
		</ul>
		<p>
			The common factor was packet loss from collisions. Too many stations were transmitting into the
			same channel for any one exchange to complete reliably. Leaving that channel was the only lever
			available to us, since we cannot control how many stations elsewhere use it.
		</p>
		<p>
			It showed up plainly at the sites that matter most. On the default channel our high elevation
			repeaters looking south often sat with a noise floor around -90 dBm. That is a punishing floor
			for a network built to hear distant stations running modest power from poor locations, and it
			eats straight into the margin those links depend on.
		</p>
	</section>

	<section id="emcomm">
		<h2>Where does emergency communications fit in?</h2>
		<p>
			The SWBC region mesh treats emergency communications as a priority. The MeshCore
			infrastructure, meaning the repeaters and the sites they live on, is built and maintained with
			emcomm use in mind rather than as a hobby network that might happen to be useful.
		</p>
		<p>
			That is what sets the standard the network has to meet, and it is the reason the congestion
			problem was treated as serious rather than merely irritating. The three things that were
			failing are precisely the things emcomm depends on: getting a message onto a channel, reaching
			one specific person directly, and being able to administer a repeater without climbing to it. A
			network that loses traffic on an ordinary weekday cannot be relied on when it matters.
		</p>
		<p>
			It also explains why the choices described on this page lean toward reach and reliability. A
			quieter channel is worth most to a network whose job is to carry short messages, often from
			stations running modest antennas in poor locations. The same thinking is behind the order we
			retune sites in: a repeater stranded on the wrong frequency is a repeater somebody has to drive
			to.
		</p>
		<p>
			This is infrastructure built and run by volunteers. It is intended to complement the official
			emergency services, not to replace them, and it is not a substitute for calling 911.
		</p>
	</section>

	<section id="regions">
		<h2>Why not use MeshCore Regions instead?</h2>
		<p>
			<strong>Regions scope routing, not radio.</strong> A region tag tells a repeater whether to
			rebroadcast a packet. It does nothing about the packet already on the air. Every radio in range
			still receives it, and it still occupies the channel for exactly as long as it otherwise would.
			Collisions happen at the receiver, caused by two transmissions overlapping in time, and the
			radio energy does not care what region the payload claims to belong to.
		</p>
		<p>
			Scoping therefore only reduces congestion where it stops a retransmission that would otherwise
			have happened inside your own coverage. For that you need the region boundary to also be a
			genuine radio boundary, with repeaters either side whose coverage does not cross the line.
		</p>
		<p>
			We cannot build that boundary, and we would not want to. Our best sites are hilltops chosen
			precisely because they reach as far as possible, and their coverage overlaps heavily by design.
			On this mesh a single packet is heard by around fifteen receivers on average. The overlap that
			makes the network resilient is exactly what makes a clean region boundary impossible. Without
			one, traffic scoped to the far side of the line is still in the air here, still using airtime
			and still colliding.
		</p>
		<p>
			There is also a case scoping can never address. A station within radio range that is not part
			of our network is on our channel no matter what region it declares, and the busy areas south of
			the border are well within range of the Lower Mainland. Region settings cannot remove them from
			the air. A different frequency can, which is what 909.000&nbsp;MHz gives us.
		</p>
		<p>
			Leaving scoping alone has a practical benefit as well. A new station joins by setting four radio
			values and nothing else. There is no region to add, no scope to pick, and no repeater
			configuration to coordinate with anyone. Its messages flood the mesh the same way everyone
			else's do, and they are carried from the moment it powers up. For a network that expects people
			to join quickly, sometimes under pressure, that simplicity is worth protecting.
		</p>
	</section>

	<section id="test-bed">
		<h2>Why 910.425 first, and why only temporarily?</h2>
		<p>
			910.425 MHz was chosen as a <em>test bed</em>, not a destination. The goal was narrow: find out
			whether the mesh itself was sound once the external traffic load was removed. If the network
			worked well on a quiet channel, the problem was congestion; if it still did not, the problem was
			our own topology and hardware.
		</p>
		<p>
			That question was answered, and the mesh ran there for about four months while we looked for a
			permanent home. 909.000&nbsp;MHz is that permanent home.
		</p>
	</section>

	<section id="choosing">
		<h2>How did we land on 909.000?</h2>
		<p>
			Over that period we tested a number of frequencies against two constraints that had to hold
			simultaneously:
		</p>
		<ul>
			<li>
				<strong>Inside the ISM band</strong>, so the network stays licence free and legal for all
				participants.
			</li>
			<li>
				<strong>Inside cavity filter specifications</strong>, so sites running filtered, high power or
				colocated equipment can operate without being detuned or desensitised.
			</li>
		</ul>
		<p>
			Alongside those two constraints we compared the background noise level at each candidate across
			the area the mesh covers.
		</p>
		<p>
			909.000 MHz satisfies both requirements and tested well in operation. Nothing else about the
			radio configuration changes: bandwidth stayed at 62.5 kHz, spreading factor at SF7 and coding
			rate at 5. Only the frequency moved.
		</p>
	</section>

	<section id="better">
		<h2>Is 909.000 actually better?</h2>
		<p>
			On the evidence we have, yes. While testing candidate frequencies we checked the noise floor
			across the general area the mesh covers, and 909.000 came back noticeably lower, with less
			interference than the alternatives we measured it against.
		</p>
		<p>
			That matters because the noise floor sets the level a signal has to rise above before a receiver
			can decode it. A quieter background gives weak and distant stations a better chance of being
			heard, which is exactly what a wide area mesh depends on.
		</p>
		<div class="callout warn">
			<p class="ct">Written before the move</p>
			<p>
				This section was written while 909.000 was still a candidate and only a handful of nodes sat on
				it. The mesh has since moved across and the Mt&nbsp;Cokley bridge between the two channels has
				been taken down, so traffic figures for the new channel now describe the whole network rather
				than a test group.
			</p>
		</div>
		<p>
			One limit is worth stating plainly. Our monitoring only started after the mesh had already left
			the default channel, so we have no recorded measurements of how bad things were beforehand. The
			failed messages and unreachable repeaters described above are what operators experienced at the
			time, not figures we can pull from a log.
		</p>
	</section>

	<section id="retune">
		<h2>How do I change my node?</h2>
		<p>
			Make this change on October 1st 2026. All four values must match. A node with the right
			frequency but the wrong bandwidth or spreading factor will not be heard by anyone, and will not
			hear the mesh. The commands below are taken from the MeshCore 1.17.1 firmware source.
		</p>

		<h3>Companions</h3>
		<p>
			Companion nodes are configured from the MeshCore client app on your phone or desktop, over
			Bluetooth or USB. Connect to your node, open its radio settings, and set the same four values:
		</p>
		<dl class="border-line divide-line/60 panel my-4 divide-y overflow-hidden rounded-[var(--radius)] border">
			{#each [['Frequency', '909.000 MHz'], ['Bandwidth', '62.5 kHz'], ['Spreading factor', '7'], ['Coding rate', '5']] as [k, v] (k)}
				<div class="flex items-center justify-between gap-4 px-4 py-3">
					<dt class="text-fg-dim text-sm">{k}</dt>
					<dd class="text-signal font-mono text-sm font-600 tabular-nums">{v}</dd>
				</div>
			{/each}
		</dl>
		<p>
			Save the settings and let the node restart. Your contacts, channels and keys are not affected by
			a radio change. Only the channel the node listens and transmits on changes.
		</p>
		<p>
			Companion firmware accepts the same <code>set radio</code> command over the serial console if you
			would rather not use the app, and the syntax is identical to the repeater example below.
		</p>

		<h3>Repeaters and room servers</h3>
		<p>
			Repeaters are configured over the serial console, or remotely through remote administration.
			Connect at <span class="font-mono">115200</span> baud, then:
		</p>
		<pre class="cmd"><span class="p">&gt; </span><span class="c">get radio</span>
<span class="r">  -&gt; 910.425,62.5,7,5</span>

<span class="p">&gt; </span><span class="c">set radio 909.000,62.5,7,5</span>
<span class="r">  -&gt; OK - reboot to apply</span>

<span class="p">&gt; </span><span class="c">reboot</span></pre>
		<p>
			The order is <code>freq,bw,sf,cr</code>, separated by commas with no spaces. The new settings are
			saved immediately but <strong>do not take effect until the node reboots</strong>. Run
			<code>get radio</code> again after it comes back to confirm it reads
			<span class="font-mono">909.000,62.5,7,5</span>.
		</p>

		<div class="callout warn">
			<p class="ct">Work from your farthest station inwards</p>
			<p>
				A repeater you change over remote administration reboots onto 909.000, and from 910.425 you can
				no longer reach it. The order you work in therefore matters more than anything else on this
				page.
			</p>
			<p>
				Remote administration reaches a distant repeater by relaying through the ones in between. Move a
				nearby repeater to 909.000 first and it leaves 910.425 taking that relay path with it, stranding
				every station further out.
			</p>
			<p>
				So start with the station that is hardest to reach, then the next one in, and leave
				<em>your own node until last</em>. Worked in that order the path to everything still on 910.425
				stays intact at every step. A repeater you can no longer reach is one you have to go and visit.
			</p>
			<p>
				The Mt Cokley bridge between the two channels used to give you some margin here. It no longer
				does; the two channels are not joined any more, so there is no fallback path to a station left
				behind. For any site you cannot easily get to in person, use the temporary mode below.
			</p>
		</div>

		<h3>Testing a remote site without stranding it</h3>
		<p>
			The firmware provides a temporary mode that reverts on its own, which is the safe way to move a
			site you cannot physically reach:
		</p>
		<pre class="cmd"><span class="p">&gt; </span><span class="c">tempradio 909.000,62.5,7,5,30</span>
<span class="r">  -&gt; OK - temp params for 30 mins</span></pre>
		<p>
			The fifth value is a timeout in minutes. The node switches to 909.000 for that period and then
			returns to its saved settings by itself. If it turns out to be unreachable on the new channel,
			you simply wait for it to come back rather than driving to the site. Once you have confirmed it
			works, make it permanent with <code>set radio</code> followed by <code>reboot</code>.
		</p>

		<h3>If your node goes quiet afterwards</h3>
		<p>
			Check the four values with <code>get radio</code> before assuming a hardware fault. The most
			common outcomes of a partial change are a node that transmits but is never heard, or one that
			hears nothing at all. Both are what a mismatched bandwidth or spreading factor looks like from
			the outside. The firmware accepts any spreading factor from 5 to 12 and any coding rate from 5
			to 8, so an incorrect value will be stored without complaint.
		</p>

		<h3>Running an observer? Restart the bridge after you retune</h3>
		<p>
			If your node feeds Ridgeline through <span class="font-mono">meshcoretomqtt</span>, restart that
			bridge after you change the radio. The bridge reads <code>get radio</code> and your node name from
			the device <em>once, when it starts</em>, and reports those cached values from then on. Change the
			frequency without restarting it and Ridgeline keeps hearing the old preset (the node is actually
			on 909.000 while its status still says 910.425), which makes the observer look mis-set and gets
			its traffic set aside. Restart it with <code>sudo systemctl restart mctomqtt</code> (or reboot the
			host it runs on); the observer re-reads the live settings and corrects itself within a status
			cycle.
		</p>
	</section>

	<section id="bridging">
		<h2>Bridging etiquette</h2>
		<p>
			This section is about good manners on a shared channel, not a list of rules. Bridging has its
			place. It just affects everyone on the mesh, so it is worth knowing what it does before you try
			it.
		</p>

		<h3>What a bridge is</h3>
		<p>
			A <strong>bridge</strong> joins two meshes that sit on different frequencies or radio settings.
			Every packet it hears on one side is transmitted again on the other, so to the people using them
			the two meshes become one. The firmware known to do this:
		</p>
		<ul>
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

		<h3>What it does to the mesh</h3>
		<p>
			Most MeshCore radios in North America ship on the default preset,
			<strong>910.525&thinsp;MHz</strong>, and that channel is shared by a very large population of
			stations. We left it because of that load: collisions were costing us channel messages, direct
			messages and remote administration of our repeaters (the
			<a href="#why-leave" class="text-signal hover:underline">first question on this page</a> has the full story). That is why a bridge to 910.525 is
			the clearest example of what to avoid:
		</p>
		<ul>
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

		<h3>Where bridging has a place</h3>
		<p>
			Testing and experimenting, and short-term jobs. During the move we ran a bridge on
			Mt&nbsp;Cokley between 910.425 and 909.000 so nobody was cut off while their node was retuned,
			and it came down once the move was complete. It joined two halves of <em>our</em> mesh, for a
			set time.
		</p>

		<h3>Doing it safely</h3>
		<ul>
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

	<section id="traffic">
		<h2>What the mesh carries</h2>
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

		<h3>Floods are what fill the air</h3>
		<p>
			A <strong>flood</strong> packet is passed on by every repeater that hears it, so a single one
			reaches the whole mesh and is on the air many times over. Flood types were heard 50 to 70 times
			each. Packets sent <strong>direct</strong>, along a path the sender already knows, only touch the
			repeaters on that path: discovery made up about 10% of the day's packets but only 0.5% of what
			was heard.
		</p>

		<h3>Requests were the biggest load</h3>
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

	<div class="panel border-line mt-12 flex flex-wrap items-center gap-4 rounded-[var(--radius)] border p-5">
		<p class="text-fg-dim min-w-[16rem] flex-1 text-sm">
			Questions about the move, or something on this page that does not match what you are seeing? Get
			in touch and we will get back to you.
		</p>
		<a
			href="/contact"
			class="bg-signal text-ink rounded-[var(--radius)] px-4 py-2 text-sm font-700 hover:brightness-110"
			>Contact us</a
		>
	</div>
</article>

<style>
	.panel {
		background: var(--color-panel);
	}
	.faq section {
		margin-top: 2.75rem;
		padding-top: 2rem;
		border-top: 1px solid var(--color-line);
		color: var(--color-fg-dim);
		scroll-margin-top: 1.5rem;
	}
	.faq section > :global(* + *) {
		margin-top: 1rem;
	}
	.faq h2 {
		font-family: var(--font-display);
		color: var(--color-fg);
		font-size: 1.35rem;
		font-weight: 700;
		line-height: 1.25;
		letter-spacing: -0.01em;
	}
	.faq h3 {
		color: var(--color-fg);
		font-size: 1.05rem;
		font-weight: 700;
		margin-top: 2rem !important;
	}
	.faq strong {
		color: var(--color-fg);
		font-weight: 600;
	}
	.faq ul {
		list-style: none;
		padding: 0;
	}
	.faq ul li {
		position: relative;
		padding-left: 1.25rem;
		margin-top: 0.5rem;
	}
	.faq ul li::before {
		content: '';
		position: absolute;
		left: 0;
		top: 0.7em;
		width: 0.45rem;
		height: 0.45rem;
		border-radius: 999px;
		background: var(--color-signal);
	}
	.faq code {
		font-family: var(--font-mono);
		font-size: 0.88em;
		color: var(--color-fg);
		background: color-mix(in srgb, var(--color-line) 55%, transparent);
		padding: 0.1em 0.4em;
		border-radius: 0.3rem;
	}
	.cmd {
		font-family: var(--font-mono);
		font-size: 0.85rem;
		line-height: 1.8;
		background: var(--color-panel);
		border: 1px solid var(--color-line);
		border-radius: var(--radius);
		padding: 1rem 1.15rem;
		overflow-x: auto;
		white-space: pre;
	}
	.cmd .p {
		color: var(--color-fg-faint);
		user-select: none;
	}
	.cmd .c {
		color: var(--color-signal);
		font-weight: 600;
	}
	.cmd .r {
		color: var(--color-fg-faint);
	}
	.callout {
		background: var(--color-panel);
		border: 1px solid var(--color-line);
		border-left: 3px solid var(--color-signal);
		border-radius: var(--radius);
		padding: 1.1rem 1.3rem;
	}
	.callout.warn {
		border-left-color: var(--color-amber);
	}
	.callout > :global(* + *) {
		margin-top: 0.75rem;
	}
	.callout .ct {
		color: var(--color-fg);
		font-weight: 700;
	}
	.callout.warn .ct {
		color: var(--color-amber);
	}
</style>
