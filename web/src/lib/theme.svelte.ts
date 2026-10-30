// Named colour themes, persisted to localStorage. The initial attribute/class is
// applied by an inline script in app.html (before paint); this store mirrors it
// for the picker UI and handles changes.
//
// Two separate signals live on <html>, and they do different jobs:
//   data-theme="<id>"    selects the token block in app.css (mutually exclusive,
//                        so there are no specificity games between themes)
//   class="theme-light"  marks the theme as having a LIGHT base — read by
//                        map-util.isLight() to pick basemap/hillshade variants,
//                        and by color-scheme. Any number of themes may set it.

export type ThemeId = 'dark' | 'light' | 'slate' | 'graphite' | 'mist' | 'apple' | 'apple-dark';
/** What the picker stores: a concrete theme, or 'apple-auto', which follows
 *  the system light/dark setting and resolves to apple / apple-dark. */
export type ThemeChoice = ThemeId | 'apple-auto';

/** Cupertino tint colours (iOS Settings → Accent colour). Stored separately
 *  from the theme; only the Cupertino themes read data-accent. */
export type AccentId = 'blue' | 'purple' | 'pink' | 'red' | 'orange' | 'green' | 'graphite';
export const ACCENTS: { id: AccentId; label: string; light: string; dark: string }[] = [
	{ id: 'blue', label: 'Blue', light: '#0071e3', dark: '#0a84ff' },
	{ id: 'purple', label: 'Purple', light: '#8944ab', dark: '#bf5af2' },
	{ id: 'pink', label: 'Pink', light: '#d30f45', dark: '#ff375f' },
	{ id: 'red', label: 'Red', light: '#d70015', dark: '#ff453a' },
	{ id: 'orange', label: 'Orange', light: '#c93400', dark: '#ff9f0a' },
	{ id: 'green', label: 'Green', light: '#248a3d', dark: '#30d158' },
	{ id: 'graphite', label: 'Graphite', light: '#6e6e73', dark: '#98989d' }
];

export interface ThemeDef {
	id: ThemeId;
	label: string;
	/** Light-based themes get the `theme-light` class (basemaps, color-scheme). */
	light: boolean;
	/** Picker swatch colours: [ground, panel, accent]. */
	swatch: [string, string, string];
	/** Browser UI colour (PWA address bar / status bar). */
	themeColor: string;
}

export const THEMES: ThemeDef[] = [
	{
		id: 'dark',
		label: 'Ridgeline',
		light: false,
		swatch: ['#0e151b', '#161f28', '#34e3c4'],
		themeColor: '#070a0e'
	},
	{
		id: 'slate',
		label: 'Slate',
		light: false,
		swatch: ['#0b1220', '#142032', '#5cc8ff'],
		themeColor: '#0b1220'
	},
	{
		id: 'graphite',
		label: 'Graphite',
		light: false,
		swatch: ['#14100c', '#221b14', '#e0a63f'],
		themeColor: '#14100c'
	},
	{
		id: 'light',
		label: 'Paper',
		light: true,
		swatch: ['#e1e5e0', '#fbfcfa', '#0f9e8a'],
		themeColor: '#e1e5e0'
	},
	{
		id: 'mist',
		label: 'Mist',
		light: true,
		swatch: ['#dbe3ec', '#f7fafd', '#2b5fa8'],
		themeColor: '#dbe3ec'
	},
	{
		id: 'apple',
		label: 'Cupertino',
		light: true,
		swatch: ['#e4e4e9', '#f3f3f6', '#0071e3'],
		themeColor: '#e4e4e9'
	},
	{
		id: 'apple-dark',
		label: 'Cupertino Dark',
		light: false,
		swatch: ['#000000', '#1c1c1e', '#0a84ff'],
		themeColor: '#000000'
	}
];

