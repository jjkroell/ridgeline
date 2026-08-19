<script lang="ts">
	// How to add an observer to this mesh, as a modal off the Observers page.
	//
	// The settings here are specific to THIS deployment and must stay in step with
	// the running system, so they are sourced rather than invented:
	//   - radio params match /about (910.425 MHz, 62.5 kHz, SF7, CR5)
	//   - the broker + audience match deploy/mosquitto-jwt.conf and the VM's
	//     config.json mqttAuth.audience — the audience must equal the hostname
	//     exactly or the token is refused as minted for another broker
	//   - the CLI is the observer-firmware branch's slot syntax (mqttN.*), not the
	//     older compile-time build (see observer.gessaman.com/docs)
	import Modal from './Modal.svelte';

	let { onclose }: { onclose: () => void } = $props();

	const BROKER = 'wss://mqtt2.ve7kod.ca:443';
	const AUDIENCE = 'mqtt2.ve7kod.ca';

	let copied = $state<string | null>(null);
	async function copy(label: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = label;
			setTimeout(() => (copied === label ? (copied = null) : null), 1400);
		} catch {
			/* clipboard blocked */
		}
	}

	// Slot 1 is the uplink; slot 2 ships preset to the Let's Mesh EU analyzer, so
	// it is explicitly disabled rather than left to publish elsewhere by default.
	const uplink = `set mqtt1.preset custom
set mqtt1.server ${BROKER}
set mqtt1.audience ${AUDIENCE}
set mqtt2.preset none
set mqtt.iata YVR`;

	const radio = `set radio 910.425,62.5,7,5
set tx 22`;

	const wifi = `set wifi.ssid YOUR_NETWORK
set wifi.pwd YOUR_PASSWORD`;

	const verify = `get mqtt.status
get mqtt1.preset
get wifi.status`;

	const everything = `${radio}
set name YOUR_OBSERVER_NAME
${wifi}
${uplink}
reboot`;
</script>

