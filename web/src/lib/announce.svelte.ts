// Announcement / "what's new" modal state. Auto-shows once per release the first
// time a visitor lands (tracked in localStorage by version), and can be reopened
// on demand from the overview page. Bump CURRENT when there's something new to
// announce and every visitor sees it once again. A seasonal theme starting
// (by its dates or the admin switch) also opens it once per season — see
// season() — so a new season needs no bump.
const STORAGE_KEY = 'ridgeline-announce-seen';
const SEASON_KEY = 'ridgeline-announce-season-seen';
export const CURRENT = '2026-10-halloween-live-map';

class Announce {
	open = $state(false);
	#season = '';

	/** Called once on app load: opens the modal if this release hasn't been seen. */
	init() {
		let seen = '';
		try {
			seen = localStorage.getItem(STORAGE_KEY) ?? '';
		} catch {
			return; // storage blocked — skip the auto-show rather than nag every load
		}
		if (seen !== CURRENT) this.open = true;
	}

	/** A season is in force: open once for it, if this browser hasn't seen it. */
	season(id: string) {
		this.#season = id;
		try {
			let seen = localStorage.getItem(SEASON_KEY);
			// Halloween was announced through CURRENT before seasons had their own key.
			if (seen === null && localStorage.getItem(STORAGE_KEY) === '2026-10-halloween-live-map') {
				seen = 'halloween';
				localStorage.setItem(SEASON_KEY, seen);
			}
			if (seen !== id) this.open = true;
		} catch {
			/* storage blocked — skip the auto-show */
		}
	}

	/** Reopen on demand (e.g. the overview "What's new" button). */
	show() {
		this.open = true;
	}

	/** Close and remember this release as seen so it won't auto-open again. */
	close() {
		this.open = false;
		try {
			localStorage.setItem(STORAGE_KEY, CURRENT);
			if (this.#season) localStorage.setItem(SEASON_KEY, this.#season);
		} catch {
			/* ignore */
		}
	}
}

export const announce = new Announce();
