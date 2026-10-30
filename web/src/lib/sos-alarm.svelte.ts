// The #sos alarm: an opt-in sound and system notification for a new emergency
// word on #sos (the same trigger as the red pulse; sos.svelte.ts calls fire()).
// Both are off by default and kept per browser. Browsers only let audio start
// after a click or tap, so while the sound is on, the first gesture on any page
// unlocks it (the live-map chime works the same way); "Test" does it too.
const SOUND_KEY = 'ridgeline-sos-sound';
const NOTIFY_KEY = 'ridgeline-sos-notify';
// Shared by every tab of this browser, so two open tabs ring once, not twice.
const LAST_KEY = 'ridgeline-sos-alarm-last';
/** Alerts closer together than this ring once. */
const QUIET_MS = 60_000;
const UNLOCK_EVENTS = ['pointerdown', 'touchend', 'click', 'keydown'];

function readOn(key: string): boolean {
	try {
		return localStorage.getItem(key) === '1';
	} catch {
		return false;
	}
}

function writeOn(key: string, on: boolean) {
	try {
		localStorage.setItem(key, on ? '1' : '0');
	} catch {
		/* storage unavailable — the choice lasts for this page only */
	}
}

class SosAlarm {
	sound = $state(false);
	notify = $state(false);
	/** Sound is on but the browser hasn't allowed audio yet (no click since load). */
	blocked = $state(false);
	/** Notification permission: 'default' | 'granted' | 'denied' | 'unsupported'. */
	permission = $state<string>('unsupported');
	/** Feedback under the notification switch (a closed prompt, a test sent…). */
	notifyNote = $state('');
	#actx: AudioContext | null = null;
	#armed = false;
	#inited = false;

