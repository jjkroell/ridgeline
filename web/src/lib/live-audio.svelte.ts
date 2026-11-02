// The live-map chime: a mellow wind-chime struck softly as each propagation
// pulse reaches a node. Originally inline in the desktop MapLibre live map; lifted
// here so the WebGL-free Leaflet fallback can play the identical sound. Both read
// and write the same localStorage keys, so the on/off state and tuning a user sets
// on one carry over to the other. Browser-only, no backend (like theme/favorites).
const SOUND_KEY = 'ridgeline-livemap-sound';
const AUDIO_KEY = 'ridgeline-livemap-audio';

// Each chord is a pentatonic-style scale whose notes stay consonant however they
// overlap, so random hits always sound musical. Lower registers read mellower.
export const CHORDS: { id: string; label: string; notes: number[] }[] = [
	{ id: 'calm', label: 'Calm', notes: [261.63, 293.66, 329.63, 392.0, 440.0] }, // C major pentatonic
	{ id: 'mellow', label: 'Mellow', notes: [220.0, 261.63, 293.66, 329.63, 392.0] }, // A minor pentatonic
	{ id: 'zen', label: 'Zen', notes: [261.63, 293.66, 311.13, 392.0, 415.3] }, // Hirajoshi
	{ id: 'deep', label: 'Deep', notes: [130.81, 146.83, 164.81, 196.0, 220.0] } // C major pentatonic, low
];
// How long each chime rings out (fundamental decay, seconds).
export const RINGS: { id: string; label: string; decay: number }[] = [
	{ id: 'short', label: 'Short', decay: 1.0 },
	{ id: 'medium', label: 'Med', decay: 1.8 },
	{ id: 'long', label: 'Long', decay: 3.2 }
];

// Jingle Bells (public domain, J. Pierpont 1857), one note per node reached
// while a season with `sound: 'jingle'` is on. Hz, an octave up so it rings
// like a bell: E5 G5 C5 D5 F5.
const E = 659.25, G = 783.99, C = 523.25, D = 587.33, F = 698.46;
const JINGLE = [
	E, E, E, E, E, E, E, G, C, D, E,
	F, F, F, F, F, E, E, E, E, D, D, E, D, G,
	E, E, E, E, E, E, E, G, C, D, E,
	F, F, F, F, F, E, E, E, G, G, F, D, C
];

/** Extra sounds a seasonal theme layers onto the chime. */
export interface SeasonSound {
	/** Play the season's tune ('jingle' = Jingle Bells on sleigh bells) instead of the wind chime. */
	tune?: 'jingle';
	/** Santa's "ho ho ho" as his sleigh lands (at most once a minute, see HO_GAP_MS). */
	ho?: boolean;
}
const HO_GAP_MS = 60_000; // sleighs land constantly: at most one "ho ho ho" a minute

class LiveChime {
	on = $state(false);
	volume = $state(0.4); // master gain, 0..0.8
	chordId = $state('calm');
	ringId = $state('medium');

	/** Sound is on but the browser hasn't let audio start yet (no user gesture
	 *  since the page loaded, or it was suspended in the background). The
	 *  controls show a "tap to enable" hint while this is true. */
	blocked = $state(false);

	#actx: AudioContext | null = null;
	#master: GainNode | null = null;
	#lastTickAt = -Infinity;
	#tuneAt = 0; // index of the next Jingle Bells note
	#tuneLast = 0; // when the last note played (the tune restarts after a pause)
	#lastHoAt = -Infinity;
	#previewing = false; // preview(): wait for the ho ho ho clip to load rather than skip it
	#loaded = false;
	#armed = false;

	get scale(): number[] {
		return CHORDS.find((c) => c.id === this.chordId)?.notes ?? CHORDS[0].notes;
	}
	get ringDecay(): number {
		return RINGS.find((r) => r.id === this.ringId)?.decay ?? 1.8;
	}

	// Hydrate from localStorage. Safe to call repeatedly; only the first run reads.
	load() {
		if (this.#loaded || typeof localStorage === 'undefined') return;
		this.#loaded = true;
		try {
			this.on = localStorage.getItem(SOUND_KEY) === '1';
			const raw = localStorage.getItem(AUDIO_KEY);
			if (raw) {
				const a = JSON.parse(raw);
				if (typeof a.vol === 'number') this.volume = Math.max(0, Math.min(0.8, a.vol));
				if (typeof a.ch === 'string' && CHORDS.some((c) => c.id === a.ch)) this.chordId = a.ch;
				if (typeof a.rg === 'string' && RINGS.some((r) => r.id === a.rg)) this.ringId = a.rg;
			}
		} catch {
			/* storage unavailable or malformed */
		}
		// Sound was left on: audio can only start inside a user gesture, so try
		// now (browsers that remember this site's engagement allow it) and
		// otherwise unlock on the first interaction anywhere on the page.
		if (this.on) this.ensure();
	}

	// Browsers only let an AudioContext start (or resume) inside an activation
	// gesture, and they disagree on which events count: Chrome/Firefox accept a
	// mouse pointerdown, Safari/iOS only a touchend/click/keydown, and a wheel
	// zoom counts nowhere. So listen for all of them, in the capture phase (the
	// map canvas handles its own pointer events), and keep listening until the
	// context actually reports `running`.
	static #UNLOCK_EVENTS = ['pointerdown', 'pointerup', 'mousedown', 'touchend', 'click', 'keydown'];

	#unlock = () => {
		if (!this.on) return this.#disarm();
		this.ensure();
	};

