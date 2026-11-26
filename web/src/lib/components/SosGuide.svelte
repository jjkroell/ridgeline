<script lang="ts">
	// Explainer for the permanent #mayday channel: what it is for, how to join
	// and monitor it, and what happens to a message sent on it — the radios,
	// Ridgeline's alerts and the KOD-BOT auto reply.
	//
	// A linkable page (/sos) so it can be shared on its own and sections
	// linked directly — #flow, #triggers, #alerts. The facts here follow the
	// code; keep them in step:
	//   - trigger words: store.sosAlertRe / sos.svelte.ts ALERT_RE (and the
	//     bot's contrib/sos_ack plugin in jjkroell/meshcore-bot)
	//   - pulse window 12 h (SOSAlertWindow / ALERT_WINDOW_MS)
	//   - emails: 30 s batch, then at most one per 10 min, except a real call after a
	//     tests-only email (api/sos_alerts.go)
	//   - monitor list: accounts that answer Yes (SosMonitorToggle, store.SetSOSMonitor)
	//   - chat clean-up (#cleanup): test (and stray join/leave) hidden after 5 min, calls for
	//     help after 12 h, everything else after 3 h (sos.svelte.ts hidden())
	//   - a message opening with an @-mention never alerts (sosReplyRe)
	//   - monthly test: KOD-BOT [Scheduled_Messages] `0 9 1 * *` on the bot Pi
	//   - map ring: lib/sos-map.ts (12 h, name match, hide per browser, admin resolve)
	import SosMonitorToggle from './SosMonitorToggle.svelte';

	let { compact = false }: { compact?: boolean } = $props();
	const m = $derived(compact ? '/m' : '');

	const sections = [
		{ id: 'limits', label: 'Limits & responsibility' },
		{ id: 'about', label: 'What #mayday is' },
		{ id: 'join', label: 'Join & monitor' },
		{ id: 'flow', label: 'Message flow' },
		{ id: 'alerts', label: 'Ridgeline alerts' },
		{ id: 'bot', label: 'Auto reply' },
		{ id: 'cleanup', label: 'Clean-up' },
		{ id: 'triggers', label: 'Trigger words' }
	];

	const triggers = ['sos', 'emergency', 'mayday', '911', 'threat', 'life', 'need help', 'needs help', 'help me'];
	const notTriggers = ['#mayday', 'wildlife', 'lifeline', 'threats', 'thanks for the help', 'test'];
</script>

