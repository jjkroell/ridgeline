// Press and drag across a bar chart or heatmap to read it (Health-style), or
// hover with a mouse. Each readable mark carries data-scrub-i="<index>"; the
// action reports the index under the pointer, and null when the pointer lifts
// or leaves. Bars match on x alone (a short bar is still "under" the finger
// anywhere in its column); a heatmap ('xy') needs the cell itself.
// touch-action: pan-y leaves vertical page scrolling to the browser.
export interface ScrubOptions {
	axis?: 'x' | 'xy';
	onpick: (i: number | null) => void;
}

export function scrub(el: HTMLElement, opts: ScrubOptions) {
	let pressed = false;
	el.style.touchAction = 'pan-y';
	el.style.userSelect = 'none';

	function pick(e: PointerEvent) {
		for (const it of el.querySelectorAll<HTMLElement>('[data-scrub-i]')) {
			const r = it.getBoundingClientRect();
			const inX = e.clientX >= r.left - 1 && e.clientX <= r.right + 1;
			const inY = opts.axis !== 'xy' || (e.clientY >= r.top - 2 && e.clientY <= r.bottom + 2);
			if (inX && inY) return opts.onpick(Number(it.dataset.scrubI));
		}
	}
	function down(e: PointerEvent) {
		pressed = true;
		pick(e);
	}
	function move(e: PointerEvent) {
		if (pressed || e.pointerType === 'mouse') pick(e);
	}
	function end() {
		pressed = false;
		opts.onpick(null);
	}
	el.addEventListener('pointerdown', down);
	el.addEventListener('pointermove', move);
	el.addEventListener('pointerup', end);
	el.addEventListener('pointercancel', end);
	el.addEventListener('pointerleave', end);
	return {
		update(next: ScrubOptions) {
			opts = next;
		},
		destroy() {
			el.removeEventListener('pointerdown', down);
			el.removeEventListener('pointermove', move);
			el.removeEventListener('pointerup', end);
			el.removeEventListener('pointercancel', end);
			el.removeEventListener('pointerleave', end);
		}
	};
}
