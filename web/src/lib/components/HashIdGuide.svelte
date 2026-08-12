<script lang="ts">
	// Explainer for multi-byte hash IDs: why a mesh benefits from moving off
	// 1-byte path IDs, and the exact steps to change a repeater, room server or
	// companion.
	//
	// Rendered as a linkable page (/hash-ids) rather than a modal so individual
	// sections can be shared directly — #why, #repeaters, #companions.
	//
	// The numbers and CLI syntax here come from MeshCore itself:
	//   - path_hash_mode is 0/1/2 for 1/2/3 bytes (CommonCLI.cpp rejects >= 3)
	//   - the *originator* stamps the width into the packet header via
	//     setPathHashSizeAndCount(), and every relay appends its hash at that
	//     width (Mesh::routeRecvPacket -> copyHashTo(..., getPathHashSize()))
	let { compact = false }: { compact?: boolean } = $props();

	let copied = $state<string | null>(null);
	async function copy(label: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = label;
			setTimeout(() => (copied === label ? (copied = null) : null), 1200);
		} catch {
			/* clipboard blocked */
		}
	}

	const sections = [
		{ id: 'why', label: 'Why it matters' },
		{ id: 'repeaters', label: 'Repeaters & rooms' },
		{ id: 'companions', label: 'Companions' }
	];

	// Chance that at least two nodes share an ID, by cohort size (birthday
	// problem over the usable ID space; 00 and FF are reserved at 1 byte).
	const odds = [
		{ n: 10, b1: '16%', b2: '0.07%', b3: '~0%' },
		{ n: 20, b1: '54%', b2: '0.3%', b3: '~0%' },
		{ n: 50, b1: '99.4%', b2: '1.9%', b3: '0.01%' },
		{ n: 100, b1: '100%', b2: '7.3%', b3: '0.03%' },
		{ n: 200, b1: '100%', b2: '26%', b3: '0.12%' }
	];

	const modes = [
		{ mode: 0, bytes: 1, space: '254', note: 'Legacy default' },
		{ mode: 1, bytes: 2, space: '65,534', note: 'Recommended' },
		{ mode: 2, bytes: 3, space: '16,777,214', note: 'Large meshes' }
	];
</script>

