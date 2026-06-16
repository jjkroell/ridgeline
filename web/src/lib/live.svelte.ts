// Reactive WebSocket connection to /api/live. Exposes a rolling buffer of the
// most recent live events plus connection state, as Svelte 5 runes.
import type { LiveEvent } from './api';

const MAX_EVENTS = 200;

class LiveFeed {
	events = $state<LiveEvent[]>([]);
	connected = $state(false);
	total = $state(0);

	#ws: WebSocket | null = null;
	#retry = 0;
	#timer: ReturnType<typeof setTimeout> | null = null;
	#started = false;

	start() {
		if (this.#started) return;
		this.#started = true;
		this.#connect();
	}

	#connect() {
		const proto = location.protocol === 'https:' ? 'wss' : 'ws';
		const ws = new WebSocket(`${proto}://${location.host}/api/live`);
		this.#ws = ws;

		ws.onopen = () => {
			this.connected = true;
			this.#retry = 0;
		};
		ws.onmessage = (e) => {
			try {
				const ev = JSON.parse(e.data) as LiveEvent;
				this.events = [ev, ...this.events].slice(0, MAX_EVENTS);
				this.total += 1;
			} catch {
				/* ignore malformed frames */
			}
		};
		ws.onclose = () => {
			this.connected = false;
			this.#scheduleReconnect();
		};
		ws.onerror = () => ws.close();
	}

	#scheduleReconnect() {
		if (this.#timer) return;
		const delay = Math.min(1000 * 2 ** this.#retry, 15000);
		this.#retry += 1;
		this.#timer = setTimeout(() => {
			this.#timer = null;
			this.#connect();
		}, delay);
	}
}

export const live = new LiveFeed();
