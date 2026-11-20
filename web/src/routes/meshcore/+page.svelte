<script lang="ts">
	// "What is MeshCore?" — an introduction for newcomers and groups, laid out as
	// a presentation: one full-height slide per idea. Scrolls like a page;
	// ←/→, PageUp/PageDown and Space step between slides; Present goes
	// full screen with one slide per screen. Fixed night palette in every theme
	// (it is a presentation, and the videos are dark). Each video plays
	// while its slide is on screen and pauses when you move on.
	import { onMount } from 'svelte';
	import Seo from '$lib/components/Seo.svelte';

	let root: HTMLDivElement;
	let current = $state(0);
	let total = $state(0);
	let presenting = $state(false);
	let canPresent = $state(false);

	const slides = () => Array.from(root.querySelectorAll<HTMLElement>('[data-slide]'));

	function go(i: number) {
		const s = slides();
		const n = Math.max(0, Math.min(s.length - 1, i));
		s[n]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	function onKey(e: KeyboardEvent) {
		const t = e.target as HTMLElement | null;
		if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT|VIDEO)$/.test(t.tagName))) return;
		if (e.metaKey || e.ctrlKey || e.altKey) return;
		if (['ArrowRight', 'ArrowDown', 'PageDown'].includes(e.key) || (e.key === ' ' && !e.shiftKey)) {
			e.preventDefault();
			go(current + 1);
		} else if (['ArrowLeft', 'ArrowUp', 'PageUp'].includes(e.key) || (e.key === ' ' && e.shiftKey)) {
			e.preventDefault();
			go(current - 1);
		} else if (e.key === 'Home') {
			e.preventDefault();
			go(0);
		} else if (e.key === 'End') {
			e.preventDefault();
			go(total - 1);
		}
	}

	async function present() {
		if (document.fullscreenElement) await document.exitFullscreen();
		else await root.requestFullscreen?.().catch(() => {});
	}

	onMount(() => {
		const s = slides();
		total = s.length;
		canPresent = !!document.fullscreenEnabled;
		const io = new IntersectionObserver(
			(entries) => {
				for (const en of entries) {
					if (!en.isIntersecting) continue;
					const el = en.target as HTMLElement;
					el.classList.add('in');
					current = s.indexOf(el);
				}
			},
			{ threshold: 0.55 }
		);
		s.forEach((el) => io.observe(el));

		// Play the clip only while its slide is on screen.
		const vio = new IntersectionObserver(
			(entries) => {
				for (const en of entries) {
					const v = en.target as HTMLVideoElement;
					if (en.isIntersecting) v.play().catch(() => {});
					else v.pause();
				}
			},
			{ threshold: 0.4 }
		);
		root.querySelectorAll('video').forEach((v) => vio.observe(v));

		const onFs = () => {
			presenting = document.fullscreenElement === root;
			if (presenting) s[current]?.scrollIntoView({ block: 'start' });
		};
		document.addEventListener('fullscreenchange', onFs);
		window.addEventListener('keydown', onKey);
		return () => {
			io.disconnect();
			vio.disconnect();
			document.removeEventListener('fullscreenchange', onFs);
			window.removeEventListener('keydown', onKey);
		};
	});

	const roles = [
		{ t: 'Companion', b: 'Your personal radio. Pairs with the app on your phone or computer so you can send and read messages.', i: 'M21 11.5a7.5 7.5 0 0 1-10.8 6.7L3.5 20l1.3-5A7.5 7.5 0 1 1 21 11.5z' },
		{ t: 'Repeater', b: 'The backbone. Sits up high, often on solar, and relays everyone’s messages onward.', i: 'M4.9 4.9a10 10 0 0 0 0 14.2M19.1 4.9a10 10 0 0 1 0 14.2M8 8a5 5 0 0 0 0 8M16 8a5 5 0 0 1 0 8M12 11.2a1 1 0 1 0 0 1.6 1 1 0 0 0 0-1.6z' },
		{ t: 'Room server', b: 'A shared bulletin board. Holds posts so people who log in later can catch up.', i: 'M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3zm0 0v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3' },
		{ t: 'Sensor', b: 'Reports readings such as weather or battery levels over the mesh.', i: 'M2 12h3l2-7 3.5 15L16 8l2 4h4' }
	];
	const steps = [
		{ t: 'Search', b: 'Your first message floods outward. Each repeater passes it on once until it finds your friend.' },
		{ t: 'Learn', b: 'The reply carries back the list of repeaters it travelled through: the route.' },
		{ t: 'Go direct', b: 'Later messages follow only that route. The rest of the mesh stays quiet.' }
	];
	const radios = [
		{ tag: 'Starter', t: 'Heltec V3 / V4', b: 'A small board with a screen. Popular first radio.' },
		{ tag: 'Pocket', t: 'SenseCAP T1000-E', b: 'Card-sized and rugged, with GPS. Clips to a bag.' },
		{ tag: 'Standalone', t: 'LilyGO T-Deck', b: 'Keyboard and screen built in. No phone needed.' },
		{ tag: 'Repeater', t: 'RAK WisBlock', b: 'Very low power. A favourite for solar sites.' }
	];
	const start = [
		{ t: 'Get a radio', b: 'Any supported board, 915 MHz version, with its antenna.' },
		{ t: 'Flash it', b: 'Install MeshCore companion firmware with the web flasher in Chrome or Edge.' },
		{ t: 'Pair the app', b: 'Install the MeshCore app on your phone and connect over Bluetooth.' },
		{ t: 'Say hello', b: 'Set the local radio settings below, then post on the Public channel.' }
	];
	const settings = [
		{ tag: 'Frequency', v: '909.000', u: 'MHz' },
		{ tag: 'Bandwidth', v: '62.5', u: 'kHz' },
		{ tag: 'Spreading', v: 'SF7', u: 'factor' },
		{ tag: 'Coding rate', v: 'CR5', u: '4/5' }
	];
