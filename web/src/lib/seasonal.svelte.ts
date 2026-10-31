// Seasonal themes for the live maps: during a season's dates, packets can
// travel as themed icons (one per payload type) instead of the plain comet
// dot, and What's new can lead with the season's own items. Outside every
// season's dates the toggle, icons, legend and announcement items all vanish
// by themselves — no deploy needed when a season ends.
//
// To add a season (e.g. Christmas): append to SEASONS with its window, icons,
// legend and announce items, and bump CURRENT in announce.svelte.ts so the
// What's new modal auto-opens again. Preview one outside its dates with
// ?season=<id> on any page (kept for the browser tab; ?season=none clears it).
// A season marked `draft` never starts from its dates, and only admins (or
// anyone on dev) can preview it with ?season=<id>. The admin panel's Seasonal
// themes switch overrides the dates entirely: one theme on (draft or not), or
// all off; "Follow the calendar" hands control back to the dates.
//
// Shared by /live-map (SeasonalControls in Map Control), /m/live-map (the
// button at the bottom left), the WebGL-free fallback and AnnouncementModal.
import type maplibregl from 'maplibre-gl';
import { auth } from './auth.svelte';
import { env } from './env.svelte';

export interface SeasonItem {
	emoji: string;
	title: string;
	body: string;
}

export interface Season {
	id: string;
	/** Active from `from` until (not including) `until`, both ISO instants. */
	from: string;
	until: string;
	/** Map Control section heading, e.g. "Halloween". */
	name: string;
	/** Toggle label, e.g. "Spooky packets"; the phone button shows `short`. */
	label: string;
	short: string;
	/** The toggle's own emoji and accent colour. */
	emoji: string;
	accent: string;
	tip: string;
	/** Icon per payload type (names as PayloadTag knows them); `other` for the rest. */
	icons: Record<string, string>;
	other: string;
	legend: { icon: string; label: string }[];
	/** What's new items shown while the season is on, and the button that turns it on. */
	announce: SeasonItem[];
	tryLabel: string;
	/** Light snowfall over the live maps while the theme is on. */
	snow?: boolean;
	/** Still being designed: never in force by date; preview only for admins / on dev. */
	draft?: boolean;
}

export const SEASONS: Season[] = [
	{
		id: 'halloween',
		from: '2026-10-24T00:00:00-07:00',
		until: '2026-11-01T12:00:00-07:00', // noon in BC on Nov 1 (BC stays on UTC−7 year-round from Nov 2026)
		name: 'Halloween',
		label: 'Spooky packets',
		short: 'Spooky',
		emoji: '🎃',
		accent: '#ff7a1a',
		tip: 'Packets travel as pumpkins, bats, ghosts and witches',
		icons: {
			Advert: '🎃',
			GroupText: '🦇',
			TextMessage: '👻',
			Trace: '🧙‍♀️',
			Ack: '🕷️',
			Path: '🕸️',
			Request: '💀',
			AnonRequest: '💀',
			Response: '💀'
		},
		other: '🍬',
		legend: [
			{ icon: '🎃', label: 'Advert' },
			{ icon: '🦇', label: 'Channel' },
			{ icon: '👻', label: 'Direct' },
			{ icon: '🧙‍♀️', label: 'Trace' },
			{ icon: '🕷️', label: 'Ack' },
			{ icon: '🕸️', label: 'Path' },
			{ icon: '💀', label: 'Req / Resp' },
			{ icon: '🍬', label: 'Other' }
		],
		announce: [
			{
				emoji: '🎃',
				title: 'Spooky packets on the live map',
				body: 'Happy Halloween! On the live map, turn on Spooky packets at the top of the map (on a phone, tap the pumpkin at the bottom left). Every packet then flies across the map as a pumpkin, bat, ghost, witch, spider, web, skull or candy, one for each kind of packet, with a legend to tell them apart. It is off unless you turn it on, and the choice is saved in this browser.'
			},
			{
				emoji: '⏱️',
				title: 'Slow the packets down, or speed them up',
				body: 'A new Speed slider in Map Control sets how fast packets travel, from a quarter speed (easy to follow each hop) up to three times faster. On a phone, tap the speed button next to the pumpkin to step through the speeds.'
			}
		],
		tryLabel: 'Try spooky packets'
	},
	{
		id: 'christmas',
		from: '2026-12-11T00:00:00-07:00', // midnight in BC on Dec 11, two weeks before Christmas
		until: '2027-01-02T00:00:00-07:00', // through New Year's Day
		name: 'Christmas',
		label: 'Festive packets',
		short: 'Festive',
		emoji: '🎄',
		accent: '#e5484d',
		tip: 'Packets travel as trees, bells, presents, Santa and reindeer',
		icons: {
			Advert: '🎄',
			GroupText: '🔔',
			TextMessage: '🎁',
			Trace: '🎅',
			Ack: '❄️',
			Path: '🦌',
			Request: '⛄',
			AnonRequest: '⛄',
			Response: '⛄'
		},
		other: '🍪',
		legend: [
			{ icon: '🎄', label: 'Advert' },
			{ icon: '🔔', label: 'Channel' },
			{ icon: '🎁', label: 'Direct' },
			{ icon: '🎅', label: 'Trace' },
			{ icon: '❄️', label: 'Ack' },
			{ icon: '🦌', label: 'Path' },
			{ icon: '⛄', label: 'Req / Resp' },
			{ icon: '🍪', label: 'Other' }
		],
		announce: [
			{
				emoji: '🎄',
				title: 'Festive packets on the live map',
				body: 'Happy holidays! On the live map, turn on Festive packets at the top of the map (on a phone, tap the tree at the bottom left). Every packet then flies across the map as a tree, bell, present, Santa, reindeer, snowflake, snowman or cookie, one for each kind of packet, with a legend to tell them apart. It is off unless you turn it on, and the choice is saved in this browser.'
			}
		],
		tryLabel: 'Try festive packets',
		snow: true
	}
];

