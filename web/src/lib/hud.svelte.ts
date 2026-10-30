// A brief confirmation for an action that has no visible result of its own
// (copy, star): a rounded pop-up in the middle of the screen, like the iOS
// "Copied" HUD. One at a time; a new one replaces the last. Rendered once by
// Hud.svelte in the root layout, so both the desktop and /m apps share it.
export type HudIcon = 'check' | 'star' | 'unstar' | 'error';

class HudStore {
	current = $state<{ msg: string; icon: HudIcon; id: number } | null>(null);
	#timer: ReturnType<typeof setTimeout> | undefined;
	#id = 0;

	show(msg: string, icon: HudIcon = 'check') {
		clearTimeout(this.#timer);
		this.current = { msg, icon, id: ++this.#id };
		this.#timer = setTimeout(() => (this.current = null), 1400);
	}
}
export const hud = new HudStore();
