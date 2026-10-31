<script lang="ts">
	// Explainer for the permanent #sos channel: what it is for, how to join
	// and monitor it, and what happens to a message sent on it — the radios,
	// Ridgeline's alerts and the KOD-BOT auto reply.
	//
	// A linkable page (/sos) so it can be shared on its own and sections
	// linked directly — #flow, #triggers, #alerts. The facts here follow the
	// code; keep them in step:
	//   - trigger words: store.sosAlertRe / sos.svelte.ts ALERT_RE (and the
	//     bot's contrib/sos_ack plugin in jjkroell/meshcore-bot)
	//   - pulse window 12 h (SOSAlertWindow / ALERT_WINDOW_MS)
	//   - emails: 30 s batch, then at most one per 10 min (api/sos_alerts.go)
	//   - test/unsub hidden after 5 min (HIDE_AFTER_MS)
	let { compact = false }: { compact?: boolean } = $props();
	const m = $derived(compact ? '/m' : '');

	const sections = [
		{ id: 'about', label: 'What #sos is' },
		{ id: 'join', label: 'Join & monitor' },
		{ id: 'flow', label: 'Message flow' },
		{ id: 'alerts', label: 'Ridgeline alerts' },
		{ id: 'bot', label: 'Auto reply' },
		{ id: 'triggers', label: 'Trigger words' }
	];

	const triggers = ['sos', 'emergency', 'mayday', '911', 'threat', 'life', 'need help', 'needs help', 'help me'];
	const notTriggers = ['#sos', 'wildlife', 'lifeline', 'threats', 'thanks for the help', 'test'];
</script>