const BY_ID = new Map(THEMES.map((t) => [t.id, t]));
const KEY = 'ridgeline-theme';
const ACCENT_KEY = 'ridgeline-accent';
const SOLID_KEY = 'ridgeline-solid';
const DEFAULT: ThemeId = 'dark';

/** Accepts anything from storage; falls back to the default. Pre-existing
 *  installs hold 'dark' or 'light', both of which are still valid ids. */
function coerce(raw: string | null): ThemeChoice {
	if (raw === 'apple-auto') return raw;
	return raw && BY_ID.has(raw as ThemeId) ? (raw as ThemeId) : DEFAULT;
}

function systemDark(): boolean {
	return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches;
}

function resolve(choice: ThemeChoice): ThemeId {
	if (choice !== 'apple-auto') return choice;
	return systemDark() ? 'apple-dark' : 'apple';
}

class Theme {
	/** The stored selection (may be 'apple-auto'). */
	choice = $state<ThemeChoice>(DEFAULT);
	/** The theme actually applied — what every map and token reads. */
	id = $state<ThemeId>(DEFAULT);
	accent = $state<AccentId>('blue');
	/** Reduce transparency: frosted bars and panels become solid. Also on
	 *  whenever the OS asks (prefers-reduced-transparency, in app.css). */
	solid = $state(false);

	/** Any Cupertino variant is active (structural layer applies). */
	get isApple(): boolean {
		return this.id === 'apple' || this.id === 'apple-dark';
	}

	get def(): ThemeDef {
		return BY_ID.get(this.id) ?? THEMES[0];
	}

	/** True when the active theme has a light base. */
	get isLight(): boolean {
		return this.def.light;
	}

	init() {
		let saved: string | null = null;
		try {
			saved = localStorage.getItem(KEY);
		} catch {
			/* storage unavailable */
		}
		this.choice = coerce(saved);
		try {
			const a = localStorage.getItem(ACCENT_KEY);
			if (a && ACCENTS.some((x) => x.id === a)) this.accent = a as AccentId;
			this.solid = localStorage.getItem(SOLID_KEY) === '1';
		} catch {
			/* storage unavailable */
		}
		// Auto follows the OS live, e.g. at sunset when macOS/iOS switch.
		if (typeof matchMedia === 'function') {
			matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
				if (this.choice === 'apple-auto') this.#apply();
			});
		}
		this.#apply();
	}

	set(choice: ThemeChoice) {
		if ((choice !== 'apple-auto' && !BY_ID.has(choice)) || choice === this.choice) return;
		this.choice = choice;
		try {
			localStorage.setItem(KEY, choice);
		} catch {
			/* storage unavailable */
		}
		this.#apply();
	}

	setAccent(id: AccentId) {
		this.accent = id;
		try {
			localStorage.setItem(ACCENT_KEY, id);
		} catch {
			/* storage unavailable */
		}
		document.documentElement.dataset.accent = id;
	}

	setSolid(on: boolean) {
		this.solid = on;
		try {
			if (on) localStorage.setItem(SOLID_KEY, '1');
			else localStorage.removeItem(SOLID_KEY);
		} catch {
			/* storage unavailable */
		}
		document.documentElement.toggleAttribute('data-solid', on);
	}

	/** Next theme in list order — a one-tap affordance where a full picker
	 *  doesn't fit. Auto sits after Cupertino Dark. */
	cycle() {
		const order: ThemeChoice[] = [...THEMES.map((t) => t.id), 'apple-auto'];
		const i = order.indexOf(this.choice);
		this.set(order[(i + 1) % order.length]);
	}

	#apply() {
		this.id = resolve(this.choice);
		const def = this.def;
		const root = document.documentElement;
		root.dataset.theme = def.id;
		root.dataset.accent = this.accent;
		root.toggleAttribute('data-solid', this.solid);
		root.classList.toggle('theme-light', def.light);
		document.querySelector('meta[name="theme-color"]')?.setAttribute('content', def.themeColor);
	}
}

export const theme = new Theme();
