import { isSearchCrawler } from './crawler';

// Announcement / "what's new" modal state. Auto-shows once per release the first
// time a visitor lands (tracked in localStorage by version), and can be reopened
// on demand from the overview page. Bump CURRENT when there's something new to
// announce and every visitor sees it once again (RELEASE_OFF / RELEASE_ON below). A seasonal theme starting
// (by its dates or the admin switch) also opens it once per season — see
// season() — so a new season needs no bump.
const STORAGE_KEY = 'ridgeline-announce-seen';
const SEASON_KEY = 'ridgeline-announce-season-seen';
// Which note is current follows the admin's emergency-channel switch: off,
// "Ridgeline no longer monitors #sos"; on, "#mayday is the emergency channel".
// Turning it on later shows the #mayday note to everyone once.
export const RELEASE_OFF = '2026-11-sos-off';
export const RELEASE_ON = '2026-11-mayday-on';

class Announce {
	open = $state(false);
	/** The release note in force (RELEASE_OFF / RELEASE_ON); set by init. */
	current = $state(RELEASE_OFF);
	#season = '';

	/** Called once /api/health has answered (with the emergency switch): opens
	 *  the modal if this release hasn't been seen. */
	init(emergency: boolean) {
		this.current = emergency ? RELEASE_ON : RELEASE_OFF;
		if (isSearchCrawler()) return;
		let seen = '';
		try {
			seen = localStorage.getItem(STORAGE_KEY) ?? '';
		} catch {
			return; // storage blocked — skip the auto-show rather than nag every load
		}
		if (seen !== this.current) this.open = true;
	}

	/** A season is in force: open once for it, if this browser hasn't seen it. */
	season(id: string) {
		this.#season = id;
		if (isSearchCrawler()) return;
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
			localStorage.setItem(STORAGE_KEY, this.current);
			if (this.#season) localStorage.setItem(SEASON_KEY, this.#season);
		} catch {
			/* ignore */
		}
	}
}

export const announce = new Announce();