const KEY = 'ridgeline-livemap-season'; // id of the season this browser turned on
const LEGACY_KEY = 'ridgeline-livemap-spooky'; // v0.24.34–35: '1' = Halloween on
const PREVIEW_KEY = 'ridgeline-season-preview';

const codepoints = (icon: string) => [...icon].map((c) => c.codePointAt(0)!.toString(16));
const imageId = (icon: string) => 'season-' + codepoints(icon).join('-');

/** Bundled picture of an emoji (static/seasonal, Noto Emoji). Emoji fonts vary
 *  — some computers have none and draw plain black glyphs — so the icons are
 *  pictures, not text. */
const src = (icon: string) =>
	'/seasonal/emoji_u' + codepoints(icon).filter((c) => c !== 'fe0f').join('_') + '.svg';

// Drawn at 2× and registered with pixelRatio 2, so icons are 26 CSS px and sharp.
const PX = 52;

class Seasonal {
	#now = $state(Date.now());
	#preview = $state('');
	#enabled = $state('');
	#loaded = false;
	#pictures = new Map<string, HTMLImageElement>();
	/** Bumped as each picture finishes loading, so canvas drawers redraw. */
	pictures = $state(0);

	/** The season in force now (or being previewed), else null. */
	season = $derived.by((): Season | null => {
		if (this.#preview) {
			const p = SEASONS.find((s) => s.id === this.#preview);
			if (p && (!p.draft || auth.isAdmin || env.isDev)) return p;
		}
		return this.scheduled;
	});

	/** The season the site offers now, ignoring any preview: the admin's switch
	 *  (Admin → Seasonal themes) when set, else the dated, non-draft one. */
	scheduled = $derived.by((): Season | null => {
		if (env.season === 'none') return null;
		if (env.season) return SEASONS.find((s) => s.id === env.season) ?? null;
		const now = this.#now;
		return SEASONS.find((s) => !s.draft && now >= Date.parse(s.from) && now < Date.parse(s.until)) ?? null;
	});

	/** Themed icons showing: a season is in force and this browser turned it on. */
	get on(): boolean {
		return !!this.season && this.#enabled === this.season.id;
	}

	load() {
		if (this.#loaded || typeof window === 'undefined') return;
		this.#loaded = true;
		// Re-check the clock each minute so a season ends (or starts) on an open page.
		setInterval(() => (this.#now = Date.now()), 60_000);
		try {
			const q = new URLSearchParams(location.search).get('season');
			if (q !== null) sessionStorage.setItem(PREVIEW_KEY, q === 'none' ? '' : q);
			this.#preview = sessionStorage.getItem(PREVIEW_KEY) ?? '';
		} catch {
			/* no preview */
		}
		try {
			this.#enabled = localStorage.getItem(KEY) ?? (localStorage.getItem(LEGACY_KEY) === '1' ? 'halloween' : '');
		} catch {
			/* storage blocked: stays off */
		}
	}

	toggle() {
		const s = this.season;
		if (!s) return;
		this.#enabled = this.on ? '' : s.id;
		try {
			localStorage.setItem(KEY, this.#enabled);
			localStorage.removeItem(LEGACY_KEY);
		} catch {
			/* not remembered, still toggles */
		}
	}

	/** The current season's icon for a payload type. */
	icon(kind: string | undefined): string {
		const s = this.season;
		if (!s) return '';
		return (kind && s.icons[kind]) || s.other;
	}

	/** MapLibre image id for a payload type's icon (see ensureImages). */
	imageId(kind: string | undefined): string {
		return imageId(this.icon(kind));
	}

	/** The bundled picture's URL, for <img>. */
	src(icon: string): string {
		return src(icon);
	}

	/** A payload type's picture once loaded (for canvas drawing), else null; starts the load. */
	picture(kind: string | undefined): HTMLImageElement | null {
		const icon = this.icon(kind);
		if (!icon) return null;
		const img = this.#load(icon);
		return img.complete && img.naturalWidth > 0 ? img : null;
	}

	#load(icon: string): HTMLImageElement {
		let img = this.#pictures.get(icon);
		if (!img) {
			img = new Image();
			img.decoding = 'async';
			img.onload = () => this.pictures++;
			img.src = src(icon);
			this.#pictures.set(icon, img);
		}
		return img;
	}

	/** Register the current season's icons as map images. A style swap drops
	 *  images, so call this whenever the overlay layers are (re)added, and when
	 *  the season changes. Pictures still loading are added when they arrive. */
	ensureImages(map: maplibregl.Map) {
		const s = this.season;
		if (!s) return;
		for (const icon of new Set([...Object.values(s.icons), s.other])) {
			const id = imageId(icon);
			if (map.hasImage(id)) continue;
			const img = this.#load(icon);
			const add = (ok: boolean) => {
				try {
					if (map.hasImage(id)) return;
					const data = raster(ok ? img : icon);
					if (data) map.addImage(id, data, { pixelRatio: 2 });
				} catch {
					/* map gone or mid style swap; the next ensureImages call retries */
				}
			};
			if (img.complete) add(img.naturalWidth > 0);
			else {
				img.addEventListener('load', () => add(true), { once: true });
				img.addEventListener('error', () => add(false), { once: true });
			}
		}
	}
}

/** One icon as map-image pixels: the bundled picture, or the emoji as text if
 *  the picture failed to load. */
function raster(from: HTMLImageElement | string): ImageData | null {
	const canvas = document.createElement('canvas');
	canvas.width = canvas.height = PX;
	const ctx = canvas.getContext('2d');
	if (!ctx) return null;
	// A soft dark halo keeps light icons (ghost, web) readable on light basemaps.
	ctx.shadowColor = 'rgba(0,0,0,0.55)';
	ctx.shadowBlur = 4;
	if (typeof from === 'string') {
		ctx.textAlign = 'center';
		ctx.textBaseline = 'middle';
		ctx.font = `${PX * 0.78}px "Apple Color Emoji","Segoe UI Emoji","Noto Color Emoji",sans-serif`;
		ctx.fillText(from, PX / 2, PX / 2 + PX * 0.04);
	} else {
		const pad = PX * 0.1;
		ctx.drawImage(from, pad, pad, PX - 2 * pad, PX - 2 * pad);
	}
	return ctx.getImageData(0, 0, PX, PX);
}

export const seasonal = new Seasonal();
