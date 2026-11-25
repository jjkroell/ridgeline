// Shareable channel links.
//
//   /channels?channel=test800            a hashtag channel (key derives from the name)
//   /channels?channel=public             the built-in Public channel
//   /channels#name=Family&key=<32 hex>   a private channel
//
// A private channel's key travels in the URL fragment, which browsers never
// send to a server (and analytics.ts tells Umami to leave fragments out).
// Opening a link to a channel this browser doesn't have asks before adding it.
import { goto } from '$app/navigation';
import { untrack } from 'svelte';
import { channels, PUBLIC_KEY_HEX, SOS_ID, type Channel } from './channels.svelte';
import { deriveHashtagKey } from './channel-crypto';
import { copyText } from './map-share';
import { confirmer } from './confirm.svelte';
import { consent } from './consent.svelte';
import { announce } from './announce.svelte';
import { isSearchCrawler } from './crawler';
import { hud } from './hud.svelte';
import { ICONS, type MenuSpec } from './mobile/context-menu.svelte';

type Target = Pick<Channel, 'name' | 'type' | 'keyHex'>;

const MAX_NAME = 64;

// The name in a link is written by whoever made it: no control or invisible
// formatting characters, and a sane length.
function cleanName(raw: string): string {
	const n = raw.trim().replace(/^#+/, '').trim();
	return n.length > MAX_NAME || /\p{C}/u.test(n) ? '' : n;
}

export function channelUrl(c: Target): string {
	const url = new URL('/channels', location.origin);
	if (c.type === 'private') {
		url.hash = new URLSearchParams({ name: c.name, key: c.keyHex }).toString();
	} else if (c.type === 'public') {
		url.searchParams.set('channel', 'public');
	} else {
		const name = c.name.replace(/^#/, '');
		// '#public' is a hashtag channel of its own, not the built-in Public.
		url.searchParams.set('channel', name.toLowerCase() === 'public' ? '#' + name : name);
	}
	return url.toString();
}

/** The channel a URL links to; null when it isn't a channel link, 'bad' when it is a broken one. */
export function parseChannelUrl(url: URL): Target | 'bad' | null {
	const hash = new URLSearchParams(url.hash.slice(1));
	if (hash.has('key')) {
		const name = cleanName(hash.get('name') ?? '');
		const keyHex = (hash.get('key') ?? '').toLowerCase();
		return name && /^[0-9a-f]{32}$/.test(keyHex) ? { name, type: 'private', keyHex } : 'bad';
	}
	const raw = url.searchParams.get('channel');
	if (!raw) return null;
	if (raw.toLowerCase() === 'public') return { name: 'Public', type: 'public', keyHex: PUBLIC_KEY_HEX };
	const name = cleanName(raw);
	return name ? { name: '#' + name, type: 'hashtag', keyHex: deriveHashtagKey(name) } : 'bad';
}

/** Copy a channel's link, first pointing out that a private one contains its key. */
export async function copyChannelLink(c: Channel) {
	if (
		c.type === 'private' &&
		!(await confirmer.ask({
			title: `Copy a link to ${c.name}?`,
			message:
				'This is a private channel, so the link contains its key. Anyone who gets the link can read the channel.',
			confirmLabel: 'Copy link'
		}))
	)
		return;
	if (await copyText(channelUrl(c))) hud.show('Link copied');
	else hud.show('Your browser blocked copying', 'error');
}

/** Press-and-hold / right-click actions for a channel row. */
export function channelMenu(c: Channel, open: (id: string) => void): MenuSpec {
	return {
		title: c.name,
		items: [
			{ label: 'Open', icon: ICONS.open, run: () => open(c.id) },
			{ label: 'Copy link', icon: ICONS.copy, run: () => copyChannelLink(c) }
		]
	};
}

const find = (t: Target) => channels.list.find((c) => c.keyHex === t.keyHex);

// Ask, then add. Resolves to the new channel's id, or null if it wasn't added.
async function confirmAdd(t: Target): Promise<string | null> {
	const ok = await confirmer.ask({
		title: `Add ${t.name} to your channels?`,
		message:
			t.type === 'private'
				? 'You opened a link to a private channel you don’t have in this browser. The link contains its key. The channel is saved in this browser only, and you can remove it at any time.'
				: 'You opened a link to a channel you don’t have in this browser. It is saved in this browser only, and you can remove it at any time.',
		confirmLabel: 'Add channel',
		cancelLabel: 'Not now'
	});
	if (!ok) return null;
	let err: string | null = null;
	if (!find(t)) {
		if (t.type === 'public') channels.restorePublic();
		else if (t.type === 'hashtag') err = channels.addHashtag(t.name);
		else err = channels.addPrivate(t.name, t.keyHex);
	}
	const added = find(t);
	if (added) return added.id;
	// Only a name differing in case gets here: a separate channel on the mesh.
	await confirmer.tell({
		title: `${t.name} wasn’t added`,
		message: err ?? 'This channel could not be added.'
	});
	return null;
}

/**
 * Follow a channel link in `url`; call from an $effect that reads page.url.
 * `select` opens a channel by id. Returns the effect's cleanup.
 */
export function followChannelUrl(url: URL, select: (id: string) => void): (() => void) | void {
	const t = parseChannelUrl(url);
	if (!t || isSearchCrawler()) return;
	// Let the page's own mount (channels.init, What's new opening) finish first.
	const known = t !== 'bad' && untrack(() => (channels.init(), find(t)));
	// A first-time visitor gets What's new and, on a small screen, a cookie banner
	// over the dialog's buttons; ask once those are out of the way. (Reading them
	// here re-runs the effect when they close.)
	if (!known && (announce.open || (consent.open && matchMedia('(max-width: 767px)').matches))) return;
	const timer = setTimeout(async () => {
		// Take the link out of the address bar, so a reload doesn't ask again and
		// a key doesn't linger there. #sos can't be removed, so its link may stay.
		if (!known || known.id !== SOS_ID)
			goto(url.pathname, { replaceState: true, keepFocus: true, noScroll: true });
		if (t === 'bad') {
			hud.show('That channel link isn’t valid', 'error');
			return;
		}
		const id = known ? known.id : await confirmAdd(t);
		if (id) select(id);
	}, 0);
	return () => clearTimeout(timer);
}