<div class="mx-auto max-w-3xl space-y-4 {compact ? 'px-4 py-4' : 'px-6 py-6 md:px-10'}">
	<!-- Jump links. Each section is anchor-addressable so a specific answer can
	     be linked directly, e.g. /hash-ids#companions -->
	<nav class="panel rise flex flex-wrap gap-2 px-5 py-4">
		{#each sections as sct (sct.id)}
			<a
				href="#{sct.id}"
				class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
				>{sct.label}</a
			>
		{/each}
	</nav>

	<div class="panel rise space-y-4 px-5 py-5 text-sm leading-relaxed" style="animation-delay:40ms">
		<section id="why" class="scroll-mt-24 space-y-4">
			<h2 class="font-display text-fg text-base font-700">Why it matters</h2>
			<section class="space-y-2">
				<div class="label">What a hash ID is</div>
				<p class="text-fg-dim">
					When a packet is relayed, each repeater stamps its own identity into the packet's path so
					the route can be retraced and replies can be sent back. It doesn't have room for a full
					32-byte public key, so it writes only the <span class="text-fg">first 1, 2 or 3 bytes</span
					> — the node's <em>hash ID</em>. That's the ID the planner analyses.
				</p>
			</section>

			<section class="space-y-2">
				<div class="label">Why 1 byte stops working</div>
				<p class="text-fg-dim">
					One byte is 256 values, and <span class="font-mono text-xs">00</span>/<span
						class="font-mono text-xs">FF</span
					>
					are reserved — so <span class="text-fg">254 usable IDs</span> for the whole mesh. Because
					IDs come from random keys, duplicates arrive far sooner than the raw count suggests:
				</p>
				<div class="border-line overflow-x-auto rounded-[var(--radius)] border">
					<table class="w-full text-xs">
						<thead class="text-fg-faint border-line/70 border-b">
							<tr>
								<th class="px-3 py-2 text-left font-medium">Routing nodes</th>
								<th class="px-3 py-2 text-right font-medium">1-byte</th>
								<th class="px-3 py-2 text-right font-medium">2-byte</th>
								<th class="px-3 py-2 text-right font-medium">3-byte</th>
							</tr>
						</thead>
						<tbody class="tnum font-mono">
							{#each odds as o (o.n)}
								<tr class="border-line/40 border-b last:border-0">
									<td class="text-fg-dim px-3 py-1.5">{o.n}</td>
									<td class="text-coral px-3 py-1.5 text-right">{o.b1}</td>
									<td class="text-fg px-3 py-1.5 text-right">{o.b2}</td>
									<td class="text-fg px-3 py-1.5 text-right">{o.b3}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
				<p class="text-fg-faint text-xs">
					Odds that at least two nodes collide. At 1 byte it's a coin flip by 20 nodes and a
					certainty by 50 — collisions aren't an edge case, they're the expected state of any mesh
					that grows.
				</p>
			</section>

			<section class="space-y-2">
				<div class="label">What a collision actually breaks</div>
				<ul class="text-fg-dim space-y-1.5">
					<li class="flex gap-2">
						<span class="text-coral shrink-0">·</span>
						<span
							><span class="text-fg">Misrouting.</span> A directly-routed packet names the next hop by
							hash. If two repeaters answer to it, the wrong one can pick the packet up — or both do,
							duplicating the transmission and burning airtime twice.</span
						>
					</li>
					<li class="flex gap-2">
						<span class="text-coral shrink-0">·</span>
						<span
							><span class="text-fg">False loop detection.</span> A repeater that sees its own ID in a
							path it never touched treats the packet as a loop and drops it. Legitimate traffic disappears,
							and it looks like a coverage problem rather than an ID problem.</span
						>
					</li>
					<li class="flex gap-2">
						<span class="text-coral shrink-0">·</span>
						<span
							><span class="text-fg">Unreadable topology.</span> Ridgeline can't tell which of the colliding
							nodes carried a hop, so path maps, hop counts and coverage analysis all get blurry.</span
						>
					</li>
				</ul>
			</section>

			<section class="space-y-2">
				<div class="label">The cost, honestly</div>
				<p class="text-fg-dim">
					Wider IDs aren't free: every hop in every packet costs that many bytes instead of one. A
					5-hop route carries 5 bytes of path at 1 byte, but 15 at 3 bytes — real airtime on a slow
					LoRa link, on every relay.
				</p>
				<p class="text-fg-dim">
					<span class="text-signal">2 bytes is the sweet spot for most meshes.</span> It costs one extra
					byte per hop and takes a 50-node mesh from near-certain collisions to under 2%. Go to 3 bytes
					when the mesh is genuinely large or you want headroom for growth.
				</p>
			</section>

			<section class="border-signal/40 bg-signal/5 space-y-2 rounded-[var(--radius)] border p-3">
				<div class="label text-signal">The part most people miss</div>
				<p class="text-fg-dim">
					The width is chosen by <span class="text-fg">whoever sends the packet</span>, not by the
					repeaters carrying it. The originator stamps it into the packet header, and every relay
					appends its hash at
					<em>that</em> width.
				</p>
				<p class="text-fg-dim">
					So your <span class="text-fg">companions matter too</span> — even though a companion never
					appears in a path itself and can't collide. If the phones and clients on your mesh are
					still sending at 1 byte, their traffic gets 1-byte paths no matter how the repeaters are
					configured. Fixing the repeaters alone only fixes the packets the repeaters originate.
				</p>
			</section>
		</section>

		<hr class="border-line/50" />

		<section id="repeaters" class="scroll-mt-24 space-y-4">
			<h2 class="font-display text-fg text-base font-700">Repeaters &amp; room servers</h2>
			<section class="space-y-2">
				<div class="label">Repeaters & room servers</div>
				<p class="text-fg-dim">
					Set over the serial CLI (USB) or remotely by sending CLI commands as an admin over a
					direct message.
				</p>
			</section>

			<section class="space-y-2">
				<div class="label">1 · Check the current setting</div>
				<div class="border-line bg-ink flex items-center gap-2 rounded-[var(--radius)] border px-3 py-2">
					<code class="text-fg flex-1 font-mono text-xs">get path.hash.mode</code>
					<button
						onclick={() => copy('get', 'get path.hash.mode')}
						class="text-fg-faint hover:text-fg shrink-0 text-xs">{copied === 'get' ? '✓' : 'copy'}</button
					>
				</div>
			</section>

			<section class="space-y-2">
				<div class="label">2 · Set the mode</div>
				<div class="border-line overflow-x-auto rounded-[var(--radius)] border">
					<table class="w-full text-xs">
						<thead class="text-fg-faint border-line/70 border-b">
							<tr>
								<th class="px-3 py-2 text-left font-medium">Command</th>
								<th class="px-3 py-2 text-left font-medium">ID width</th>
								<th class="px-3 py-2 text-right font-medium">Address space</th>
								<th class="px-3 py-2 text-left font-medium"></th>
							</tr>
						</thead>
						<tbody>
							{#each modes as m (m.mode)}
								<tr class="border-line/40 border-b last:border-0">
									<td class="px-3 py-2">
										<code class="text-fg font-mono">set path.hash.mode {m.mode}</code>
									</td>
									<td class="text-fg-dim px-3 py-2">{m.bytes} byte{m.bytes > 1 ? 's' : ''}</td>
									<td class="text-fg-dim tnum px-3 py-2 text-right font-mono">{m.space}</td>
									<td class="px-3 py-2">
										<span class="text-xs {m.note === 'Recommended' ? 'text-signal' : 'text-fg-faint'}"
											>{m.note}</span
										>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
				<p class="text-fg-faint text-xs">
					The argument is the <em>mode</em> (0/1/2), not the byte count — mode {' '}
					<span class="font-mono">1</span> means 2-byte IDs. Anything above 2 is rejected with
					<span class="font-mono">Error, must be 0,1, or 2</span>; mode 3 is reserved.
				</p>
			</section>

			<section class="space-y-2">
				<div class="label">3 · Confirm</div>
				<p class="text-fg-dim">
					The node replies <span class="font-mono text-xs">OK</span> and saves immediately —
					<span class="text-fg">no reboot needed</span>. Re-run
					<span class="font-mono text-xs">get path.hash.mode</span> to verify, then check back here once
					it has re-advertised.
				</p>
			</section>

			<section class="border-amber/40 bg-amber/5 rounded-[var(--radius)] border p-3">
				<p class="text-fg-dim text-xs">
					<span class="text-amber">Pick your ID before you switch.</span> Changing width changes which
					nodes you can collide with — a free 1-byte ID says nothing about whether your 2-byte ID is free.
					Use the planner behind this dialog to check the target length first.
				</p>
			</section>
		</section>

		<hr class="border-line/50" />

		<section id="companions" class="scroll-mt-24 space-y-4">
			<h2 class="font-display text-fg text-base font-700">Companions</h2>
			<section class="space-y-2">
				<p class="text-fg-dim">
					A companion never appears in a routing path, so it can't collide with anything — but its
					setting decides the path width for
					<span class="text-fg">every packet it originates</span>, which is most of the human traffic
					on a mesh. Leaving clients at 1 byte quietly holds the whole mesh at 1-byte paths.
				</p>
			</section>

			<section class="space-y-2">
				<div class="label">Where to change it</div>
				<ul class="text-fg-dim space-y-1.5">
					<li class="flex gap-2">
						<span class="text-signal shrink-0">·</span>
						<span
							><span class="text-fg">MeshCore app</span> (phone or desktop) — in the radio/advanced settings,
							alongside the other transmit options.</span
						>
					</li>
					<li class="flex gap-2">
						<span class="text-signal shrink-0">·</span>
						<span
							><span class="text-fg">RemoteTerm</span> and other power-user clients — exposed as a path
							hash mode setting.</span
						>
					</li>
					<li class="flex gap-2">
						<span class="text-signal shrink-0">·</span>
						<span
							>Any client speaking the companion protocol can set it; it's a standard command rather
							than a vendor extension.</span
						>
					</li>
				</ul>
				<p class="text-fg-faint text-xs">
					The values match the repeater table: 1, 2 or 3 bytes. Applies to newly sent packets right
					away.
				</p>
			</section>

			<section class="space-y-2">
				<div class="label">Firmware support</div>
				<p class="text-fg-dim">
					Needs reasonably current companion firmware — older builds don't report or accept the
					setting, and the option simply won't appear in your app. If it's missing, update the
					radio's firmware first.
				</p>
			</section>

			<section class="border-line bg-ink/50 rounded-[var(--radius)] border p-3">
				<div class="label mb-1">Rolling it out</div>
				<p class="text-fg-dim text-xs">
					Mixed widths coexist safely — nothing breaks while some nodes are still on 1 byte, so
					there's no flag day. Move the busiest senders and the repeaters they route through first,
					and the paths you actually care about get wider immediately.
				</p>
			</section>
		</section>
	</div>
</div>
