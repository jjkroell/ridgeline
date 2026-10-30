// Swipe-to-act on a list row (Mail-style). Drag the row left to reveal the
// action behind it; release past THRESHOLD to fire it. Vertical scrolling is
// left to the browser (touch-action: pan-y), so only horizontal drags arrive
// here. A drag swallows the click that would otherwise follow it.
const THRESHOLD = 72;
const MAX = 96;

export function swipeAction(row: HTMLElement, onfire: () => void) {
	let x0 = 0;
	let y0 = 0;
	let dx = 0;
	let active = false;
	let dragged = false;
	row.style.touchAction = 'pan-y';

	function set(px: number, animate: boolean) {
		row.style.transition = animate ? 'transform 0.3s cubic-bezier(0.32, 0.72, 0, 1)' : 'none';
		row.style.transform = px ? `translateX(${px}px)` : '';
		row.parentElement?.style.setProperty('--swipe', String(Math.min(1, -px / THRESHOLD)));
	}
	function down(e: PointerEvent) {
		if (e.pointerType === 'mouse') return;
		x0 = e.clientX;
		y0 = e.clientY;
		dx = 0;
		active = true;
		dragged = false;
	}
	function move(e: PointerEvent) {
		if (!active) return;
		const mx = e.clientX - x0;
		if (!dragged) {
			if (Math.abs(e.clientY - y0) > 10) return void (active = false);
			if (Math.abs(mx) < 10) return;
			dragged = true;
			row.setPointerCapture(e.pointerId);
		}
		dx = Math.max(-MAX, Math.min(0, mx));
		set(dx, false);
	}
	function up() {
		if (!active) return;
		active = false;
		if (dragged && dx <= -THRESHOLD) onfire();
		set(0, true);
	}
	function click(e: MouseEvent) {
		if (dragged) {
			e.preventDefault();
			e.stopPropagation();
			dragged = false;
		}
	}
	row.addEventListener('pointerdown', down);
	row.addEventListener('pointermove', move);
	row.addEventListener('pointerup', up);
	row.addEventListener('pointercancel', up);
	row.addEventListener('click', click, true);
	return {
		update(fn: () => void) {
			onfire = fn;
		},
		destroy() {
			row.removeEventListener('pointerdown', down);
			row.removeEventListener('pointermove', move);
			row.removeEventListener('pointerup', up);
			row.removeEventListener('pointercancel', up);
			row.removeEventListener('click', click, true);
		}
	};
}