	init() {
		if (this.#inited || typeof window === 'undefined') return;
		this.#inited = true;
		this.permission = 'Notification' in window ? Notification.permission : 'unsupported';
		this.sound = readOn(SOUND_KEY);
		this.notify = readOn(NOTIFY_KEY) && this.permission === 'granted';
		if (this.sound) this.#arm();
		// The permission can change in the browser's site settings while the page
		// is open (e.g. un-blocking it): follow it, where the browser says so, and
		// re-read it whenever the tab comes back into focus.
		if (this.permission !== 'unsupported') {
			navigator.permissions
				?.query({ name: 'notifications' })
				.then((st) => (st.onchange = this.#syncPermission))
				.catch(() => {});
			window.addEventListener('focus', this.#syncPermission);
		}
	}

	#syncPermission = () => {
		const was = this.permission;
		this.permission = Notification.permission;
		if (this.permission !== 'granted') this.notify = false;
		else if (was !== 'granted') this.notifyNote = '';
	};

	setSound(on: boolean) {
		this.sound = on;
		writeOn(SOUND_KEY, on);
		if (on) {
			this.#unlock(); // called from the switch's click, so audio may start now
			this.#arm();
		} else this.#disarm();
	}

	/** Turning notifications on asks the browser for permission (from the click). */
	async setNotify(on: boolean) {
		if (on && this.permission !== 'granted') {
			if (this.permission === 'unsupported') return;
			// Chrome's quieter prompt is only a bell in the address bar, and the
			// request stays pending until it is answered there: say where to look.
			this.notifyNote =
				'Waiting for your answer. No box? Look for a bell icon at the left or right end of the address bar.';
			try {
				this.permission = await Notification.requestPermission();
			} catch {
				/* very old Safari: callback form only — treat as not granted */
			}
			if (this.permission !== 'granted') on = false;
			// 'default' after asking = the prompt was closed (or auto-dismissed).
			this.notifyNote =
				this.permission === 'default' ? 'The browser’s question was closed without an answer — tick the box to be asked again.' : '';
		}
		this.notify = on;
		writeOn(NOTIFY_KEY, on);
	}

	/** Ring and notify once for a new alert; `key` identifies the transmission. */
	fire(key: string, sender?: string, text?: string) {
		if (!this.sound && !this.notify) return;
		const now = Date.now();
		try {
			const last = JSON.parse(localStorage.getItem(LAST_KEY) ?? 'null');
			if (last && (last.key === key || now - last.at < QUIET_MS)) return;
			localStorage.setItem(LAST_KEY, JSON.stringify({ key, at: now }));
		} catch {
			/* storage unavailable — ring anyway */
		}
		if (this.sound) this.#ring();
		if (this.notify) this.#notify(sender, text);
	}

	/** The Test button: ring and/or show a sample notification. */
	test() {
		if (this.sound) {
			this.#unlock();
			this.#ring();
		}
		if (this.notify)
			this.#notify('Test', 'This is what a #sos alert looks like. No one needs help.').then((err) => {
				this.notifyNote = err
					? `The browser refused the notification (${err}).`
					: 'Test notification sent. Nothing appeared? Check that your computer or phone allows notifications from this browser, and that Do Not Disturb or Focus is off.';
			});
	}

	// --- sound ---

	#unlock = () => {
		try {
			if (!this.#actx) {
				const AC =
					window.AudioContext ??
					(window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
				this.#actx = new AC();
				this.#actx.onstatechange = () => this.#sync();
			}
			if (this.#actx.state !== 'running') this.#actx.resume().then(this.#sync, this.#sync);
		} catch {
			/* no Web Audio */
		}
		this.#sync();
	};

	#sync = () => {
		const running = this.#actx?.state === 'running';
		this.blocked = this.sound && !running;
		if (running || !this.sound) this.#disarm();
		else this.#arm();
	};

	#arm() {
		if (this.#armed) return;
		this.#armed = true;
		this.blocked = this.#actx?.state !== 'running';
		for (const e of UNLOCK_EVENTS) window.addEventListener(e, this.#unlock, { capture: true, passive: true });
	}

	#disarm() {
		if (!this.#armed) return;
		this.#armed = false;
		for (const e of UNLOCK_EVENTS) window.removeEventListener(e, this.#unlock, { capture: true });
	}

	/** Two rising three-note bells, about two seconds: urgent, not a siren. */
	#ring() {
		const ctx = this.#actx;
		if (!ctx || ctx.state !== 'running') return;
		const out = ctx.createGain();
		out.gain.value = 0.5;
		out.connect(ctx.destination);
		const notes = [659.25, 830.61, 987.77]; // E5, G#5, B5
		const t0 = ctx.currentTime + 0.05;
		for (let rep = 0; rep < 2; rep++) {
			notes.forEach((f, i) => {
				const t = t0 + rep * 0.95 + i * 0.16;
				for (const [mult, level, type] of [
					[1, 0.6, 'triangle'],
					[2.01, 0.15, 'sine']
				] as const) {
					const osc = ctx.createOscillator();
					const g = ctx.createGain();
					osc.type = type;
					osc.frequency.value = f * mult;
					g.gain.setValueAtTime(0.0001, t);
					g.gain.exponentialRampToValueAtTime(level, t + 0.01);
					g.gain.exponentialRampToValueAtTime(0.0001, t + 0.7);
					osc.connect(g).connect(out);
					osc.start(t);
					osc.stop(t + 0.75);
				}
			});
		}
	}

	// --- notification ---

	/** Shows the notification; resolves to an error message, or '' when shown. */
	async #notify(sender?: string, text?: string): Promise<string> {
		this.#syncPermission();
		if (this.permission !== 'granted') return 'notifications are not allowed for this site';
		const title = sender ? `SOS on the mesh: ${sender}` : 'SOS on the mesh';
		const opts: NotificationOptions = {
			body: text?.trim() || 'An emergency word was heard on #sos.',
			tag: 'ridgeline-sos',
			// @ts-expect-error renotify: alert again even when replacing a shown one (Chrome)
			renotify: true,
			requireInteraction: true,
			icon: '/icons/icon-192.png',
			data: { url: '/channels?channel=sos' }
		};
		// Phones (Android Chrome, iOS home-screen apps) only show notifications
		// through the service worker; desktops take either.
		let swErr = '';
		try {
			const reg = await navigator.serviceWorker?.getRegistration();
			if (reg) {
				await reg.showNotification(title, opts);
				return '';
			}
		} catch (e) {
			swErr = e instanceof Error ? e.message : String(e); // try the page notification
		}
		try {
			const n = new Notification(title, opts);
			n.onclick = () => {
				window.focus();
				location.href = '/channels?channel=sos';
				n.close();
			};
			return '';
		} catch (e) {
			return swErr || (e instanceof Error ? e.message : String(e));
		}
	}
}

export const sosAlarm = new SosAlarm();
