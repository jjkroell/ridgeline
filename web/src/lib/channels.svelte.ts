// User's group channels, persisted to localStorage. Channels live only in the
// browser — keys never go to the server; GroupText is decrypted client-side
// (see channel-crypto.ts). The well-known Public channel is added by default
// but can be removed like any other. #sos, the mesh's emergency channel, is
// permanent: every browser carries it and it cannot be removed.
import { deriveHashtagKey, channelHashByte, decryptGroupText } from './channel-crypto';

export type ChannelType = 'public' | 'hashtag' | 'private';

/** Message sort order in the reader: 'asc' = newest at bottom, 'desc' = newest at top. */
export type SortDir = 'asc' | 'desc';

export interface Channel {
	id: string;
	name: string; // display name (hashtag channels include the leading '#')
	type: ChannelType;
	keyHex: string; // 16-byte AES key, 32 lowercase hex chars
	hashByte: string; // 1-byte channel id, 2 uppercase hex chars (derived from key)
}

export interface DecryptedMessage {
	channel: string;
	sender: string;
	text: string;
}

const STORAGE_KEY = 'ridgeline-channels';
const SORT_KEY = 'ridgeline-channel-sort';
const PUBLIC_KEY_HEX = '8b3387e9c5cdea6ac9e5edbaa115cd72';

function publicChannel(): Channel {
	return {
		id: 'public',
		name: 'Public',
		type: 'public',
		keyHex: PUBLIC_KEY_HEX,
		hashByte: channelHashByte(PUBLIC_KEY_HEX)
	};
}

/** id of the permanent #sos channel. */
export const SOS_ID = 'sos';

function sosChannel(): Channel {
	const keyHex = deriveHashtagKey('sos');
	return { id: SOS_ID, name: '#sos', type: 'hashtag', keyHex, hashByte: channelHashByte(keyHex) };
}

/** A channel the user cannot remove. */
export function isPermanent(c: Pick<Channel, 'id'> | null | undefined): boolean {
	return c?.id === SOS_ID;
}

// Put #sos in a saved list that lacks it — right after Public when that is
// there, else first — replacing a '#sos' the user added by hand before it was
// built in (same key), so it never shows twice.
function withSOS(list: Channel[]): Channel[] {
	const sos = sosChannel();
	const rest = list.filter((c) => c.id !== SOS_ID && c.keyHex !== sos.keyHex);
	const at = rest[0]?.type === 'public' ? 1 : 0;
	return [...rest.slice(0, at), sos, ...rest.slice(at)];
}

function rid(): string {
	try {
		return crypto.randomUUID();
	} catch {
		return 'ch-' + Math.random().toString(36).slice(2) + Date.now().toString(36);
	}
}

// Drop malformed persisted entries and recompute the hash byte so it always
// reflects the stored key (e.g. after a crypto change).
function normalize(c: Partial<Channel>): Channel | null {
	if (!c || typeof c.keyHex !== 'string' || !/^[0-9a-f]{32}$/.test(c.keyHex.toLowerCase())) {
		return null;
	}
	const keyHex = c.keyHex.toLowerCase();
	const type: ChannelType =
		c.type === 'public' || c.type === 'hashtag' || c.type === 'private' ? c.type : 'private';
	return {
		id: typeof c.id === 'string' && c.id ? c.id : rid(),
		name: typeof c.name === 'string' && c.name ? c.name : 'Channel',
		type,
		keyHex,
		hashByte: channelHashByte(keyHex)
	};
}

class Channels {
	list = $state<Channel[]>([publicChannel(), sosChannel()]);
	// Per-channel reader sort direction, keyed by channel id.
	sortById = $state<Record<string, SortDir>>({});
	#loaded = false;

