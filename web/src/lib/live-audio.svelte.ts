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
	#lastTickAt = 0;
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
	play(seed: number) {
		if (!this.on || !this.#actx || !this.#master) return;
		// While suspended the audio clock is frozen, so anything scheduled now
		// would all fire at once when it resumes. Drop chimes until it runs.
		if (this.#actx.state !== 'running') return;
		const wall = performance.now();
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

	// Play a sample chime immediately (bypasses the trickle throttle) so a tuning
	// change is audible right away.
	preview() {
		if (!this.on) return;
		this.ensure();
		this.#lastTickAt = 0;
		this.play(Math.floor(Math.random() * this.scale.length));
	}
}

export const chime = new LiveChime();
