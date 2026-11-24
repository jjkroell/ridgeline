<script lang="ts">
	// Recommended settings for a radio on this mesh: the LoRa preset every node
	// must match, the path hash ID width, and the repeater tuning (advert
	// intervals, TX delay by neighbour count). Replaces the long-form About page;
	// its bridging and traffic sections moved to /faq. Every command here was
	// checked against MeshCore 1.17.1 (src/helpers/CommonCLI.cpp), including the
	// allowed ranges: txdelay/direct.txdelay 0–2, advert.interval 60–240 min
	// (0 = off), flood.advert.interval 3–168 h, rxdelay 0–20.
	import Seo from '$lib/components/Seo.svelte';
	import { onMount } from 'svelte';

	const radio = [
		{ k: 'Frequency', v: '909.000 MHz' },
		{ k: 'Bandwidth', v: '62.5 kHz' },
		{ k: 'Spreading factor', v: '7' },
		{ k: 'Coding rate', v: '5' }
	];

	const hash = [
		{ bytes: 2, mode: 1, ids: '65,024' },
		{ bytes: 3, mode: 2, ids: '16.6 million' }
	];

	// Advert intervals: zero-hop in minutes, flood in hours.
	const advert = { zeroHop: 60, flood: 30 };

	// Repeaters that hear more neighbours wait longer before passing a packet on,
	// so the ones closer to the sender get to cover their area first.
	const tiers = [
		{ n: '0–1', tx: '0.3', direct: '0.1' },
		{ n: '2–4', tx: '0.5', direct: '0.3' },
		{ n: '5–9', tx: '1.0', direct: '0.5' },
		{ n: '10–14', tx: '1.5', direct: '1.0' },
		{ n: '15+', tx: '2.0', direct: '2.0' }
	];
	const common = ['set agc.reset.interval 4', 'set multi.acks 1', 'set rxdelay 3'];

	let tier = $state(1);
	const block = $derived(
		[
			`set txdelay ${tiers[tier].tx}`,
			`set direct.txdelay ${tiers[tier].direct}`,
			...common,
			`set advert.interval ${advert.zeroHop}`,
			`set flood.advert.interval ${advert.flood}`
		].join('\n')
	);

	// Shared links from the old About page (/about#bridging, /about#traffic) arrive
	// here via the /about redirect; those sections now live on the FAQ.
	onMount(() => {
		const id = location.hash.slice(1);
		if (id === 'bridging' || id === 'traffic') location.replace('/faq#' + id);
	});

	let copied = $state('');
	async function copy(key: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = key;
			setTimeout(() => copied === key && (copied = ''), 1500);
		} catch {
			/* clipboard blocked */
		}
	}
</script>

<Seo
	title="Recommended MeshCore settings — coastal BC mesh on 909.000 MHz"
	description="The radio settings for the Vancouver Island and Lower Mainland MeshCore mesh (909.000 MHz, 62.5 kHz, SF7, CR5), path hash IDs, and repeater tuning by neighbour count."
	path="/settings"
/>