<Modal {onclose} size="2xl">
	<div class="border-line/70 flex items-center gap-3 border-b px-5 py-4">
		<h2 class="font-display text-fg text-base font-700">Add an observer</h2>
		<button onclick={onclose} class="label hover:text-signal ml-auto transition-colors"
			>Close</button
		>
	</div>

	<div class="space-y-5 overflow-y-auto px-5 py-4">
		<p class="text-fg-dim text-sm leading-relaxed">
			An observer is a receive-only MeshCore node that reports the packets it hears
			to Ridgeline over the internet. It doesn't extend the mesh or carry anyone's
			traffic — it just adds a vantage point, and more vantage points mean a truer
			picture of which links actually work.
		</p>

		<section class="space-y-2">
			<div class="label">1 · Flash the firmware</div>
			<p class="text-fg-dim text-sm leading-relaxed">
				Any WiFi-capable ESP32 MeshCore board works — Heltec V3/V4, LilyGo T3S3,
				T-Beam, Station G2, Xiao S3 WIO. Adam Gessaman's flasher installs the
				observer build straight from the browser (Chrome or Edge), so there is
				nothing to compile.
			</p>
			<div class="flex flex-wrap gap-2">
				<a
					href="https://observer.gessaman.com/"
					target="_blank"
					rel="noopener noreferrer"
					class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
					>Observer flasher ↗</a
				>
				<a
					href="https://observer.gessaman.com/docs"
					target="_blank"
					rel="noopener noreferrer"
					class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
					>Firmware docs ↗</a
				>
			</div>
			<p class="text-fg-faint text-xs leading-relaxed">
				Then connect over serial at 115200 baud and enter the commands below.
			</p>
		</section>

		<section class="space-y-2">
			<div class="label">2 · Match the mesh's radio settings</div>
			<p class="text-fg-dim text-sm leading-relaxed">
				A radio on the wrong settings hears nothing at all — this is the most
				common reason a new observer looks dead.
			</p>
			<div class="border-line bg-ink flex items-start gap-2 rounded-[var(--radius)] border px-3 py-2">
				<pre class="text-fg flex-1 overflow-x-auto font-mono text-xs leading-relaxed">{radio}</pre>
				<button
					onclick={() => copy('radio', radio)}
					class="text-fg-faint hover:text-fg shrink-0 text-xs">{copied === 'radio' ? '✓' : 'copy'}</button
				>
			</div>
		</section>

		<section class="space-y-2">
			<div class="label">3 · WiFi</div>
			<div class="border-line bg-ink flex items-start gap-2 rounded-[var(--radius)] border px-3 py-2">
				<pre class="text-fg flex-1 overflow-x-auto font-mono text-xs leading-relaxed">{wifi}</pre>
				<button
					onclick={() => copy('wifi', wifi)}
					class="text-fg-faint hover:text-fg shrink-0 text-xs">{copied === 'wifi' ? '✓' : 'copy'}</button
				>
			</div>
			<p class="text-fg-faint text-xs leading-relaxed">
				The value is the rest of the line — don't wrap it in quotes.
			</p>
		</section>

		<section class="space-y-2">
			<div class="label">4 · Point it at Ridgeline</div>
			<p class="text-fg-dim text-sm leading-relaxed">
				Ridgeline's broker authenticates observers: your node signs a token with
				its own key, so it can only publish under its own identity and nobody can
				report traffic as you. Setting the audience is what turns that on — there
				is no password to obtain and nothing to register.
			</p>
			<div class="border-line bg-ink flex items-start gap-2 rounded-[var(--radius)] border px-3 py-2">
				<pre class="text-fg flex-1 overflow-x-auto font-mono text-xs leading-relaxed">{uplink}</pre>
				<button
					onclick={() => copy('uplink', uplink)}
					class="text-fg-faint hover:text-fg shrink-0 text-xs"
					>{copied === 'uplink' ? '✓' : 'copy'}</button
				>
			</div>
			<p class="text-fg-faint text-xs leading-relaxed">
				Use <code class="text-fg-dim font-mono">YCD</code> instead of
				<code class="text-fg-dim font-mono">YVR</code> for the Nanaimo side — it becomes
				the region on your observer's topic. If you'd rather keep feeding the Let's
				Mesh analyzer as well, leave slots 1 and 2 alone and put the settings above
				on <code class="text-fg-dim font-mono">mqtt3</code> instead.
			</p>
		</section>

		<section class="space-y-2">
			<div class="label">5 · Reboot and check</div>
			<div class="border-line bg-ink flex items-start gap-2 rounded-[var(--radius)] border px-3 py-2">
				<pre class="text-fg flex-1 overflow-x-auto font-mono text-xs leading-relaxed">{verify}</pre>
				<button
					onclick={() => copy('verify', verify)}
					class="text-fg-faint hover:text-fg shrink-0 text-xs"
					>{copied === 'verify' ? '✓' : 'copy'}</button
				>
			</div>
			<p class="text-fg-faint text-xs leading-relaxed">
				Your station should appear on this page within a few minutes of hearing its
				first packet.
			</p>
		</section>

		<section class="space-y-2 border-t border-line/70 pt-4">
			<div class="label">Everything at once</div>
			<div class="border-line bg-ink flex items-start gap-2 rounded-[var(--radius)] border px-3 py-2">
				<pre
					class="text-fg flex-1 overflow-x-auto font-mono text-xs leading-relaxed">{everything}</pre>
				<button
					onclick={() => copy('all', everything)}
					class="text-fg-faint hover:text-fg shrink-0 text-xs">{copied === 'all' ? '✓' : 'copy'}</button
				>
			</div>
			<p class="text-fg-faint text-xs leading-relaxed">
				Replace the three placeholders first. Paste a line at a time if your serial
				console drops characters.
			</p>
		</section>
	</div>
</Modal>