	/** Load from localStorage. Safe to call repeatedly; only reads once. */
	init() {
		if (this.#loaded) return;
		this.#loaded = true;
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			if (raw) {
				const parsed = JSON.parse(raw);
				if (Array.isArray(parsed)) {
					const saved = parsed.map(normalize).filter((c): c is Channel => c !== null);
					this.list = withSOS(saved);
					if (!saved.some((c) => c.id === SOS_ID)) this.#persist();
				}
			}
		} catch {
			/* storage unavailable or corrupt — keep the default */
		}
		try {
			const rawSort = localStorage.getItem(SORT_KEY);
			if (rawSort) {
				const parsed = JSON.parse(rawSort);
				if (parsed && typeof parsed === 'object') this.sortById = parsed;
			}
		} catch {
			/* keep the default sort */
		}
	}

	#persist() {
		try {
			localStorage.setItem(STORAGE_KEY, JSON.stringify(this.list));
		} catch {
			/* storage unavailable */
		}
	}

	#persistSort() {
		try {
			localStorage.setItem(SORT_KEY, JSON.stringify(this.sortById));
		} catch {
			/* storage unavailable */
		}
	}

	/** Reader sort for a channel; defaults to 'asc' (newest at bottom, chat-style). */
	getSort(id: string | null): SortDir {
		return id && this.sortById[id] === 'desc' ? 'desc' : 'asc';
	}

	setSort(id: string, dir: SortDir) {
		this.sortById = { ...this.sortById, [id]: dir };
		this.#persistSort();
	}

	/** Flip a channel's sort and return the new direction. */
	toggleSort(id: string): SortDir {
		const next: SortDir = this.getSort(id) === 'asc' ? 'desc' : 'asc';
		this.setSort(id, next);
		return next;
	}

	get hasPublic(): boolean {
		return this.list.some((c) => c.type === 'public');
	}

	/** Add a public hashtag channel by name; key is derived. Returns an error string or null. */
	addHashtag(name: string): string | null {
		const clean = name.trim().replace(/^#+/, '').trim();
		if (!clean) return 'Enter a channel name.';
		const display = '#' + clean;
		if (this.list.some((c) => c.name.toLowerCase() === display.toLowerCase())) {
			return 'That channel is already added.';
		}
		const keyHex = deriveHashtagKey(clean);
		this.list = [
			...this.list,
			{ id: rid(), name: display, type: 'hashtag', keyHex, hashByte: channelHashByte(keyHex) }
		];
		this.#persist();
		return null;
	}

	/** Add a private channel with a user-supplied 16-byte key (32 hex). Returns an error string or null. */
	addPrivate(name: string, keyInput: string): string | null {
		const n = name.trim();
		if (!n) return 'Enter a channel name.';
		const hex = keyInput.trim().replace(/[\s:]/g, '').toLowerCase();
		if (!/^[0-9a-f]{32}$/.test(hex)) return 'Key must be 32 hex characters (16 bytes).';
		if (this.list.some((c) => c.keyHex === hex)) return 'A channel with this key already exists.';
		this.list = [
			...this.list,
			{ id: rid(), name: n, type: 'private', keyHex: hex, hashByte: channelHashByte(hex) }
		];
		this.#persist();
		return null;
	}

	remove(id: string) {
		if (id === SOS_ID) return; // permanent
		this.list = this.list.filter((c) => c.id !== id);
		this.#persist();
		if (id in this.sortById) {
			const { [id]: _, ...rest } = this.sortById;
			this.sortById = rest;
			this.#persistSort();
		}
	}

	/** Re-add the default Public channel if it was removed. */
	restorePublic() {
		if (this.hasPublic) return;
		this.list = [publicChannel(), ...this.list];
		this.#persist();
	}

	/**
	 * Decrypt a GroupText payload (hex) against the configured channels. Tries
	 * every channel whose hash byte matches the payload; the HMAC check picks
	 * the right one. Returns null when no configured channel decodes it.
	 */
	decrypt(payloadHex?: string): DecryptedMessage | null {
		if (!payloadHex || payloadHex.length < 2) return null;
		const hashByte = payloadHex.slice(0, 2).toUpperCase();
		for (const c of this.list) {
			if (c.hashByte !== hashByte) continue;
			const d = decryptGroupText(payloadHex, c.keyHex);
			if (d) return { channel: c.name, sender: d.sender, text: d.text };
		}
		return null;
	}
}

export const channels = new Channels();
