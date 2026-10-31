// How fast packets travel across the live maps (Map Control's Speed slider, or
// the speed button on a phone). A multiplier on the pulse duration: below 1
// the comets crawl so each hop is easy to follow, above 1 they race. Shared by
// /live-map, /m/live-map and the WebGL-free fallback; remembered per browser.
const KEY = 'ridgeline-livemap-speed';

export const SPEEDS = [0.25, 0.5, 0.75, 1, 1.5, 2, 3];

export function speedLabel(s: number): string {
	return (s === 0.25 ? '¼' : s === 0.5 ? '½' : s === 0.75 ? '¾' : String(s)) + '×';
}

class LiveSpeed {
	value = $state(1);
	#loaded = false;

	load() {
		if (this.#loaded || typeof localStorage === 'undefined') return;
		this.#loaded = true;
		try {
			const v = Number(localStorage.getItem(KEY));
			if (SPEEDS.includes(v)) this.value = v;
		} catch {
			/* storage blocked: normal speed */
		}
	}

	set(v: number) {
		this.value = v;
		try {
			localStorage.setItem(KEY, String(v));
		} catch {
			/* not remembered, still applies */
		}
	}

	/** Next preset, wrapping round (the phone's tap-to-cycle button). */
	cycle() {
		const i = SPEEDS.indexOf(this.value);
		this.set(SPEEDS[(i + 1) % SPEEDS.length]);
	}
}

export const liveSpeed = new LiveSpeed();