<article class="st mx-auto max-w-3xl px-6 py-12 leading-relaxed">
	<header class="mb-8">
		<p class="label text-signal mb-2">Settings</p>
		<h1 class="font-display text-fg text-3xl font-900 tracking-tight sm:text-4xl">
			Recommended settings
		</h1>
		<p class="text-fg-dim mt-4 text-lg">
			The settings for a MeshCore radio on the coastal BC mesh, across Vancouver Island and the Lower
			Mainland. Companions only need the radio settings. If you run a repeater, the tuning further
			down keeps it from competing with its neighbours for airtime.
		</p>
	</header>

	<section id="radio">
		<h2>Radio</h2>
		<p>
			All four values must match, on companions and repeaters alike. A radio that is right on
			frequency but wrong on any of the others won't hear the mesh, and the mesh won't hear it.
		</p>
		<dl class="border-line divide-line/60 panel divide-y overflow-hidden rounded-[var(--radius)] border">
			{#each radio as r (r.k)}
				<div class="flex items-center justify-between gap-4 px-4 py-3">
					<dt class="text-fg-dim text-sm">{r.k}</dt>
					<dd class="text-signal font-mono text-sm font-600 tabular-nums">{r.v}</dd>
				</div>
			{/each}
		</dl>
		<p>
			<strong>Companions:</strong> in the MeshCore app, open the radio settings, choose the
			<em>USA/Canada</em> preset, then change the frequency to 909.000. The preset then shows as
			<em>Custom</em>, which is expected.
		</p>
		<p><strong>Repeaters and room servers:</strong> over the serial console or remote admin:</p>
		<pre class="cmd"><span class="p">&gt; </span><span class="c">set radio 909.000,62.5,7,5</span>
<span class="p">&gt; </span><span class="c">reboot</span></pre>
		<p>
			The new radio settings take effect after the reboot. Before changing a repeater you can only
			reach remotely, read
			<a href="/faq#retune">how to retune a node without stranding it</a>.
		</p>
	</section>

	<section id="hash-ids">
		<h2>Path hash IDs: 2 or 3 bytes</h2>
		<p>
			Each hop in a route is written as the first few bytes of that repeater's key. With 1 byte there
			are only 254 possible IDs, and on a mesh this size many repeaters share one, so routes get
			mixed up. Use <strong>2 or 3 bytes</strong>.
		</p>
		<div class="border-line panel overflow-x-auto rounded-[var(--radius)] border">
			<table class="w-full text-sm">
				<thead class="text-fg-faint border-line/70 border-b text-xs">
					<tr>
						<th class="px-4 py-2 text-left font-medium">ID width</th>
						<th class="px-4 py-2 text-left font-medium">Repeater command</th>
						<th class="px-4 py-2 text-right font-medium">Distinct IDs</th>
					</tr>
				</thead>
				<tbody>
					{#each hash as h (h.mode)}
						<tr class="border-line/40 border-b last:border-0">
							<td class="text-fg px-4 py-2.5 font-600">{h.bytes} bytes</td>
							<td class="px-4 py-2.5"><code>set path.hash.mode {h.mode}</code></td>
							<td class="text-fg-dim px-4 py-2.5 text-right font-mono tabular-nums">{h.ids}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p>
			The command takes the <em>mode</em>, not the byte count: mode 1 is 2 bytes and mode 2 is
			3 bytes. On a companion it's in the app under <em>Settings → Experimental → Default path hash
			size</em>.
		</p>
		<div class="callout warn">
			<p class="ct">Needs firmware 1.14 or newer</p>
			<p>
				Repeaters older than 1.14 don't relay wider IDs. Check the repeaters your traffic goes
				through first.
				<a href="/hash-ids">The hash ID page</a> shows which width your node uses now and which IDs are
				already taken.
			</p>
		</div>
	</section>

	<section id="repeaters">
		<h2>Repeaters</h2>

		<h3>Advert intervals</h3>
		<div class="border-line divide-line/60 panel divide-y overflow-hidden rounded-[var(--radius)] border">
			<div class="flex items-center justify-between gap-4 px-4 py-3">
				<div>
					<div class="text-fg text-sm font-600">Zero hop</div>
					<div class="text-fg-faint text-xs">Heard only by radios in direct range</div>
				</div>
				<code>set advert.interval {advert.zeroHop}</code>
			</div>
			<div class="flex items-center justify-between gap-4 px-4 py-3">
				<div>
					<div class="text-fg text-sm font-600">Flood</div>
					<div class="text-fg-faint text-xs">Repeated across the whole mesh</div>
				</div>
				<code>set flood.advert.interval {advert.flood}</code>
			</div>
		</div>
		<p>
			That's a zero-hop advert every {advert.zeroHop} minutes and a flood advert every {advert.flood}
			hours. A flood advert takes up airtime on every repeater in the mesh, so don't send them more
			often than this.
		</p>

		<h3 id="txdelay">TX delay by neighbour count</h3>
		<p>
			A repeater that hears many neighbours should wait longer before passing a packet on, so that
			repeaters closer to the sender cover their own area first. Fewer repeaters then transmit the
			same packet at once.
		</p>
		<p>
			To count your repeater's neighbours, run <code>neighbors</code> on it. It lists the repeaters it
			has heard directly. Repeaters running openHop also send this list to Ridgeline, and it shows as
			<em>RF neighbours</em> on their page in the <a href="/nodes">node directory</a>. Give a new
			repeater a few days to hear the others before you pick a row.
		</p>
		<div class="border-line panel overflow-x-auto rounded-[var(--radius)] border">
			<table class="w-full text-sm">
				<thead class="text-fg-faint border-line/70 border-b text-xs">
					<tr>
						<th class="px-4 py-2 text-left font-medium">Neighbours</th>
						<th class="px-4 py-2 text-right font-medium">txdelay</th>
						<th class="px-4 py-2 text-right font-medium">direct.txdelay</th>
					</tr>
				</thead>
				<tbody>
					{#each tiers as t, i (t.n)}
						<tr
							class="border-line/40 cursor-pointer border-b last:border-0 {i === tier
								? 'bg-signal/10'
								: 'hover:bg-line/30'}"
							onclick={() => (tier = i)}
						>
							<td class="text-fg px-4 py-2.5 font-600 tabular-nums">{t.n}</td>
							<td class="text-fg-dim px-4 py-2.5 text-right font-mono tabular-nums">{t.tx}</td>
							<td class="text-fg-dim px-4 py-2.5 text-right font-mono tabular-nums">{t.direct}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p>All repeaters, whatever their neighbour count, also get:</p>
		<ul>
			<li><code>set agc.reset.interval 4</code>: resets the receiver's gain every 4 seconds.</li>
			<li>
				<code>set multi.acks 1</code>: when the repeater passes a direct message's delivery receipt
				back to the sender, it sends it twice. The second send gives the receipt another chance to get
				through.
			</li>
			<li>
				<code>set rxdelay 3</code>: a repeater that hears a flood strongly passes it on straight away,
				and one that hears it weakly waits a moment first. Often the packet has already been passed on by
				then, so the weak repeater doesn't need to send it at all. Off by default, and marked
				experimental in MeshCore.
			</li>
		</ul>

		<h3 id="commands">Copy and paste</h3>
		<p>
			Pick your neighbour count, then paste the commands into the repeater's command line (remote
			admin, or the serial console).
		</p>
		<div class="flex flex-wrap gap-1.5" role="radiogroup" aria-label="Neighbours">
			{#each tiers as t, i (t.n)}
				<button
					type="button"
					role="radio"
					aria-checked={i === tier}
					onclick={() => (tier = i)}
					class="rounded-[var(--radius)] border px-3 py-1.5 font-mono text-sm tabular-nums transition-colors {i ===
					tier
						? 'border-signal bg-signal/15 text-signal font-600'
						: 'border-line text-fg-dim hover:text-fg'}">{t.n}</button
				>
			{/each}
		</div>
		<div class="relative">
			<pre class="cmd">{#each block.split('\n') as line (line)}<span class="p">&gt; </span><span class="c">{line}</span>
{/each}</pre>
			<button
				type="button"
				onclick={() => copy('block', block)}
				class="border-line bg-panel text-fg-dim hover:text-fg absolute top-2 right-2 rounded-[var(--radius)] border px-2.5 py-1 text-xs"
				>{copied === 'block' ? 'Copied ✓' : 'Copy'}</button
			>
		</div>
		<p>
			These take effect straight away, with no reboot. Read any of them back with <code>get</code>,
			for example <code>get txdelay</code>.
		</p>
	</section>

	<div class="panel border-line mt-12 flex flex-wrap items-center gap-4 rounded-[var(--radius)] border p-5">
		<p class="text-fg-dim min-w-[16rem] flex-1 text-sm">
			Something here doesn't match what you're seeing, or you have a question about the mesh? Get in
			touch.
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
	.st section {
		margin-top: 2.5rem;
		padding-top: 2rem;
		border-top: 1px solid var(--color-line);
		color: var(--color-fg-dim);
		scroll-margin-top: 1.5rem;
	}
	.st section > :global(* + *) {
		margin-top: 1rem;
	}
	.st h2 {
		font-family: var(--font-display);
		color: var(--color-fg);
		font-size: 1.35rem;
		font-weight: 700;
		line-height: 1.25;
		letter-spacing: -0.01em;
	}
	.st h3 {
		color: var(--color-fg);
		font-size: 1.05rem;
		font-weight: 700;
		margin-top: 2rem !important;
		scroll-margin-top: 1.5rem;
	}
	.st strong {
		color: var(--color-fg);
		font-weight: 600;
	}
	.st section a {
		color: var(--color-signal);
	}
	.st section a:hover {
		text-decoration: underline;
	}
	.st ul {
		list-style: none;
		padding: 0;
	}
	.st ul li {
		position: relative;
		padding-left: 1.25rem;
		margin-top: 0.5rem;
	}
	.st ul li::before {
		content: '';
		position: absolute;
		left: 0;
		top: 0.7em;
		width: 0.45rem;
		height: 0.45rem;
		border-radius: 999px;
		background: var(--color-signal);
	}
	.st code {
		font-family: var(--font-mono);
		font-size: 0.88em;
		color: var(--color-fg);
		background: color-mix(in srgb, var(--color-line) 55%, transparent);
		padding: 0.1em 0.4em;
		border-radius: 0.3rem;
		white-space: nowrap;
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
	.callout {
		background: var(--color-panel);
		border: 1px solid var(--color-line);
		border-left: 3px solid var(--color-amber);
		border-radius: var(--radius);
		padding: 1.1rem 1.3rem;
	}
	.callout > :global(* + *) {
		margin-top: 0.75rem;
	}
	.callout .ct {
		color: var(--color-amber);
		font-weight: 700;
	}
</style>
