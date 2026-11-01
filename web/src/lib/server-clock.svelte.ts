// What the browser knows about the server: how far the viewer's clock is from
// it, and whether the data on screen is live or a saved copy.
//
// Clock: every "2m ago" and every Reporting/Silent used to compare a server
// timestamp with Date.now() — the VIEWER's clock. A device an hour fast (a
// missed DST change, a wrong time zone with the hours set by hand) saw every
// observer as last heard "1h" ago. Each API response carries the server's
// Date header, so the skew is measured and serverNow() corrects for it.
// Offsets under SKEW_IGNORE_MS are ignored: Date has one-second resolution and
// a correct clock should read exactly as before.
//
// Offline: the service worker answers /api/ from its cache when the network
// fails, and marks the answer with X-Ridgeline-Cached. Without a flag the page
// shows that copy as if it were live — every observer ageing in lock-step with
// nothing to say why. OfflineBanner reads `offline` and says so.

const SKEW_IGNORE_MS = 30_000;

class ServerClock {
	/** Milliseconds to add to Date.now() to get the server's time. */
	offset = 0;
	/** True while API reads are failing or being answered from the offline cache. */
	offline = $state(false);
	/** When the saved copy on screen was fetched (ms, server time), if known. */
	savedAt = $state<number | null>(null);

	/** Learn from one API response. `sentAt` is Date.now() when the request left. */
	note(res: Response, sentAt: number) {
		const cached = res.headers.get('x-ridgeline-cached');
		if (cached !== null || res.headers.get('x-ridgeline-offline')) {
			this.#offline(cached ? Date.parse(cached) : NaN);
			return;
		}
		this.offline = false;
		this.savedAt = null;
		const date = Date.parse(res.headers.get('date') ?? '');
		if (Number.isNaN(date)) return;
		// Date is floored to the second; +500 ms centres it. Compare against the
		// midpoint of the round trip, when the server most likely stamped it.
		const raw = date + 500 - (sentAt + Date.now()) / 2;
		this.offset = Math.abs(raw) < SKEW_IGNORE_MS ? 0 : raw;
	}

	/** An API read failed outright (no service worker, or it had nothing cached). */
	failed() {
		this.#offline(NaN);
	}

	#offline(at: number) {
		this.offline = true;
		if (!Number.isNaN(at)) this.savedAt = at;
	}
}

export const serverClock = new ServerClock();

/** The server's current time in ms — use instead of Date.now() whenever the
 *  result is compared with a timestamp the server produced. */
export function serverNow(): number {
	return Date.now() + serverClock.offset;
}
