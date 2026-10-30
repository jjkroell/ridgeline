// iOS-style context menu: press and hold a row in the phone app (or right-
// click a row on desktop) to get its actions without opening it. One menu at
// a time, rendered once by ContextMenu.svelte in the root layout.
import { goto } from '$app/navigation';
import { favorites } from '$lib/favorites.svelte';
import { hud } from '$lib/hud.svelte';
import { shortKey } from '$lib/format';
import type { Node, Observer } from '$lib/api';

export interface MenuItem {
	label: string;
	/** SVG path data on a 24×24 grid, drawn as a stroke. */
	icon: string;
	run: () => void | Promise<void>;
}
export interface MenuSpec {
	title: string;
	subtitle?: string;
	items: MenuItem[];
}
interface Open extends MenuSpec {
	rect: DOMRect;
	/** Set for a right-click: a plain menu at the pointer, no lifted row. */
	at?: { x: number; y: number };
}

class ContextMenuStore {
	current = $state<Open | null>(null);

	open(spec: MenuSpec, el: Element, at?: { x: number; y: number }) {
		this.current = { ...spec, rect: el.getBoundingClientRect(), at };
	}
	close() {
		this.current = null;
	}
}
export const contextMenu = new ContextMenuStore();

export const ICONS = {
	open: 'M9 6l6 6-6 6',
	star: 'M12 2.5l2.9 5.9 6.5.95-4.7 4.6 1.1 6.45L12 17.9l-5.8 3.05 1.1-6.45-4.7-4.6 6.5-.95z',
	map: 'M12 21s7-6.6 7-12a7 7 0 1 0-14 0c0 5.4 7 12 7 12zM12 9.2a2.6 2.6 0 1 0 0 5.2 2.6 2.6 0 0 0 0-5.2z',
	copy: 'M9 9h10v12H9zM5 15H4V3h11v1',
	share: 'M12 15V3m0 0L8 7m4-4 4 4M5 11v9a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-9'
};

export async function copyText(text: string, what: string) {
	try {
		await navigator.clipboard.writeText(text);
		hud.show(`${what} copied`);
	} catch {
		hud.show('Your browser blocked copying', 'error');
	}
}

/** The system share sheet where there is one (iPhone, Android); else copy the link. */
export async function shareLink(title: string, path: string) {
	const url = location.origin + path;
	if (navigator.share) {
		try {
			await navigator.share({ title, url });
		} catch {
			/* dismissed */
		}
	} else await copyText(url, 'Link');
}

export const openItem = (href: string): MenuItem => ({ label: 'Open', icon: ICONS.open, run: () => goto(href) });

/** Star or unstar a node, with the pop-up — the row's star can be off screen. */
export function toggleStar(pubkey: string) {
	favorites.toggle(pubkey);
	const on = favorites.has(pubkey);
	hud.show(on ? 'Starred' : 'Unstarred', on ? 'star' : 'unstar');
}

/**
 * Right-click only, for desktop rows: the menu opens at the pointer. A
 * right-click with a modifier key still gets the browser's own menu, so
 * "Open link in new tab" stays one gesture away.
 */
export function rightclick(el: HTMLElement, get: () => MenuSpec) {
	function menu(e: MouseEvent) {
		if (e.shiftKey || e.altKey || e.ctrlKey || e.metaKey) return;
		e.preventDefault();
		contextMenu.open(get(), el, { x: e.clientX, y: e.clientY });
	}
	el.addEventListener('contextmenu', menu);
	return {
		update(next: () => MenuSpec) {
			get = next;
		},
		destroy() {
			el.removeEventListener('contextmenu', menu);
		}
	};
}

/**
 * Press-and-hold on touch, right-click with a mouse. A hold that opened the
 * menu swallows the click that would otherwise follow, so the row's own link
 * doesn't fire too. Movement past a few pixels is a scroll or swipe, not a hold.
 */