	#arm() {
		if (this.#armed || typeof window === 'undefined') return;
		this.#armed = true;
		for (const ev of LiveChime.#UNLOCK_EVENTS) window.addEventListener(ev, this.#unlock, { capture: true, passive: true });
	}

	#disarm() {
		if (!this.#armed) return;
		this.#armed = false;
		for (const ev of LiveChime.#UNLOCK_EVENTS) window.removeEventListener(ev, this.#unlock, { capture: true });
	}

	#syncState = () => {
		const running = this.#actx?.state === 'running';
		this.blocked = this.on && !running;
		if (running || !this.on) this.#disarm();
		else this.#arm(); // suspended / iOS 'interrupted' → resume on the next gesture
	};

	#persist() {
		try {
			localStorage.setItem(SOUND_KEY, this.on ? '1' : '0');
			localStorage.setItem(AUDIO_KEY, JSON.stringify({ vol: this.volume, ch: this.chordId, rg: this.ringId }));
		} catch {
			/* storage unavailable */
		}
	}

	// Build the AudioContext on first use and resume it if it's suspended. It
	// only actually starts when called within a user gesture; outside one it
	// stays suspended and the unlock listeners take over.
	ensure() {
		if (!this.#actx) {
			const AC =
				window.AudioContext ??
				(window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
			if (!AC) return;
			this.#actx = new AC();
			this.#master = this.#actx.createGain();
			this.#master.gain.value = this.volume;
			this.#master.connect(this.#actx.destination);
			this.#actx.onstatechange = this.#syncState;
		}
		if (this.#actx.state !== 'running') this.#actx.resume().then(this.#syncState, this.#syncState);
		this.#syncState();
	}

	toggle() {
		this.on = !this.on;
		if (this.on) this.ensure(); // within the click gesture so the browser allows it
		else this.#syncState();
		this.#persist();
	}

	setVolume(v: number) {
		this.volume = v;
		if (this.#master) this.#master.gain.value = v;
		this.#persist();
	}
	setChord(id: string) {
		this.chordId = id;
		this.#persist();
	}
	setRing(id: string) {
		this.ringId = id;
		this.#persist();
	}

	// A soft tubular-chime hit: a fundamental that rings out slowly plus a quieter
	// inharmonic partial (≈2.76×, a real wind chime's ratio) that shimmers and fades
	// fast. Soft attack, long tail. `seed` picks a scale note so nodes differ.
	play(seed: number, season?: SeasonSound) {
		if (!this.on || !this.#actx || !this.#master) return;
		// While suspended the audio clock is frozen, so anything scheduled now
		// would all fire at once when it resumes. Drop chimes until it runs.
		if (this.#actx.state !== 'running') return;
		const wall = performance.now();
		if (season?.ho && wall - this.#lastHoAt >= HO_GAP_MS) {
			this.#lastHoAt = wall;
			this.#ho(this.#actx, this.#master, this.#previewing);
		}
		if (season?.tune === 'jingle') {
			if (wall - this.#lastTickAt < 150) return; // keep the tune legible
			this.#lastTickAt = wall;
			if (wall - this.#tuneLast > 8000) this.#tuneAt = 0; // quiet for a while: start from the top
			this.#tuneLast = wall;
			this.#bell(this.#actx, this.#master, JINGLE[this.#tuneAt]);
			this.#tuneAt = (this.#tuneAt + 1) % JINGLE.length;
			return;
		}
		if (wall - this.#lastTickAt < 120) return; // sparse, gentle trickle — chimes don't rush
		this.#lastTickAt = wall;
		const t = this.#actx.currentTime;
		const base = this.scale[Math.abs(seed) % this.scale.length];
		const partials = [
			{ ratio: 1, peak: 0.08, decay: this.ringDecay },
			{ ratio: 2.76, peak: 0.026, decay: this.ringDecay * 0.5 }
		];
		for (const p of partials) {
			const osc = this.#actx.createOscillator();
			const g = this.#actx.createGain();
			osc.type = 'sine';
			osc.frequency.value = base * p.ratio;
			osc.detune.value = (Math.random() - 0.5) * 12; // tiny drift, never mechanical
			g.gain.setValueAtTime(0, t);
			g.gain.linearRampToValueAtTime(p.peak, t + 0.012); // soft attack, no click
			g.gain.exponentialRampToValueAtTime(0.0001, t + p.decay);
			osc.connect(g).connect(this.#master);
			osc.start(t);
			osc.stop(t + p.decay + 0.05);
		}
	}

	// A bright handbell note with a little sleigh-bell shake on top: a short
	// fundamental, two inharmonic partials, then three tiny high noise "jingles".
	#bell(ctx: AudioContext, out: AudioNode, freq: number) {
		const t = ctx.currentTime;
		for (const p of [
			// Ring sets how long the bell sounds: Short 0.5 s, Med 0.9 s, Long 1.6 s.
			{ ratio: 1, peak: 0.07, decay: this.ringDecay * 0.5 },
			{ ratio: 2.76, peak: 0.022, decay: Math.min(0.5, this.ringDecay * 0.2) },
			{ ratio: 5.4, peak: 0.01, decay: 0.15 }
		]) {
			const osc = ctx.createOscillator();
			const g = ctx.createGain();
			osc.type = 'sine';
			osc.frequency.value = freq * p.ratio;
			g.gain.setValueAtTime(0, t);
			g.gain.linearRampToValueAtTime(p.peak, t + 0.005);
			g.gain.exponentialRampToValueAtTime(0.0001, t + p.decay);
			osc.connect(g).connect(out);
			osc.start(t);
			osc.stop(t + p.decay + 0.05);
		}
		const noise = this.#noise(ctx);
		for (let i = 0; i < 3; i++) {
			const at = t + i * 0.035 + Math.random() * 0.01;
			const src = ctx.createBufferSource();
			src.buffer = noise;
			const bp = ctx.createBiquadFilter();
			bp.type = 'bandpass';
			bp.frequency.value = 6000 + Math.random() * 2500;
			bp.Q.value = 8;
			const g = ctx.createGain();
			g.gain.setValueAtTime(0, at);
			g.gain.linearRampToValueAtTime(0.05, at + 0.003);
			g.gain.exponentialRampToValueAtTime(0.0001, at + 0.09);
			src.connect(bp).connect(g).connect(out);
			src.start(at);
			src.stop(at + 0.1);
		}
	}

	// Santa's "ho ho ho" (static/seasonal/hohoho.mp3, a recorded clip), fetched and decoded the first time it's wanted. If it
	// isn't ready yet it's skipped this time — or, for a preview, played as
	// soon as it arrives.
	#hoBuf: AudioBuffer | null = null;
	#hoLoad: Promise<AudioBuffer | null> | null = null;

	#loadHo(ctx: AudioContext): Promise<AudioBuffer | null> {
		this.#hoLoad ??= fetch('/seasonal/hohoho.mp3')
			.then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(String(r.status)))))
			.then((data) => ctx.decodeAudioData(data))
			.then((buf) => (this.#hoBuf = buf))
			.catch(() => {
				this.#hoLoad = null; // try again next time
				return null;
			});
		return this.#hoLoad;
	}

	#ho(ctx: AudioContext, out: AudioNode, waitForLoad = false) {
		const start = (buf: AudioBuffer) => {
			const src = ctx.createBufferSource();
			src.buffer = buf;
			const g = ctx.createGain();
			g.gain.value = 0.35; // the clip peaks at -3 dB; this sits it just above the bells
			src.connect(g).connect(out);
			src.start();
		};
		if (this.#hoBuf) return start(this.#hoBuf);
		const loading = this.#loadHo(ctx);
		if (waitForLoad) void loading.then((buf) => buf && start(buf));
	}

	#noiseBuf: AudioBuffer | null = null;
	#noise(ctx: AudioContext): AudioBuffer {
		if (!this.#noiseBuf || this.#noiseBuf.sampleRate !== ctx.sampleRate) {
			const b = ctx.createBuffer(1, Math.floor(ctx.sampleRate * 0.2), ctx.sampleRate);
			const d = b.getChannelData(0);
			for (let i = 0; i < d.length; i++) d[i] = Math.random() * 2 - 1;
			this.#noiseBuf = b;
		}
		return this.#noiseBuf;
	}

	// Play a sample chime immediately (bypasses the trickle throttle) so a tuning
	// change is audible right away.
	preview(season?: SeasonSound) {
		if (!this.on) return;
		this.ensure();
		this.#lastTickAt = -Infinity;
		this.#lastHoAt = -Infinity;
		this.#previewing = true;
		this.play(Math.floor(Math.random() * this.scale.length), season);
		this.#previewing = false;
	}
}

export const chime = new LiveChime();
