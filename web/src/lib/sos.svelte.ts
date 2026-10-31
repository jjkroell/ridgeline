// The permanent #sos channel's alert flag (an emergency word there pulses the
// #sos row and the Channels nav item until the viewer switches to #sos), its two cards — what the channel is for (info) and
// who monitors it — each either open or minimized to a pill beside the channel
// name, per browser. Also the roster from the server, and the rule that keeps
// the chat clean: "test" / "unsub" housekeeping messages are hidden from the
// #sos chat five minutes after they are heard (the sender stays listed).
// Shared by the desktop and mobile channel pages and the Channels widget.
import { api, type ChannelMonitor } from './api';
import { SOS_ID } from './channels.svelte';
import { sosAlarm } from './sos-alarm.svelte';

const INFO_KEY = 'ridgeline-sos-card';
const ALERT_ACK_KEY = 'ridgeline-sos-alert-ack';
/** How long an emergency word on #sos keeps it flagged, unless opened first. */
export const ALERT_WINDOW_MS = 12 * 60 * 60_000;
// Emergency words, whole-word, any case. "#sos" (the channel's own tag, as in
// "announce #sos on Public") does not count, nor does a bare "help" ("thanks
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
/** The three #sos cards; at most one is open, the rest are pills. */
export type SosCardKind = 'info' | 'monitors' | 'alerts';
/** Only alerts this fresh ring the alarm (not ones found on load or reconnect). */
const ALARM_FRESH_MS = 5 * 60_000;
// How often an open page refreshes the roster; it changes only when someone
// sends "test" or "unsub", so this is slow on purpose.
const REFRESH_MS = 60_000;
/** A housekeeping message stays in the #sos chat this long, then is hidden. */
export const HIDE_AFTER_MS = 5 * 60_000;

/**
 * Classifies a channel message by its first word, in any case, trailing
 * punctuation allowed: "test" checks in, "unsub"/"unsubscribe" checks out.
 * Mirrors monitorWord in internal/store/sos.go — keep the two in step.
 */
export function monitorWord(text: string): 'test' | 'unsub' | null {
	const first = text.trim().split(/\s+/)[0] ?? '';
	const w = first.replace(/[\p{P}]+$/u, '').toLowerCase();
	if (w === 'test') return 'test';
	if (w === 'unsub' || w === 'unsubscribe') return 'unsub';
	return null;
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
	/** Info card minimized to its pill. Defaults open, so first launch shows it. */
	infoMinimized = $state(false);
	/** Monitor list minimized to its pill (the default — the pill carries the count). */
	monitorsMinimized = $state(true);
	/** This browser's alarm switches minimized to their pill (the default). */
	alertsMinimized = $state(true);
	/** Whether this browser has seen the #sos card before (false = first launch). */
	seen = $state(false);
	monitors = $state<ChannelMonitor[]>([]);
	/** Newest #sos alert the server heard (epoch ms; 0 = none in 12 h). */
	lastAlert = $state(0);
	loaded = $state(false);
	/** Newest #sos alert this browser has seen by opening the channel (epoch ms). */
	alertAckAt = $state(0);
	/** Clock for the five-minute hide; ticks while a page watches. */
	now = $state(Date.now());
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
			this.alertAckAt = Number(localStorage.getItem(ALERT_ACK_KEY)) || 0;
		} catch {
			/* storage unavailable — alerts re-show after a reload */
		}
	}

	/** First launch is used up once a channels page has shown #sos. */
	markSeen() {
		if (this.seen) return;
		this.seen = true;
		writeFlag(INFO_KEY, this.infoMinimized);
		writeFlag(MONITORS_KEY, this.monitorsMinimized);
		writeFlag(ALERTS_KEY, this.alertsMinimized);
	}

	/**
	 * The server's view, for every page: an emergency word on #sos within the
	 * window, newer than the last time this browser switched to #sos. Updated
	 * by the roster poll (once a minute) and at once by the live feed (noteLive).
	 */
	get flagged(): boolean {
		return this.lastAlert > this.alertAckAt && this.now - this.lastAlert < ALERT_WINDOW_MS;
	}

	/**
	 * Whether the #sos row should pulse: the server's flag, or (instantly, from
	 * the page's own decoded messages — a newest-last list of {text, t}) a
	 * message there within the window that mentions an emergency word, newer
	 * than the last time this browser switched to #sos.
	 */
	alerting(msgs: { text: string; t: number }[] | undefined): boolean {
		if (this.flagged) return true;
		if (!msgs) return false;
		for (let i = msgs.length - 1; i >= 0; i--) {
			const m = msgs[i];
			if (m.t <= this.alertAckAt || this.now - m.t > ALERT_WINDOW_MS) return false;
			if (alertWord(m.text)) return true;
		}
		return false;
	}

	/** Clear the pulse: the user switched to #sos, so everything heard until now is seen. */
	ackAlert() {
		// max(): the server's clock stamps lastAlert; never leave it ahead of the ack.
		this.alertAckAt = Math.max(Date.now(), this.lastAlert);
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
	 * Whether a decrypted message is hidden from the chat: a #sos test/unsub
	 * heard (t, epoch ms) more than five minutes ago.
	 */
	hidden(channelId: string, text: string, t: number): boolean {
		return channelId === SOS_ID && this.now - t > HIDE_AFTER_MS && monitorWord(text) !== null;
	}

	/**
	 * A packet from the live feed: an emergency word on #sos flags the channel
	 * at once, instead of on the next roster poll.
	 */
	noteLive(ev: { channel?: string; text?: string; sender?: string; messageHash: string; receivedAt: string }) {
		if (ev.channel !== SOS_ID || !ev.text || !alertWord(ev.text)) return;
		const t = +new Date(ev.receivedAt);
		if (t > this.lastAlert) {
			this.lastAlert = t;
			this.now = Date.now();
		}
		this.#alarm(t, ev.messageHash, ev.sender, ev.text);
	}

	/** Ring the opt-in alarm for a fresh alert this browser hasn't acknowledged. */
	#alarm(t: number, key: string, sender?: string, text?: string) {
		if (t > this.alertAckAt && Date.now() - t < ALARM_FRESH_MS) sosAlarm.fire(key, sender, text);
	}

	async load() {
		try {
			const r = await api.sosMonitors();
			this.monitors = r.monitors ?? [];
			const t = r.lastAlert ? +new Date(r.lastAlert) : 0;
			// A newer alert than the last poll saw (and the live feed missed): ring.
			if (this.loaded && t > this.lastAlert) this.#alarm(t, 'poll:' + t);
			this.lastAlert = t;
		} catch {
			/* keep the last list */
		} finally {
			this.loaded = true;
		}
	}

	/** Keep the roster and the clock fresh while a page shows them; returns the stop function. */
	watch(): () => void {
		if (this.#users++ === 0) {
			this.now = Date.now();
			this.load();
			this.#timers = [
				setInterval(() => this.load(), REFRESH_MS),
				setInterval(() => (this.now = Date.now()), 15_000)
			];
		}
		return () => {
			if (--this.#users === 0) this.#timers.forEach(clearInterval);
		};
	}
}

export const sos = new SOS();
