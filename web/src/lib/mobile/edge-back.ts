// Swipe from the left edge to go back, as in every iOS app. An installed
// (home-screen) web app on iPhone gets no browser back gesture at all, so
// there the app supplies one; in Safari itself, and on Android, the system
// already owns that edge and this stays off.
//
// The page follows the finger; let go past a third of the width (or with a
// quick flick) and it slides off and the app goes back, otherwise it springs
// home. Touch events, not pointer events: only a non-passive touchmove can
// stop the page scrolling while the edge is being dragged.
const EDGE = 24;
const SPRING = 'transform 0.3s cubic-bezier(0.32, 0.72, 0, 1)';

/** True only for an iPhone/iPad web app opened from the home screen. */
export function needsEdgeBack(): boolean {
	return (navigator as Navigator & { standalone?: boolean }).standalone === true;
}

/** Set while a swipe-back navigates, so the page cross-fade can stand aside:
 *  the page has already slid away under the finger. */
let sliding = false;
export function consumeEdgeBack(): boolean {
	const was = sliding;
	sliding = false;
	return was;
}

export function edgeBack(el: HTMLElement, opts: { enabled: () => boolean; onback: () => void }) {
	let x0 = 0;
	let y0 = 0;
	let t0 = 0;
	let dx = 0;
	let state: 'idle' | 'maybe' | 'drag' = 'idle';

	function set(px: number, animate: boolean) {
		el.style.transition = animate ? SPRING : 'none';
		el.style.transform = px ? `translateX(${px}px)` : '';
		el.style.boxShadow = px ? '-12px 0 28px rgba(0,0,0,0.22)' : '';
	}
	function start(e: TouchEvent) {
		const t = e.touches[0];
		if (e.touches.length !== 1 || t.clientX > EDGE || !opts.enabled()) return;
		x0 = t.clientX;
		y0 = t.clientY;
		t0 = e.timeStamp;
		dx = 0;
		state = 'maybe';
	}
	function move(e: TouchEvent) {
		if (state === 'idle') return;
		const t = e.touches[0];
		const ddx = t.clientX - x0;
		const ddy = t.clientY - y0;
		if (state === 'maybe') {
			if (Math.hypot(ddx, ddy) < 8) return;
			// Mostly sideways and rightwards is a back swipe; anything else
			// is a scroll that happened to start near the edge.
			if (ddx <= Math.abs(ddy)) {
				state = 'idle';
				return;
			}
			state = 'drag';
		}
		e.preventDefault();
		e.stopPropagation();
		dx = Math.max(0, ddx);
		set(dx, false);
	}
	function end(e: TouchEvent) {
		if (state !== 'drag') {
			state = 'idle';
			return;
		}
		state = 'idle';
		e.stopPropagation();
		const w = el.clientWidth;
		const v = dx / Math.max(1, e.timeStamp - t0);
		if (dx > w / 3 || (v > 0.5 && dx > 40)) {
			set(w, true);
			sliding = true;
			setTimeout(() => {
				const from = location.href;
				opts.onback();
				// The next page renders into this same element: put it back
				// once the address has changed (or give up after a second).
				let tries = 0;
				const wait = setInterval(() => {
					if (location.href !== from || ++tries > 30) {
						clearInterval(wait);
						requestAnimationFrame(() => set(0, false));
					}
				}, 35);
			}, 220);
		} else set(0, true);
	}
	el.addEventListener('touchstart', start, { passive: true });
	el.addEventListener('touchmove', move, { passive: false });
	el.addEventListener('touchend', end);
	el.addEventListener('touchcancel', end);
	return {
		update(next: typeof opts) {
			opts = next;
		},
		destroy() {
			el.removeEventListener('touchstart', start);
			el.removeEventListener('touchmove', move);
			el.removeEventListener('touchend', end);
			el.removeEventListener('touchcancel', end);
		}
	};
}