<div class="mx-auto max-w-3xl space-y-4 {compact ? 'px-4 py-4' : 'px-6 py-6 md:px-10'}">
	<!-- Read before the rest: amber, above the menu, ending with 911 first. -->
	<section
		id="limits"
		class="panel rise relative scroll-mt-24 space-y-2 overflow-hidden px-5 py-4 text-sm leading-relaxed"
		style="animation-delay:0ms;border-color:color-mix(in srgb, var(--color-amber) 45%, var(--color-line));background-image:linear-gradient(color-mix(in srgb, var(--color-amber) 9%, transparent), color-mix(in srgb, var(--color-amber) 9%, transparent))"
	>
		<span class="bg-amber absolute inset-y-0 left-0 w-[3px]" aria-hidden="true"></span>
		<h2 class="font-display text-fg flex items-center gap-2 text-base font-700">
			<svg viewBox="0 0 24 24" class="text-amber h-5 w-5 shrink-0" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 9v4M12 17h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z" /></svg>
			Limits and responsibility
		</h2>
		<p class="text-fg">
			#mayday and Ridgeline are run by volunteers and offered <span class="font-700">as is</span>, with
			<span class="font-700">no guarantee that a message will be sent, heard, emailed or answered</span>. Coverage has
			gaps, and radios, repeaters, the internet, Ridgeline, its emails and KOD-BOT can all fail without warning.
			People on the monitoring list haven't promised to respond.
		</p>
		<p class="text-fg">
			Anyone can post under any name, and <span class="font-700">everything on #mayday is public, including your
			location</span>.
		</p>
		<p class="text-fg">
			#mayday is <span class="font-700">not an emergency service</span> and must not replace 911, a satellite messenger
			or a personal locator beacon. If you choose to help, look after your own safety first.
		</p>
		<p class="text-fg">
			To the extent the law allows, the volunteers who run Ridgeline, the repeaters and KOD-BOT accept no liability
			for loss or injury from using #mayday or relying on it.
		</p>
		<p class="text-fg">
			In a life-threatening emergency, <span class="font-700">call 911 first</span> if you have any phone or cell
			service. The mesh is a backup, not a replacement: nobody is paid to watch it, and nothing on it reaches
			emergency services by itself.
		</p>
	</section>

	<nav style="animation-delay:30ms" class="panel rise flex flex-wrap gap-2 px-5 py-4">
		{#each sections as sct (sct.id)}
			<a
				href="#{sct.id}"
				class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
				>{sct.label}</a
			>
		{/each}
	</nav>

	<div class="panel rise space-y-6 px-5 py-5 text-sm leading-relaxed" style="animation-delay:40ms">
		<section id="about" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">What #mayday is</h2>
			<p class="text-fg-dim">
				<span class="text-fg font-mono">#mayday</span> is the mesh's channel for people who need help. Post there if
				you or someone near you needs assistance, and <span class="text-fg">say where you are</span>. Keep it
				clear otherwise — no chat or radio checks — so that every message on it is worth a notification.
			</p>
			<p class="text-fg-dim">
				It's an ordinary MeshCore hashtag channel, so anyone can join it with no key to share. A call for help
				on Public can get lost in the chatter, and many people keep Public notifications to mentions only.
				<span class="text-fg font-mono">#emergency</span> is for emergencies that affect everyone, like an
				earthquake; #mayday is for one person or group who needs help. Join both.
			</p>
		</section>

		<section id="join" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">Join and monitor</h2>
			<ol class="text-fg-dim list-decimal space-y-1.5 pl-5">
				<li>
					In your MeshCore app or radio, add a hashtag channel named <span class="text-fg font-mono">sos</span>.
				</li>
				<li>
					Enable notifications in your MeshCore app for <span class="text-fg font-mono">#mayday</span> to ensure you
					are alerted in real time.
				</li>
				<li>
					Let others know you're listening: start typing your radio's name below, pick it, answer
					<span class="text-fg">Yes</span>, and it goes on the <span class="text-fg">monitoring</span> list on
					Ridgeline's #mayday page. It needs a Ridgeline account, and a radio Ridgeline has heard: if it says the name
					isn't valid, send a flood advert from your radio, wait a minute and try again.
					<span class="text-fg">You only need to enter one companion</span>, even if you have several.
				</li>
			</ol>
			<div class="border-line bg-panel-2/40 rounded-[var(--radius)] border px-4 py-3">
				<SosMonitorToggle mobile={compact} />
			</div>
			<p class="text-fg-dim">
				There's nothing to send on the channel to join or leave. Answer <span class="text-fg">No</span> at any time
				to come off the list. You can also change it under <span class="text-fg">Monitoring</span> on #mayday, or in
				<a href="{m}/account#sos-monitor" class="text-coral hover:underline">Account</a>.
			</p>
			<p class="text-fg-dim">
				<span class="text-fg">Who's on the channel?</span> In the MeshCore app, open #mayday, tap
				<span class="text-fg">⋮</span> (the three-dot menu) → <span class="text-fg whitespace-nowrap"
					><svg viewBox="0 0 24 24" class="-mt-0.5 mr-0.5 inline h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>Channel Settings</span
				> →
				<span class="text-fg">Participants</span> to see who your radio has heard on it.
			</p>
		</section>

		<section id="flow" class="scroll-mt-24 space-y-3">
			<h2 class="font-display text-fg text-base font-700">What happens to a message on #mayday</h2>
			<div class="space-y-2">
				<div class="border-line rounded-[var(--radius)] border px-4 py-3">
					<div class="label">1 · You send</div>
					<p class="text-fg-dim mt-1">
						You post on #mayday from your radio or app (using a
						<a href="#triggers" class="text-coral hover:underline">trigger word</a>), for example:
					</p>
					<p class="border-line text-fg bg-panel-2/40 mt-2 rounded-[var(--radius)] border px-3 py-2 font-mono text-xs">
						SOS twisted ankle at the Elk Lake trailhead, need help getting out
					</p>
					<p
						data-sos-include
						class="text-fg mt-2.5 flex gap-2 rounded-[var(--radius)] px-3 py-2"
						style="background:color-mix(in srgb, var(--color-coral) 12%, transparent); border:1px solid color-mix(in srgb, var(--color-coral) 40%, transparent)"
					>
						<svg viewBox="0 0 24 24" class="text-coral mt-0.5 h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 21s-7-6.2-7-11a7 7 0 0 1 14 0c0 4.8-7 11-7 11z" /><circle cx="12" cy="10" r="2.5" /></svg>
						<span>
							<span class="text-coral font-600">Always include</span> where you need help — a place name,
							trail, road or coordinates — and what kind of help you're asking for.
						</span>
					</p>
				</div>
				<div class="text-fg-faint pl-6" aria-hidden="true">↓</div>
				<div class="border-line rounded-[var(--radius)] border px-4 py-3">
					<div class="label">2 · The mesh carries it</div>
					<p class="text-fg-dim mt-1">
						Repeaters in range flood it onward, hop by hop, across Vancouver Island and the Lower Mainland. Every
						radio that hears it gets the same message at about the same time.
					</p>
				</div>
				<div class="text-fg-faint pl-6" aria-hidden="true">↓ three places it lands</div>
				<div class="grid gap-2 md:grid-cols-3">
					<div class="border-line rounded-[var(--radius)] border px-4 py-3">
						<div class="label">Monitors' radios</div>
						<p class="text-fg-dim mt-1">
							Everyone with #mayday on their radio or app gets it directly, with a notification if they left them
							on. This is what reaches people who can help.
						</p>
					</div>
					<div class="border-line rounded-[var(--radius)] border px-4 py-3">
						<div class="label">Ridgeline</div>
						<p class="text-fg-dim mt-1">
							Ridgeline's observers hear it and the site shows it under #mayday. If it has a
							<a href="#triggers" class="text-coral hover:underline">trigger word</a>, Ridgeline raises the
							<a href="#alerts" class="text-coral hover:underline">alerts</a> below.
						</p>
					</div>
					<div class="border-line rounded-[var(--radius)] border px-4 py-3">
						<div class="label">KOD-BOT</div>
						<p class="text-fg-dim mt-1">
							If it has a trigger word, the bot <a href="#bot" class="text-coral hover:underline">replies on
							#mayday</a> so you know the mesh carried it.
						</p>
					</div>
				</div>
			</div>
			<p class="text-fg-faint text-xs">
				Each of the three hears the message only if the mesh reaches it. A reply from KOD-BOT shows your message got
				out; it doesn't mean anyone has read it yet. Watch #mayday for a person to answer.
			</p>
		</section>

		<section id="alerts" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">Ridgeline's alerts</h2>
			<p class="text-fg-dim">When a #mayday message with a trigger word is heard:</p>
			<ul class="text-fg-dim space-y-2">
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">A red pulse</span> on <span class="text-fg">Channels</span> in the menu and on
						the #mayday row, for everyone, until you open #mayday (or for 12 hours, or until an admin marks the alert
							resolved). No setting needed.</span
					>
				</li>
				<li id="map-ring" class="flex scroll-mt-24 gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">A red ring on the maps</span> (<a href="{m}/map" class="text-coral hover:underline">Map</a>
						and <a href="{m}/live-map" class="text-coral hover:underline">Live Map</a>), pulsing over the sender's node
						and labelled <span class="text-fg font-mono">SOS · name</span>, for 12 hours. It only appears if their radio
						shares its location in its adverts, and it sits at the last position the radio advertised, not a live GPS
						fix. Opening #mayday doesn't clear it. Tap the label to open #mayday or <span class="text-fg">hide it on your
						map</span>; an admin can mark it <span class="text-fg">resolved</span>, which clears it, and the red pulse, for everyone. A new
						alert from that person brings it back. <span class="text-fg">So responders can find you, turn on location
						sharing in your MeshCore app or radio.</span></span
					>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">A sound and a pop-up notification</span>, if you turn them on in the #mayday
						channel's <span class="text-fg">Alerts</span> card (on a phone, also under Settings). They're saved in
						that browser only, and Ridgeline has to be open — on a phone, on screen. On an iPhone, add Ridgeline to
						your home screen first to get notifications.</span
					>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">An email</span> with the message and a link to #mayday, to every account that turned
						on #mayday alerts under <a href="{m}/account#sos-alerts" class="text-coral hover:underline">Account</a>
						(needs a verified email). It works while Ridgeline is closed. The first email waits about 30 seconds so
						messages sent together arrive as one; after that, at most one email every 10 minutes, carrying
						everything heard in between. A real call right after a test email doesn't wait out those 10 minutes.
						Messages that say they're a test ("SOS test…") are left out unless you tick
						<span class="text-fg">Include test messages</span> there.</span
					>
				</li>
			</ul>
			<p id="monthly-test" class="text-fg-dim scroll-mt-24">
				<span class="text-fg">Monthly test.</span> At <span class="text-fg">9:00 AM Pacific on the 1st of every
				month</span>, KOD-BOT sends a test on #mayday:
				<span class="text-fg font-mono">SOS test message. no emergency in progress</span>. It sets off the alerts
				like a real call, to check the whole chain still works end to end — the mesh, Ridgeline's observers, the
				pulse, the map ring, notifications and email. There's nothing to do. The email goes only to accounts with
				<span class="text-fg">Include test messages</span> on, so turn that on if you want to confirm your alerts
				reach you.
			</p>
		</section>

		<section id="bot" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">The KOD-BOT auto reply</h2>
			<p class="text-fg-dim">
				KOD-BOT also listens on #mayday. When it hears a trigger word it replies on #mayday, addressed to you:
			</p>
			<p class="border-line text-fg bg-panel-2/40 rounded-[var(--radius)] border px-3 py-2 font-mono text-xs">
				@[your name] SOS received, email sent to Ridgeline Alert users. Send your location and nature of the emergency ASAP.
			</p>
			<ul class="text-fg-dim space-y-1.5">
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>The reply means the mesh carried your message as far as the bot's radio. It shows in #mayday on Ridgeline too.</span>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>One reply per person per minute, so repeated calls don't flood the channel. Replies that start with an
						@-mention get none.</span>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						>It opens with an @-mention, so it can't set off an alert itself. Messages that aren't calls for
						help (a <span class="text-fg font-mono">test</span>, chat) get no reply, and no other bot commands
						answer on #mayday.</span
					>
				</li>
			</ul>
		</section>

		<section id="cleanup" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">Clean-up</h2>
			<p class="text-fg-dim">To keep #mayday easy to read, Ridgeline clears messages from its #mayday view as they age:</p>
			<ul class="text-fg-dim space-y-1.5">
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>Messages starting with <span class="text-fg font-mono">test</span> after
						<span class="text-fg">5 minutes</span></span>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>Calls for help (with a <a href="#triggers" class="text-coral hover:underline">trigger word</a>)
						after <span class="text-fg">12 hours</span>, the same as the alert</span>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>Everything else, including KOD-BOT's replies, after <span class="text-fg">3 hours</span></span>
				</li>
			</ul>
			<p class="text-fg-dim">
				This only tidies Ridgeline's view. The messages have already gone out over the radio, so they
				<span class="text-fg">stay in the MeshCore app</span> and on every other radio that heard them, until
				you delete them there.
			</p>
		</section>

		<section id="triggers" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">Trigger words</h2>
			<p class="text-fg-dim">
				Ridgeline's alerts and the auto reply use the same words, anywhere in the message, in any case, as whole
				words:
			</p>
			<div class="flex flex-wrap gap-1.5">
				{#each triggers as t (t)}
					<span
						class="text-coral rounded-[var(--radius)] border px-2 py-0.5 font-mono text-xs"
						style="border-color:color-mix(in srgb, var(--color-coral) 40%, var(--color-line))">{t}</span
					>
				{/each}
			</div>
			<p class="text-fg-dim">
				So <span class="text-fg">"SOS at Mt Seymour"</span>, <span class="text-fg">"life-threatening injury"</span>
				and <span class="text-fg">"can someone help me"</span> all count. These don't:
			</p>
			<div class="flex flex-wrap gap-1.5">
				{#each notTriggers as t (t)}
					<span class="border-line text-fg-dim rounded-[var(--radius)] border px-2 py-0.5 font-mono text-xs">{t}</span>
				{/each}
			</div>
			<p class="text-fg-faint text-xs">
				"#mayday" on its own is the channel's name, and a bare "help" doesn't count, so "thanks for the help" stays
				quiet. A message without a trigger word still goes to everyone on #mayday — it just raises no alert.
			</p>
			<p class="text-fg-dim">
				<span class="text-fg">Replies don't count either.</span> A message that <span class="text-fg">starts with an
				@-mention</span> — <span class="text-fg font-mono">@[Dave] what's your emergency?</span> — is part of a
				conversation, not a new call, so it raises no alert, sends no email and gets no auto reply, even with a
				trigger word in it. So when you <span class="text-fg">call for help, don't start with someone's name</span>:
				write <span class="text-fg font-mono">SOS at the lake, @[Mike] is hurt</span>, not
				<span class="text-fg font-mono">@[Mike] SOS at the lake</span>. And when you reply to a call, start with
				their @-mention so the channel doesn't alert again.
			</p>
		</section>
	</div>

	<p class="text-fg-faint px-1 text-xs">
		See it live: <a href="{m}/channels?channel=mayday" class="text-coral hover:underline">open #mayday on Ridgeline</a>.
	</p>
</div>