<div class="mx-auto max-w-3xl space-y-4 {compact ? 'px-4 py-4' : 'px-6 py-6 md:px-10'}">
	<nav class="panel rise flex flex-wrap gap-2 px-5 py-4">
		{#each sections as sct (sct.id)}
			<a
				href="#{sct.id}"
				class="border-line text-fg-dim hover:border-line-bright hover:text-fg rounded-[var(--radius)] border px-3 py-1.5 text-sm transition-colors"
				>{sct.label}</a
			>
		{/each}
	</nav>

	<div
		class="panel rise relative overflow-hidden px-5 py-4 text-sm leading-relaxed"
		style="animation-delay:20ms;border-color:color-mix(in srgb, var(--color-coral) 35%, var(--color-line))"
	>
		<span class="bg-coral absolute inset-y-0 left-0 w-[3px]" aria-hidden="true"></span>
		<p class="text-fg">
			In a life-threatening emergency, <span class="font-700">call 911 first</span> if you have any phone or cell
			service. The mesh is a backup, not a replacement: nobody is paid to watch it, and nothing on it reaches
			emergency services by itself.
		</p>
	</div>

	<div class="panel rise space-y-6 px-5 py-5 text-sm leading-relaxed" style="animation-delay:40ms">
		<section id="about" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">What #sos is</h2>
			<p class="text-fg-dim">
				<span class="text-fg font-mono">#sos</span> is the mesh's channel for people who need help. Post there if
				you or someone near you needs assistance, and <span class="text-fg">say where you are</span>. Keep it
				clear otherwise — no chat or radio checks — so that every message on it is worth a notification.
			</p>
			<p class="text-fg-dim">
				It's an ordinary MeshCore hashtag channel, so anyone can join it with no key to share. A call for help
				on Public can get lost in the chatter, and many people keep Public notifications to mentions only.
				<span class="text-fg font-mono">#emergency</span> is for emergencies that affect everyone, like an
				earthquake; #sos is for one person or group who needs help. Join both.
			</p>
		</section>

		<section id="join" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">Join and monitor</h2>
			<ol class="text-fg-dim list-decimal space-y-1.5 pl-5">
				<li>
					In your MeshCore app or radio, add a hashtag channel named <span class="text-fg font-mono">sos</span>.
				</li>
				<li>
					Send one — just one — <span class="text-fg font-mono">test</span> message on it. You're listed as a
					monitor on Ridgeline's #sos page, so everyone can see how many people are listening.
				</li>
				<li>
					<span class="text-fg">Leave notifications on for #sos</span>, so a real call reaches you.
				</li>
			</ol>
			<p class="text-fg-dim">
				Send <span class="text-fg font-mono">unsub</span> to come off the list. To keep the channel clean,
				Ridgeline hides test and unsub messages after 5 minutes; your name stays on the list. They have already
				gone out over the radio, so other apps may still show them.
			</p>
		</section>

		<section id="flow" class="scroll-mt-24 space-y-3">
			<h2 class="font-display text-fg text-base font-700">What happens to a message on #sos</h2>
			<div class="space-y-2">
				<div class="border-line rounded-[var(--radius)] border px-4 py-3">
					<div class="label">1 · You send</div>
					<p class="text-fg-dim mt-1">
						You post on #sos from your radio or app, e.g.
						<span class="text-fg font-mono">SOS twisted ankle at the Elk Lake trailhead, need help getting out</span>.
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
							Everyone with #sos on their radio or app gets it directly, with a notification if they left them
							on. This is what reaches people who can help.
						</p>
					</div>
					<div class="border-line rounded-[var(--radius)] border px-4 py-3">
						<div class="label">Ridgeline</div>
						<p class="text-fg-dim mt-1">
							Ridgeline's observers hear it and the site shows it under #sos. If it has a
							<a href="#triggers" class="text-coral hover:underline">trigger word</a>, Ridgeline raises the
							<a href="#alerts" class="text-coral hover:underline">alerts</a> below.
						</p>
					</div>
					<div class="border-line rounded-[var(--radius)] border px-4 py-3">
						<div class="label">KOD-BOT</div>
						<p class="text-fg-dim mt-1">
							If it has a trigger word, the bot <a href="#bot" class="text-coral hover:underline">replies on
							#sos</a> so you know the mesh carried it.
						</p>
					</div>
				</div>
			</div>
			<p class="text-fg-faint text-xs">
				Each of the three hears the message only if the mesh reaches it. A reply from KOD-BOT shows your message got
				out; it doesn't mean anyone has read it yet. Watch #sos for a person to answer.
			</p>
		</section>

		<section id="alerts" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">Ridgeline's alerts</h2>
			<p class="text-fg-dim">When a #sos message with a trigger word is heard:</p>
			<ul class="text-fg-dim space-y-2">
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">A red pulse</span> on <span class="text-fg">Channels</span> in the menu and on
						the #sos row, for everyone, until you open #sos (or for 12 hours). No setting needed.</span
					>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">A sound and a pop-up notification</span>, if you turn them on in the #sos
						channel's <span class="text-fg">Alerts</span> card (on a phone, also under Settings). They're saved in
						that browser only, and Ridgeline has to be open — on a phone, on screen. On an iPhone, add Ridgeline to
						your home screen first to get notifications.</span
					>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						><span class="text-fg">An email</span> with the message and a link to #sos, to every account that turned
						on #sos alerts under <a href="{m}/account#sos-alerts" class="text-coral hover:underline">Account</a>
						(needs a verified email). It works while Ridgeline is closed. The first email waits about 30 seconds so
						messages sent together arrive as one; after that, at most one email every 10 minutes, carrying
						everything heard in between.</span
					>
				</li>
			</ul>
		</section>

		<section id="bot" class="scroll-mt-24 space-y-2">
			<h2 class="font-display text-fg text-base font-700">The KOD-BOT auto reply</h2>
			<p class="text-fg-dim">
				KOD-BOT also listens on #sos. When it hears a trigger word it replies on #sos, addressed to you:
			</p>
			<p class="border-line text-fg bg-panel-2/40 rounded-[var(--radius)] border px-3 py-2 font-mono text-xs">
				@[your name] *auto reply*&nbsp; An email was sent to users who have it enabled in Ridgeline.
			</p>
			<ul class="text-fg-dim space-y-1.5">
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>The reply means the mesh carried your message as far as the bot's radio. It shows in #sos on Ridgeline too.</span>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span>One reply per person per minute, so repeated calls don't flood the channel.</span>
				</li>
				<li class="flex gap-2">
					<span class="text-coral shrink-0">·</span>
					<span
						>It never uses a trigger word itself, so it can't set off an alert. Messages that aren't calls for
						help (a <span class="text-fg font-mono">test</span>, chat) get no reply, and no other bot commands
						answer on #sos.</span
					>
				</li>
			</ul>
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
				"#sos" on its own is the channel's name, and a bare "help" doesn't count, so "thanks for the help" stays
				quiet. A message without a trigger word still goes to everyone on #sos — it just raises no alert.
			</p>
		</section>
	</div>

	<p class="text-fg-faint px-1 text-xs">
		See it live: <a href="{m}/channels?channel=sos" class="text-coral hover:underline">open #sos on Ridgeline</a>.
	</p>
</div>
