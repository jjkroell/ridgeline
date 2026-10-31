// Halloween mode for the live maps: each packet travels as a pumpkin, bat,
// ghost, witch… (one per payload type) instead of the plain comet dot. A
// per-browser toggle in Map Control (desktop) or the pumpkin button (phone),
// shared by /live-map, /m/live-map and the WebGL-free fallback.
import type maplibregl from 'maplibre-gl';

const KEY = 'ridgeline-livemap-spooky';

/** Icon per payload type (names as PayloadTag knows them). */
export const SPOOKY_ICONS: Record<string, string> = {
	Advert: '🎃',
	GroupText: '🦇',
	TextMessage: '👻',
	Trace: '🧙‍♀️',
	Ack: '🕷️',
	Path: '🕸️',
	Request: '💀',
	AnonRequest: '💀',
	Response: '💀'
};
export const SPOOKY_OTHER = '🍬';

/** The legend, in the order it's shown. */
export const SPOOKY_LEGEND: { icon: string; label: string }[] = [
	{ icon: '🎃', label: 'Advert' },
	{ icon: '🦇', label: 'Channel' },
	{ icon: '👻', label: 'Direct' },
	{ icon: '🧙‍♀️', label: 'Trace' },
	{ icon: '🕷️', label: 'Ack' },
	{ icon: '🕸️', label: 'Path' },
	{ icon: '💀', label: 'Req / Resp' },
	{ icon: '🍬', label: 'Other' }
];

export function spookyIcon(kind: string | undefined): string {
	return (kind && SPOOKY_ICONS[kind]) || SPOOKY_OTHER;
}

const imageId = (icon: string) => 'spooky-' + [...icon].map((c) => c.codePointAt(0)!.toString(16)).join('-');

/** MapLibre image id for a payload type's icon. */
export function spookyImageId(kind: string | undefined): string {
	return imageId(spookyIcon(kind));
}

// Drawn at 2× and registered with pixelRatio 2, so icons are 26 CSS px and sharp.
const PX = 52;

/** Register every icon as a map image. A style swap drops images, so call
 *  this whenever the overlay layers are (re)added. */
export function ensureSpookyImages(map: maplibregl.Map) {
	const icons = [...new Set([...Object.values(SPOOKY_ICONS), SPOOKY_OTHER])];
	for (const icon of icons) {
		const id = imageId(icon);
		if (map.hasImage(id)) continue;
		const canvas = document.createElement('canvas');
		canvas.width = canvas.height = PX;
		const ctx = canvas.getContext('2d');
		if (!ctx) continue;
		ctx.textAlign = 'center';
		ctx.textBaseline = 'middle';
		ctx.font = `${PX * 0.78}px "Apple Color Emoji","Segoe UI Emoji","Noto Color Emoji",sans-serif`;
		// A soft dark halo keeps light icons (ghost, web) readable on light basemaps.
		ctx.shadowColor = 'rgba(0,0,0,0.55)';
		ctx.shadowBlur = 4;
		ctx.fillText(icon, PX / 2, PX / 2 + PX * 0.04);
		map.addImage(id, ctx.getImageData(0, 0, PX, PX), { pixelRatio: 2 });
	}
}

class Spooky {
	on = $state(false);
	#loaded = false;

	load() {
		if (this.#loaded || typeof localStorage === 'undefined') return;
		this.#loaded = true;
		try {
			this.on = localStorage.getItem(KEY) === '1';
		} catch {
			/* storage blocked: stays off */
		}
	}

	toggle() {
		this.on = !this.on;
		try {
			localStorage.setItem(KEY, this.on ? '1' : '0');
		} catch {
			/* not remembered, still toggles */
		}
	}
}

export const spooky = new Spooky();