</script>

<Seo
	title="What is MeshCore?"
	description="An introduction to MeshCore: text messaging over long-range radio with no internet or cell towers, how it works, and how to join the coastal BC mesh."
	path="/meshcore"
/>

<div bind:this={root} class="deck" class:presenting>
	<!-- Controls -->
	<div class="controls">
		<span class="count tnum">{current + 1} / {total || 16}</span>
		<button type="button" onclick={() => go(current - 1)} aria-label="Previous slide" disabled={current === 0}>
			<svg viewBox="0 0 24 24"><path d="M15 6l-6 6 6 6" /></svg>
		</button>
		<button type="button" onclick={() => go(current + 1)} aria-label="Next slide" disabled={current >= total - 1}>
			<svg viewBox="0 0 24 24"><path d="M9 6l6 6-6 6" /></svg>
		</button>
		{#if canPresent}
			<button type="button" class="present" onclick={present}>{presenting ? 'Exit' : 'Present'}</button>
		{/if}
	</div>

	<!-- 1 Cover -->
	<section data-slide class="slide">
		<div class="wrap cover">
			<div class="head">
				<p class="eyebrow r" style="--i:0">An introduction</p>
				<h1 class="r" style="--i:1">What is MeshCore?</h1>
				<p class="lead r" style="--i:2">Text messaging over radio. No internet, no cell towers, no monthly bill.</p>
				<p class="hint r" style="--i:3">Scroll, or use the arrow keys. <b>Present</b> goes full screen.</p>
			</div>
			<svg class="art r" style="--i:3" viewBox="0 0 760 640" aria-label="A mesh of radio repeaters on hilltops, with one route between two phones highlighted" role="img">
				<g stroke="#24414A" stroke-width="3" stroke-dasharray="6 10" fill="none">
					<path d="M60 420 L190 300 L330 180 L470 300 L600 200 L700 360" />
					<path d="M190 300 L200 500 L340 420 L470 300 L480 520 L600 440 L700 360" />
					<path d="M330 180 L340 420 M600 200 L600 440 M200 500 L60 420 M480 520 L340 420" />
				</g>
				<path class="route" d="M60 420 L190 300 L330 180 L470 300 L600 200 L700 360" fill="none" stroke="#E8B454" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" />
				<g fill="#34E3C4">
					<path d="M190 272 L214 314 L166 314 Z" /><path d="M330 152 L354 194 L306 194 Z" /><path d="M470 272 L494 314 L446 314 Z" /><path d="M600 172 L624 214 L576 214 Z" />
				</g>
				<g fill="#12222A" stroke="#34E3C4" stroke-width="4">
					<path d="M200 472 L224 514 L176 514 Z" /><path d="M340 392 L364 434 L316 434 Z" /><path d="M480 492 L504 534 L456 534 Z" /><path d="M600 412 L624 454 L576 454 Z" />
				</g>
				<rect x="38" y="384" width="44" height="72" rx="9" fill="#E8B454" />
				<rect x="678" y="324" width="44" height="72" rx="9" fill="#E8B454" />
				<path d="M0 640 C120 590 220 600 330 570 C450 540 560 600 760 560 L760 640 Z" fill="#12222A" />
			</svg>
		</div>
	</section>

	<!-- Full intro video -->
	<section data-slide class="slide video full">
		<div class="vwrap">
			<p class="eyebrow r" style="--i:0">Watch · about a minute and a quarter</p>
			<video
				src="/intro/meshcore-intro.webm"
				poster="/intro/meshcore-intro.png"
				muted
				controls
				playsinline
				preload="metadata"
				aria-label="Animated overview of MeshCore: why it matters, what it is, how far LoRa reaches, the four radio roles, how messages find their way, privacy, and the settings to join the coastal BC mesh"
			></video>
		</div>
	</section>

	<!-- 2 Why -->
	<section data-slide class="slide accent">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">Why it matters</p>
			<h1 class="r" style="--i:1">When the towers go quiet, the mesh keeps talking.</h1>
			<p class="lead r" style="--i:2">Phones and the internet depend on towers, power and cables someone else runs. A mesh is a network of small radios owned by the people who use it, passing messages hand to hand.</p>
		</div>
	</section>

	<!-- 3 What -->
	<section data-slide class="slide">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">What it is</p>
			<h2 class="r" style="--i:1">MeshCore is free software that turns small radios into a messaging network.</h2>
			<div class="grid three">
				<div class="card r" style="--i:2"><svg class="ico" viewBox="0 0 24 24"><path d="M8 7l-5 5 5 5M16 7l5 5-5 5M14 4l-4 16" /></svg><h3>Open source</h3><p>Free firmware, MIT licensed, built in the open by a worldwide community.</p></div>
				<div class="card r" style="--i:3"><svg class="ico" viewBox="0 0 24 24"><path d="M13 2 4 14h7l-1 8 9-12h-7z" /></svg><h3>Tiny radios</h3><p>Runs on pocket-sized LoRa boards that sip power. Solar is enough for a hilltop repeater.</p></div>
				<div class="card r" style="--i:4"><svg class="ico" viewBox="0 0 24 24"><path d="M21 2l-2 2m-7.6 7.6a5.5 5.5 0 1 1-7.8 7.8 5.5 5.5 0 0 1 7.8-7.8zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3" /></svg><h3>Licence free</h3><p>Uses the unlicensed ISM band, so anyone can join. No ham licence, no fees.</p></div>
			</div>
		</div>
	</section>

	<!-- 4 LoRa -->
	<section data-slide class="slide alt">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">The radio underneath</p>
			<h2 class="r" style="--i:1">LoRa trades speed for distance</h2>
			<p class="lead r" style="--i:2">LoRa (“long range”) radio is slow, but a whisper of power reaches a long way. That is the right trade for short text messages.</p>
			<div class="table r" style="--i:3">
				<table>
					<thead><tr><th></th><th>Wi-Fi</th><th>Cellular</th><th class="hl">LoRa</th></tr></thead>
					<tbody>
						<tr><td>Reach</td><td>Across a house</td><td>Kilometres, from a tower</td><td class="hl">Kilometres, radio to radio; far more hilltop to hilltop</td></tr>
						<tr><td>Speed</td><td>Video and more</td><td>Video and more</td><td class="hl">Short texts only</td></tr>
						<tr><td>Power</td><td>Mains or big battery</td><td>Recharge daily</td><td class="hl">Days on a small battery</td></tr>
						<tr><td>Needs</td><td>A router and internet</td><td>Towers and a carrier</td><td class="hl">Nothing but other radios</td></tr>
					</tbody>
				</table>
			</div>
		</div>
	</section>

	<!-- LoRa range clip -->
	<section data-slide class="slide video">
		<video
			src="/intro/lora-range.webm"
			poster="/intro/lora-range.png"
			muted
			loop
			playsinline
			preload="metadata"
			aria-label="Animation: Wi-Fi reaches across a house; a pocket LoRa radio reaches kilometres; repeaters on high ground link far-off hilltops"
		></video>
	</section>

	<!-- 5 Video -->
	<section data-slide class="slide video">
		<video
			src="/intro/mesh-routing.webm"
			poster="/intro/mesh-routing.png"
			muted
			loop
			playsinline
			preload="metadata"
			aria-label="Animation: a message from your phone floods across hilltop repeaters, the reply brings back the route, and later messages go straight along that route to your friend"
		></video>
	</section>

	<!-- 6 Roles -->
	<section data-slide class="slide">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">The building blocks</p>
			<h2 class="r" style="--i:1">Every radio plays one of four roles</h2>
			<div class="grid four">
				{#each roles as x, i (x.t)}
					<div class="card r" style="--i:{i + 2}"><svg class="ico" viewBox="0 0 24 24"><path d={x.i} /></svg><h3>{x.t}</h3><p>{x.b}</p></div>
				{/each}
			</div>
		</div>
	</section>

	<!-- 7 Routing -->
	<section data-slide class="slide alt">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">How messages find their way</p>
			<h2 class="r" style="--i:1">Flood once, then go direct</h2>
			<div class="grid three">
				{#each steps as x, i (x.t)}
					<div class="card r" style="--i:{i + 2}"><p class="num">{i + 1}</p><h3>{x.t}</h3><p>{x.b}</p></div>
				{/each}
			</div>
			<p class="lead r" style="--i:5">Radio time is shared by everyone in range. Sending each message only where it needs to go leaves room for the whole network to grow.</p>
		</div>
	</section>

	<!-- 8 Privacy -->
	<section data-slide class="slide">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">Privacy</p>
			<h2 class="r" style="--i:1">Who can read what</h2>
			<p class="lead r" style="--i:2">Everything on the air is encrypted. Who can read it depends on who holds the key.</p>
			<div class="table r" style="--i:3">
				<table>
					<thead><tr><th>Message</th><th>Who can read it</th><th>Good for</th></tr></thead>
					<tbody>
						<tr><td>Direct message</td><td>Only you and the recipient</td><td>Private conversations</td></tr>
						<tr><td>Private channel</td><td>Anyone you share its key with</td><td>A family, club or team</td></tr>
						<tr><td>#hashtag channel</td><td>Anyone who knows the name</td><td>Open topics like #weather</td></tr>
						<tr><td>Public channel</td><td>Everyone on the mesh</td><td>Saying hi, asking for help</td></tr>
					</tbody>
				</table>
			</div>
			<p class="small r" style="--i:4">Your radio also announces its name and type to the mesh. Its location is shared only if you set one. A mesh is not a replacement for 911.</p>
		</div>
	</section>

	<!-- Privacy clip -->
	<section data-slide class="slide video">
		<video
			src="/intro/who-can-read.webm"
			poster="/intro/who-can-read.png"
			muted
			loop
			playsinline
			preload="metadata"
			aria-label="Animation: who can read a message. A direct message only your friend; a private channel anyone with the key; a hashtag channel anyone who knows its name; the Public channel everyone"
		></video>
	</section>

	<!-- 9 Compare -->
	<section data-slide class="slide alt">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">A common question</p>
			<h2 class="r" style="--i:1">How is it different from Meshtastic?</h2>
			<p class="lead r" style="--i:2">Both run on the same kind of LoRa radio, but they speak different languages and can’t talk to each other.</p>
			<div class="table r" style="--i:3">
				<table>
					<thead><tr><th></th><th class="hl">MeshCore</th><th>Meshtastic</th></tr></thead>
					<tbody>
						<tr><td>Who relays</td><td class="hl">Only repeaters</td><td>Every radio, by default</td></tr>
						<tr><td>Routing</td><td class="hl">Learns a direct route</td><td>Floods every message</td></tr>
						<tr><td>Hops</td><td class="hl">Up to 64</td><td>3 by default, 7 at most</td></tr>
						<tr><td>Best at</td><td class="hl">Wide areas built on fixed repeaters</td><td>Small groups where everyone relays</td></tr>
					</tbody>
				</table>
			</div>
		</div>
	</section>

	<!-- 10 Hardware -->
	<section data-slide class="slide">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">What you need</p>
			<h2 class="r" style="--i:1">One small radio and a phone</h2>
			<div class="grid four">
				{#each radios as x, i (x.t)}
					<div class="card r" style="--i:{i + 2}"><p class="tag">{x.tag}</p><h3>{x.t}</h3><p>{x.b}</p></div>
				{/each}
			</div>
			<p class="lead r" style="--i:6">Buy the 915 MHz version for North America, and never power a radio on without its antenna attached.</p>
		</div>
	</section>

	<!-- 11 Start -->
	<section data-slide class="slide alt">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">Getting on the air</p>
			<h2 class="r" style="--i:1">Four steps, about half an hour</h2>
			<div class="grid four">
				{#each start as x, i (x.t)}
					<div class="card r" style="--i:{i + 2}"><p class="num">{i + 1}</p><h3>{x.t}</h3><p>{x.b}</p></div>
				{/each}
			</div>
		</div>
	</section>

	<!-- 12 Local -->
	<section data-slide class="slide">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">Our mesh · coastal British Columbia</p>
			<h2 class="r" style="--i:1">Join with four settings</h2>
			<p class="lead r" style="--i:2">All four must match exactly, or your radio won’t hear the mesh and the mesh won’t hear you.</p>
			<div class="grid four">
				{#each settings as x, i (x.tag)}
					<div class="card r" style="--i:{i + 3}"><p class="tag">{x.tag}</p><p class="big tnum">{x.v}</p><p>{x.u}</p></div>
				{/each}
			</div>
			<p class="lead r" style="--i:7">Then watch yourself appear on the <a href="/live-map">live map</a> and in the <a href="/nodes">node directory</a>. The <a href="/faq">frequency FAQ</a> has the details.</p>
		</div>
	</section>

	<!-- 13 Close -->
	<section data-slide class="slide alt">
		<div class="wrap">
			<p class="eyebrow r" style="--i:0">Thank you</p>
			<h1 class="r" style="--i:1">Questions?</h1>
			<div class="grid three">
				<a class="card link r" style="--i:2" href="https://github.com/meshcore-dev/MeshCore" target="_blank" rel="noopener"><p class="tag">The project</p><p>github.com/<wbr />meshcore-dev/MeshCore</p></a>
				<a class="card link r" style="--i:3" href="/"><p class="tag">See the mesh live</p><p>Ridgeline overview</p></a>
				<a class="card link r" style="--i:4" href="/contact"><p class="tag">Get in touch</p><p>Contact us</p></a>
			</div>
		</div>
	</section>
</div>

<style>
	.deck {
		--ink: #0b1418;
		--ink-2: #12222a;
		--line: #24414a;
		--fg: #eef3f1;
		--fg-2: #b7ccc8;
		--teal: #34e3c4;
		--amber: #e8b454;
		position: relative;
		background: var(--ink);
		color: var(--fg);
	}
	.slide {
		min-height: 100dvh;
		display: flex;
		align-items: center;
		background: var(--ink);
		padding: 72px 24px;
		box-sizing: border-box;
		scroll-margin-top: 0;
	}
	.slide.alt { background: var(--ink-2); }
	.slide.accent { background: var(--teal); color: var(--ink); }
	.slide.accent .eyebrow, .slide.accent h1, .slide.accent .lead { color: var(--ink); }
	.wrap { width: 100%; max-width: 1240px; margin: 0 auto; display: flex; flex-direction: column; gap: 28px; }
	.cover { flex-direction: row; align-items: center; gap: 48px; }
	.cover .head { flex: 1 1 0; display: flex; flex-direction: column; gap: 20px; min-width: 0; }
	.art { flex: 0 1 520px; width: 100%; max-width: 520px; height: auto; }

	h1 { font-family: var(--font-display); font-size: clamp(2.6rem, 6vw, 5.2rem); font-weight: 900; line-height: 1.02; letter-spacing: -0.02em; margin: 0; }
	h2 { font-family: var(--font-display); font-size: clamp(2rem, 4vw, 3.4rem); font-weight: 800; line-height: 1.08; letter-spacing: -0.01em; margin: 0; max-width: 1000px; }
	h3 { font-family: var(--font-display); font-size: 1.45rem; font-weight: 700; margin: 0; color: var(--fg); }
	p { margin: 0; }
	.eyebrow { font-size: 0.85rem; font-weight: 700; letter-spacing: 0.18em; text-transform: uppercase; color: var(--teal); }
	.lead { font-size: clamp(1.1rem, 1.8vw, 1.4rem); line-height: 1.5; color: var(--fg-2); max-width: 820px; }
	.lead a { color: var(--teal); text-decoration: underline; text-underline-offset: 3px; }
	.small { font-size: 1rem; line-height: 1.5; color: var(--fg-2); }
	.hint { font-size: 0.95rem; color: var(--fg-2); opacity: 0.8; }

	.grid { display: grid; gap: 20px; }
	.grid.three { grid-template-columns: repeat(3, minmax(0, 1fr)); }
	.grid.four { grid-template-columns: repeat(4, minmax(0, 1fr)); }
	.card { background: var(--ink-2); border: 1.5px solid var(--line); border-radius: 18px; padding: 26px; display: flex; flex-direction: column; gap: 12px; }
	.alt .card { background: var(--ink); }
	.card p { font-size: 1.08rem; line-height: 1.45; color: var(--fg-2); }
	.card .num { font-family: var(--font-display); font-size: 3rem; font-weight: 800; line-height: 1; color: var(--teal); }
	.card .tag { font-size: 0.8rem; font-weight: 700; letter-spacing: 0.14em; text-transform: uppercase; color: var(--amber); }
	.card .big { font-family: var(--font-display); font-size: clamp(2rem, 3.4vw, 3rem); font-weight: 800; line-height: 1; color: var(--teal); }
	.card.link { text-decoration: none; transition: border-color 0.15s; overflow-wrap: anywhere; }
	.card.link:hover { border-color: var(--teal); }
	.ico { width: 40px; height: 40px; fill: none; stroke: var(--teal); stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }

	.table { overflow-x: auto; border: 1.5px solid var(--line); border-radius: 14px; }
	table { width: 100%; border-collapse: collapse; font-size: 1.05rem; color: var(--fg-2); min-width: 560px; }
	th, td { text-align: left; padding: 14px 18px; border-bottom: 1px solid var(--line); vertical-align: top; }
	tbody tr:last-child td { border-bottom: none; }
	thead th { background: var(--ink-2); color: var(--fg); font-weight: 700; }
	.alt thead th { background: var(--ink); }
	td:first-child { color: var(--fg); font-weight: 600; }
	.hl { color: var(--fg); }
	thead .hl { color: var(--teal); }

	.slide.video { padding: 0; background: #0b1418; justify-content: center; }
	.slide.video video { width: 100%; max-height: 100dvh; aspect-ratio: 16 / 9; object-fit: contain; display: block; }
	.slide.video.full { padding: 56px 24px; }
	.vwrap { width: 100%; max-width: 1400px; margin: 0 auto; display: flex; flex-direction: column; gap: 16px; }
	.slide.video.full video { max-height: calc(100dvh - 150px); border-radius: 14px; background: #0b1418; }

	/* Arrival: each slide's parts rise in once, staged. */
	.r { opacity: 0; transform: translateY(18px); transition: opacity 0.6s ease, transform 0.6s cubic-bezier(0.2, 0.7, 0.2, 1); transition-delay: calc(var(--i, 0) * 90ms); }
	:global(.slide.in) .r { opacity: 1; transform: none; }
	.route { stroke-dasharray: 1200; stroke-dashoffset: 1200; }
	:global(.slide.in) .route { animation: draw 1.6s 0.5s ease forwards; }
	@keyframes draw { to { stroke-dashoffset: 0; } }
	@media (prefers-reduced-motion: reduce) {
		.r { opacity: 1; transform: none; transition: none; }
		.route { stroke-dashoffset: 0; animation: none !important; }
	}

	.controls { position: fixed; right: 20px; bottom: 20px; z-index: 30; display: flex; align-items: center; gap: 6px; padding: 6px; border-radius: 999px; background: rgba(11, 20, 24, 0.82); border: 1px solid var(--line); backdrop-filter: blur(8px); }
	.controls .count { font-size: 0.85rem; color: var(--fg-2); padding: 0 8px; }
	.controls button { height: 36px; min-width: 36px; display: grid; place-items: center; border-radius: 999px; color: var(--fg); }
	.controls button:hover:not(:disabled) { background: var(--line); }
	.controls button:disabled { opacity: 0.35; }
	.controls svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
	.controls .present { padding: 0 14px; font-size: 0.85rem; font-weight: 700; color: var(--ink); background: var(--teal); }
	.controls .present:hover { background: var(--teal) !important; filter: brightness(1.08); }

	/* Present: full screen, one slide per screen, snapping. */
	.deck.presenting { overflow-y: auto; height: 100vh; scroll-snap-type: y mandatory; }
	.deck.presenting .slide { height: 100vh; min-height: 0; scroll-snap-align: start; }
	.deck.presenting .wrap { max-width: 1400px; }
	.deck.presenting .controls { position: fixed; }

	@media (max-width: 1000px) {
		.grid.four { grid-template-columns: repeat(2, minmax(0, 1fr)); }
		.cover { flex-direction: column; align-items: flex-start; }
		.art { max-width: 420px; }
	}
	@media (max-width: 640px) {
		.slide { min-height: auto; padding: 56px 16px; }
		.grid.three, .grid.four { grid-template-columns: minmax(0, 1fr); }
		.card { padding: 20px; }
		.controls { right: 12px; bottom: 12px; }
		.hint { display: none; }
		.slide.video video { max-height: none; }
	}
</style>
