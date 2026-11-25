import { page } from '$app/state';
import { replaceState } from '$app/navigation';
import { addressBarUrl } from '$lib/address-bar';
import { channels } from '$lib/channels.svelte';
import type { LiveGroup } from '$lib/live.svelte';
import type { Node } from '$lib/api';

// Feed page search + type filter, shared by the desktop (/live) and phone
// (/m/live) feeds. Works on what the browser already holds: up to an hour of
// the live stream, but in practice ~10 minutes (the server backfills 3,000
// reports and the buffer keeps 5,000), so it needs nothing from the server.

/** Type chips. A packet type not listed in any chip falls under "Other". */
export const FEED_TYPES = [
	{ id: 'advert', label: 'Adverts', types: ['Advert'] },
	{ id: 'channel', label: 'Channel', types: ['GroupText', 'GroupData'] },
	{ id: 'dm', label: 'DMs', types: ['TextMessage'] },
	{ id: 'ack', label: 'Acks', types: ['Ack'] },
	{ id: 'path', label: 'Paths', types: ['Path'] },
	{ id: 'trace', label: 'Traces', types: ['Trace'] },
	{ id: 'request', label: 'Req/Resp', types: ['Request', 'Response', 'AnonRequest'] },
	{ id: 'control', label: 'Control', types: ['Control'] },
	{ id: 'other', label: 'Other', types: [] as string[] }
] as const;

const KNOWN = new Set<string>(FEED_TYPES.flatMap((t) => t.types));

/** Which chip a payload type belongs to. */
export function feedTypeOf(payloadType: string): string {
	return FEED_TYPES.find((t) => (t.types as readonly string[]).includes(payloadType))?.id ?? 'other';
}

/** Search words: all must match; a leading "-" excludes. Case-insensitive. */
export function parse(q: string) {
	const inc: string[] = [];
	const exc: string[] = [];
	for (const w of q.toLowerCase().split(/\s+/).filter(Boolean)) {
		if (w.startsWith('-') && w.length > 1) exc.push(w.slice(1));
		else if (w !== '-') inc.push(w);
	}
	return { inc, exc };
}

/** Everything a person might search a packet by, lower-cased. `extra` carries
 *  page-specific text (the desktop's Contents summary, resolved hop names). */
export function groupHaystack(g: LiveGroup, extra: (string | undefined)[] = []): string {
	const parts: (string | undefined)[] = [
		g.payloadType,
		g.routeType,
		g.messageHash,
		g.node?.name,
		g.node?.publicKey,
		g.node?.role,
		...extra
	];
	const seen = new Set<string>();
	for (const e of g.events) {
		parts.push(e.observerName, e.observerId, e.channel, e.sender, e.text);
		if (e.channelHash) parts.push('ch ' + e.channelHash);
		for (const h of e.path ?? []) seen.add(h);
		if (e.payloadType === 'GroupText' && !e.text && e.payloadRaw) {
			const dec = channels.decrypt(e.payloadRaw);
			if (dec) parts.push(dec.channel, dec.sender, dec.text);
		}
	}
	parts.push(...seen);
	return parts.filter(Boolean).join(' \u0001 ').toLowerCase();
}

export class FeedFilter {
	q = $state('');
	types = $state<string[]>([]);

	/** Read ?q= and ?t= from the address bar (shareable filtered views). */
	load() {
		const sp = addressBarUrl(page.url).searchParams;
		this.q = sp.get('q') ?? '';
		const ids = new Set<string>(FEED_TYPES.map((t) => t.id));
		this.types = (sp.get('t') ?? '').split(',').filter((t) => ids.has(t));
	}

	#save() {
		const url = addressBarUrl(page.url);
		const q = this.q.trim();
		if (q) url.searchParams.set('q', q);
		else url.searchParams.delete('q');
		if (this.types.length) url.searchParams.set('t', this.types.join(','));
		else url.searchParams.delete('t');
		try {
			replaceState(url, {});
		} catch {
			/* router not ready yet; the next change will write it */
		}
	}

	setQuery(q: string) {
		this.q = q;
		this.#save();
	}
	toggle(id: string) {
		this.types = this.types.includes(id) ? this.types.filter((t) => t !== id) : [...this.types, id];
		this.#save();
	}
	clear() {
		this.q = '';
		this.types = [];
		this.#save();
	}

	get active() {
		return this.q.trim() !== '' || this.types.length > 0;
	}

	/** Apply type chips then search words. `extra` = page-specific search text. */
	apply(groups: LiveGroup[], extra?: (g: LiveGroup) => (string | undefined)[]): LiveGroup[] {
		let out = groups;
		if (this.types.length) {
			const want = new Set(this.types);
			out = out.filter((g) => want.has(feedTypeOf(g.payloadType)));
		}
		const { inc, exc } = parse(this.q);
		if (inc.length || exc.length) {
			out = out.filter((g) => {
				const hay = groupHaystack(g, extra?.(g));
				return inc.every((w) => hay.includes(w)) && !exc.some((w) => hay.includes(w));
			});
		}
		return out;
	}

	/** Per-chip counts over `groups` (before the type filter, after nothing). */
	counts(groups: LiveGroup[]): Record<string, number> {
		const c: Record<string, number> = {};
		for (const g of groups) {
			const id = KNOWN.has(g.payloadType) ? feedTypeOf(g.payloadType) : 'other';
			c[id] = (c[id] ?? 0) + 1;
		}
		return c;
	}
}

/** Known nodes whose name (or key prefix) matches every search word: shown
 *  above the results so a node that hasn't been heard in the feed's short
 *  window (e.g. "texada repeater") still leads somewhere. */
export function matchingNodes(nodes: Node[], q: string, max = 3): Node[] {
	const { inc } = parse(q);
	if (!inc.length) return [];
	const heard = (n: Node) => +new Date(n.lastHeard ?? n.lastSeen ?? 0) || 0;
	return nodes
		.filter((n) => {
			const name = (n.name ?? '').toLowerCase();
			const key = n.publicKey.toLowerCase();
			return inc.every((w) => name.includes(w) || key.startsWith(w));
		})
		.sort((a, b) => heard(b) - heard(a))
		.slice(0, max);
}

/** "the last 9 minutes" for the oldest packet the page holds. */
export function spanLabel(oldest: string | undefined, now: number): string {
	if (!oldest) return 'the last hour';
	const min = Math.max(1, Math.round((now - +new Date(oldest)) / 60000));
	if (min >= 55) return 'the last hour';
	return min === 1 ? 'the last minute' : `the last ${min} minutes`;
}
