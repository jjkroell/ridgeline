// The permanent #mayday channel's alert flag (an emergency word there pulses the
// #mayday row and the Channels nav item until the viewer switches to #mayday), its two cards — what the channel is for (info) and
// who monitors it — each either open or minimized to a pill beside the channel
// name, per browser. Also the roster from the server, and the rules that keep
// the chat clean: "join" / "leave" / "test" housekeeping messages are hidden
// from the #mayday chat five minutes after they are heard, calls for help after
// 12 hours, everything else after 3.
// Shared by the desktop and mobile channel pages and the Channels widget.
import { serverNow } from '$lib/server-clock.svelte';
import { api, type SOSMonitor } from './api';
import { channels, SOS_ID } from './channels.svelte';
import { sosAlarm } from './sos-alarm.svelte';

const INFO_KEY = 'ridgeline-sos-card';
const ALERT_ACK_KEY = 'ridgeline-sos-alert-ack';
/** How long an emergency word on #mayday keeps it flagged, unless opened first. */
export const ALERT_WINDOW_MS = 12 * 60 * 60_000;
// Emergency words, whole-word, any case. "#mayday" (the channel's own tag, as in
// "announce #mayday on Public") does not count, nor does a bare "help" ("thanks
// for the help"); "need help" / "help me" do. Mirrors sosAlertRe in
// internal/store/sos.go — keep the two in step.
const ALERT_RE = /(?<![#\w])(sos|emergency|mayday|911|threat|life)(?!\w)|\bneed(s)? help\b|\bhelp me\b/i;

// A message opening with an @-mention is a reply, not a new call for help
// (store.sosReplyRe).
const REPLY_RE = /^\s*@\[/;

/** Whether a message is a call for help: an emergency word, and not a reply. */
export function alertWord(text: string): boolean {
	return ALERT_RE.test(text) && !REPLY_RE.test(text);
}
const MONITORS_KEY = 'ridgeline-sos-monitors';
const ALERTS_KEY = 'ridgeline-sos-alerts-card';
/** Map rings this browser hid: sender → the alert time hidden (epoch ms). */
const RING_HIDE_KEY = 'ridgeline-sos-ring-hidden';
/** The three #mayday cards; at most one is open, the rest are pills. */
export type SosCardKind = 'info' | 'monitors' | 'alerts';
/** Only alerts this fresh ring the alarm (not ones found on load or reconnect). */
const ALARM_FRESH_MS = 5 * 60_000;
// How often an open page refreshes the roster; it changes only when someone
// goes on or off the list, so this is slow on purpose.
const REFRESH_MS = 60_000;
/** A housekeeping message stays in the #mayday chat this long, then is hidden. */
export const HIDE_AFTER_MS = 5 * 60_000;
/** Any other #mayday message stays this long, then is hidden. */
export const CHAT_HIDE_MS = 3 * 60 * 60_000;
/** A call for help (an emergency word, not a reply) stays as long as its alert. */
export const ALERT_HIDE_MS = ALERT_WINDOW_MS;

function firstWord(text: string): string {
	const first = text.trim().split(/\s+/)[0] ?? '';
	return first.replace(/[\p{P}]+$/u, '').toLowerCase();
}

/** Housekeeping on #mayday, hidden after five minutes: a message whose first word
 *  (any case, trailing punctuation allowed) is "test" — radio checks, the
 *  monthly test — or "join" / "leave", which listed people until v0.24.78 and
 *  which some may still send. */
export function housekeeping(text: string): boolean {
	return ['test', 'join', 'leave'].includes(firstWord(text));
}

function readFlag(key: string, fallback: 'open' | 'min'): { value: 'open' | 'min'; seen: boolean } {
	try {
		const v = localStorage.getItem(key);
		if (v === 'open' || v === 'min') return { value: v, seen: true };
	} catch {
		/* storage unavailable — fall back every time */
	}
	return { value: fallback, seen: false };
}

function writeFlag(key: string, min: boolean) {
	try {
		localStorage.setItem(key, min ? 'min' : 'open');
	} catch {
		/* storage unavailable */
	}
}

class SOS {
	/** The admin's switch (health, then each roster poll): off, nothing here
	 *  flags, rings, alarms or hides, and the pages leave #mayday plain. */
	enabled = $state(false);
	/** Info card minimized to its pill. Defaults open, so first launch shows it. */
	infoMinimized = $state(false);
	/** Monitor list minimized to its pill (the default — the pill carries the count). */
	monitorsMinimized = $state(true);
	/** This browser's alarm switches minimized to their pill (the default). */
	alertsMinimized = $state(true);
	/** Whether this browser has seen the #mayday card before (false = first launch). */
	seen = $state(false);
	monitors = $state<SOSMonitor[]>([]);
	/** Newest #mayday alert the server heard (epoch ms; 0 = none in 12 h). */
	lastAlert = $state(0);
	/** Who sent #mayday alerts and when (epoch ms), newest per sender — for the map rings. */
	alertSenders = $state<{ sender: string; at: number }[]>([]);
	/** Newest alert an admin resolved (epoch ms): the page's own messages up to it don't pulse. */
	clearedAt = $state(0);
	/** Rings hidden on this browser's maps: sender → newest alert time hidden. */
	ringHidden = $state<Record<string, number>>({});
	loaded = $state(false);
	/** Newest #mayday alert this browser has seen by opening the channel (epoch ms). */
	alertAckAt = $state(0);
	/** Clock for the five-minute hide; ticks while a page watches. */
	now = $state(serverNow());
	#inited = false;
	#timers: ReturnType<typeof setInterval>[] = [];
	#users = 0;

	init() {
		if (this.#inited) return;
		this.#inited = true;
		const info = readFlag(INFO_KEY, 'open');
		this.seen = info.seen;
		this.infoMinimized = info.value === 'min';
		this.monitorsMinimized = readFlag(MONITORS_KEY, 'min').value === 'min';
		this.alertsMinimized = readFlag(ALERTS_KEY, 'min').value === 'min';
		// One card open at a time (older saves could have two).
		if (!this.infoMinimized) this.monitorsMinimized = this.alertsMinimized = true;
		else if (!this.monitorsMinimized) this.alertsMinimized = true;
		try {
			this.ringHidden = JSON.parse(localStorage.getItem(RING_HIDE_KEY) || '{}') || {};
		} catch {
			/* storage unavailable or garbled — show every ring */
		}
		try {
			this.alertAckAt = Number(localStorage.getItem(ALERT_ACK_KEY)) || 0;
		} catch {
			/* storage unavailable — alerts re-show after a reload */
		}
	}

	/** First launch is used up once a channels page has shown #mayday. */
	markSeen() {
		if (this.seen) return;
		this.seen = true;
		writeFlag(INFO_KEY, this.infoMinimized);
		writeFlag(MONITORS_KEY, this.monitorsMinimized);
		writeFlag(ALERTS_KEY, this.alertsMinimized);
	}

	/**
	 * The server's view, for every page: an emergency word on #mayday within the
	 * window, newer than the last time this browser switched to #mayday. Updated
	 * by the roster poll (once a minute) and at once by the live feed (noteLive).
	 */
	get flagged(): boolean {
		return this.enabled && this.lastAlert > this.alertAckAt && this.now - this.lastAlert < ALERT_WINDOW_MS;
	}

	/**
	 * Whether the #mayday row should pulse: the server's flag, or (instantly, from
	 * the page's own decoded messages — a newest-last list of {text, t}) a
	 * message there within the window that mentions an emergency word, newer
	 * than the last time this browser switched to #mayday and than any alert an
	 * admin has resolved.
	 */
	alerting(msgs: { text: string; t: number }[] | undefined): boolean {
		if (this.flagged) return true;
		if (!msgs || !this.enabled) return false;
		for (let i = msgs.length - 1; i >= 0; i--) {
			const m = msgs[i];
			if (m.t <= this.alertAckAt || m.t <= this.clearedAt || this.now - m.t > ALERT_WINDOW_MS) return false;
			if (alertWord(m.text)) return true;
		}
		return false;
	}

	/**
	 * Alert senders within the window, for the map rings. Unlike the channel
	 * pulse, opening #mayday doesn't clear them: the map is where you look for
	 * where help is needed, for as long as the alert is fresh.
	 */
	get ringAlerts(): { sender: string; at: number }[] {
		if (!this.enabled) return [];
		return this.alertSenders.filter(
			(a) => this.now - a.at < ALERT_WINDOW_MS && !((this.ringHidden[a.sender] ?? 0) >= a.at)
		);
	}

	/** Hide a sender's ring on this browser's maps, until they send a newer alert. */
	hideRing(sender: string, at: number) {
		const now = serverNow();
		const kept = Object.fromEntries(
			Object.entries(this.ringHidden).filter(([, t]) => now - t < ALERT_WINDOW_MS)
		);
		this.ringHidden = { ...kept, [sender]: at };
		try {
			localStorage.setItem(RING_HIDE_KEY, JSON.stringify(this.ringHidden));
		} catch {
			/* storage unavailable — hidden until reload */
		}
	}

	/** Clear the pulse: the user switched to #mayday, so everything heard until now is seen. */
	ackAlert() {
		// max(): the server's clock stamps lastAlert; never leave it ahead of the ack.
		this.alertAckAt = Math.max(serverNow(), this.lastAlert);
		try {
			localStorage.setItem(ALERT_ACK_KEY, String(this.alertAckAt));
		} catch {
			/* storage unavailable */
		}
	}

	/** Open one card (minimizing the others to their pills) or minimize it. */
	setCard(kind: SosCardKind, open: boolean) {
		// Opening: only `kind` stays open. Minimizing: `kind` closes, others keep.
		const min = (k: SosCardKind, was: boolean) => (open ? k !== kind : k === kind || was);
		this.infoMinimized = min('info', this.infoMinimized);
		this.monitorsMinimized = min('monitors', this.monitorsMinimized);
		this.alertsMinimized = min('alerts', this.alertsMinimized);
		writeFlag(INFO_KEY, this.infoMinimized);
		writeFlag(MONITORS_KEY, this.monitorsMinimized);
		writeFlag(ALERTS_KEY, this.alertsMinimized);
	}

	/**
	 * Whether a decrypted message is hidden from the chat, by its age (t, epoch
	 * ms) on #mayday: a join/leave/test after five minutes, a call for help after 12
	 * hours (its alert window), anything else — replies included — after 3.
	 */
	hidden(channelId: string, text: string, t: number): boolean {
		if (channelId !== SOS_ID || !this.enabled) return false;
		const limit = housekeeping(text) ? HIDE_AFTER_MS : alertWord(text) ? ALERT_HIDE_MS : CHAT_HIDE_MS;
		return this.now - t > limit;
	}

	/**
	 * A packet from the live feed: an emergency word on #mayday flags the channel
	 * at once, instead of on the next roster poll.
	 */
	noteLive(ev: { channel?: string; text?: string; sender?: string; messageHash: string; receivedAt: string }) {
		if (!this.enabled || ev.channel !== SOS_ID || !ev.text || !alertWord(ev.text)) return;
		const t = +new Date(ev.receivedAt);
		if (t > this.lastAlert) {
			this.lastAlert = t;
			this.now = serverNow();
		}
		const sender = ev.sender?.trim();
		const prev = sender ? this.alertSenders.find((a) => a.sender === sender) : undefined;
		if (sender && (!prev || t > prev.at))
			this.alertSenders = [{ sender, at: t }, ...this.alertSenders.filter((a) => a.sender !== sender)];
		this.#alarm(t, ev.messageHash, ev.sender, ev.text);
	}

	/** Ring the opt-in alarm for a fresh alert this browser hasn't acknowledged. */
	#alarm(t: number, key: string, sender?: string, text?: string) {
		if (t > this.alertAckAt && serverNow() - t < ALARM_FRESH_MS) sosAlarm.fire(key, sender, text);
	}

	async load() {
		try {
			const r = await api.sosMonitors();
			this.setEnabled(r.enabled === true);
			this.monitors = r.monitors ?? [];
			const t = r.lastAlert ? +new Date(r.lastAlert) : 0;
			// A newer alert than the last poll saw (and the live feed missed): ring.
			if (this.loaded && t > this.lastAlert) this.#alarm(t, 'poll:' + t);
			this.lastAlert = t;
			this.alertSenders = (r.alerts ?? []).map((a) => ({ sender: a.sender, at: +new Date(a.at) }));
			this.clearedAt = r.clearedAt ? +new Date(r.clearedAt) : 0;
		} catch {
			/* keep the last list */
		} finally {
			this.loaded = true;
		}
	}

	/** Follow the admin's switch; on also builds #mayday into the channel list. */
	setEnabled(on: boolean) {
		if (this.enabled === on) return;
		this.enabled = on;
		channels.setEmergency(on);
	}

	/** Keep the roster and the clock fresh while a page shows them; returns the stop function. */
	watch(): () => void {
		if (this.#users++ === 0) {
			this.now = serverNow();
			this.load();
			this.#timers = [
				setInterval(() => this.load(), REFRESH_MS),
				setInterval(() => (this.now = serverNow()), 15_000)
			];
		}
		return () => {
			if (--this.#users === 0) this.#timers.forEach(clearInterval);
		};
	}
}

export const sos = new SOS();