export function longpress(el: HTMLElement, get: () => MenuSpec) {
	const HOLD_MS = 450;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let x0 = 0;
	let y0 = 0;
	let fired = false;
	el.style.setProperty('-webkit-touch-callout', 'none');
	el.style.userSelect = 'none';

	const cancel = () => clearTimeout(timer);
	function fire() {
		fired = true;
		navigator.vibrate?.(10);
		contextMenu.open(get(), el);
	}
	function down(e: PointerEvent) {
		fired = false;
		if (e.pointerType === 'mouse') return;
		x0 = e.clientX;
		y0 = e.clientY;
		timer = setTimeout(fire, HOLD_MS);
	}
	function move(e: PointerEvent) {
		if (Math.hypot(e.clientX - x0, e.clientY - y0) > 8) cancel();
	}
	function click(e: MouseEvent) {
		if (!fired) return;
		e.preventDefault();
		e.stopPropagation();
		fired = false;
	}
	function menu(e: MouseEvent) {
		e.preventDefault();
		// Android raises contextmenu for the same hold the timer already caught.
		if (!fired) {
			cancel();
			fire();
			fired = false;
		}
	}
	el.addEventListener('pointerdown', down);
	el.addEventListener('pointermove', move);
	el.addEventListener('pointerup', cancel);
	el.addEventListener('pointercancel', cancel);
	el.addEventListener('click', click, true);
	el.addEventListener('contextmenu', menu);
	return {
		update(next: () => MenuSpec) {
			get = next;
		},
		destroy() {
			cancel();
			el.removeEventListener('pointerdown', down);
			el.removeEventListener('pointermove', move);
			el.removeEventListener('pointerup', cancel);
			el.removeEventListener('pointercancel', cancel);
			el.removeEventListener('click', click, true);
			el.removeEventListener('contextmenu', menu);
		}
	};
}

/** Actions for a node row. `base` is '/m' in the phone app, '' on desktop. */
export function nodeMenu(n: Node, base: '' | '/m' = '/m'): MenuSpec {
	const name = n.name || shortKey(n.publicKey);
	const fav = favorites.has(n.publicKey);
	const items: MenuItem[] = [
		openItem(`${base}/nodes/${n.publicKey}`),
		{ label: fav ? 'Unstar' : 'Star', icon: ICONS.star, run: () => toggleStar(n.publicKey) }
	];
	if (n.latitude != null && n.longitude != null && (n.latitude !== 0 || n.longitude !== 0))
		items.push({
			label: 'Show on map',
			icon: ICONS.map,
			run: () => goto(`${base}/map?lat=${n.latitude!.toFixed(5)}&lon=${n.longitude!.toFixed(5)}&z=13&node=${n.publicKey}`)
		});
	items.push(
		{ label: 'Copy public key', icon: ICONS.copy, run: () => copyText(n.publicKey, 'Public key') },
		share(name, `/nodes/${n.publicKey}`, base)
	);
	return { title: name, subtitle: shortKey(n.publicKey), items };
}

export function observerMenu(o: Observer, base: '' | '/m' = '/m'): MenuSpec {
	const href = `/observers/${encodeURIComponent(o.id)}`;
	const items: MenuItem[] = [openItem(base + href)];
	if (o.publicKey) {
		items.push({ label: 'Open its node', icon: ICONS.open, run: () => goto(`${base}/nodes/${o.publicKey}`) });
		items.push({ label: 'Copy public key', icon: ICONS.copy, run: () => copyText(o.publicKey!, 'Public key') });
	}
	items.push(share(o.name ?? o.id, href, base));
	return { title: o.name ?? o.id, subtitle: o.region || undefined, items };
}

// Phones get the system share sheet; a desktop browser's share sheet is an
// odd detour from a right-click, so there it just copies the link.
const share = (title: string, path: string, base: string): MenuItem =>
	base
		? { label: 'Share', icon: ICONS.share, run: () => shareLink(title, path) }
		: { label: 'Copy link', icon: ICONS.copy, run: () => copyText(location.origin + path, 'Link') };
