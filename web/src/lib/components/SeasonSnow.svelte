<!--
  Light snowfall over a live map, while a season with `snow` is in force and
  the viewer's Snow switch is on (seasonal.snowing — independent of the
  themed packets; see $lib/seasonal.svelte). A click-through canvas filling the
  nearest positioned ancestor, under the map's controls (z-10). Skipped for
  prefers-reduced-motion; the animation stops whenever it is not showing.
-->
<script lang="ts">
	import { seasonal } from '$lib/seasonal.svelte';

	let canvas = $state<HTMLCanvasElement>();
	const show = $derived(seasonal.snowing);

	interface Flake {
		x: number;
		y: number;
		r: number;
		vy: number;
		sway: number;
		phase: number;
		a: number;
	}

	$effect(() => {
		if (!show || !canvas) return;
		if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const el = canvas;
		const ctx = el.getContext('2d');
		if (!ctx) return;

		let w = 0;
		let h = 0;
		let flakes: Flake[] = [];
		const flake = (y: number): Flake => {
			const r = 0.8 + Math.random() * 1.8; // bigger flakes fall faster: a little depth
			return {
				x: Math.random() * w,
				y,
				r,
				vy: 12 + r * 14,
				sway: 6 + Math.random() * 14,
				phase: Math.random() * Math.PI * 2,
				a: 0.35 + Math.random() * 0.45
			};
		};
		const resize = () => {
			const dpr = Math.min(devicePixelRatio || 1, 2);
			w = el.clientWidth;
			h = el.clientHeight;
			el.width = Math.round(w * dpr);
			el.height = Math.round(h * dpr);
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			// Light: about one flake per 9,000 px², capped.
			const n = Math.min(160, Math.round((w * h) / 9000));
			while (flakes.length < n) flakes.push(flake(Math.random() * h));
			flakes.length = n;
		};
		const ro = new ResizeObserver(resize);
		ro.observe(el);
		resize();

		let raf = 0;
		let last = performance.now();
		const frame = (now: number) => {
			const dt = Math.min(0.05, (now - last) / 1000); // no jump after a hidden tab
			last = now;
			ctx.clearRect(0, 0, w, h);
			ctx.fillStyle = '#ffffff';
			for (const f of flakes) {
				f.y += f.vy * dt;
				f.phase += dt * 0.9;
				if (f.y - f.r > h) Object.assign(f, flake(-f.r * 2));
				const x = f.x + Math.sin(f.phase) * f.sway;
				ctx.globalAlpha = f.a;
				ctx.beginPath();
				ctx.arc(x, f.y, f.r, 0, Math.PI * 2);
				ctx.fill();
			}
			ctx.globalAlpha = 1;
			raf = requestAnimationFrame(frame);
		};
		raf = requestAnimationFrame(frame);
		return () => {
			cancelAnimationFrame(raf);
			ro.disconnect();
			ctx.clearRect(0, 0, w, h);
		};
	});
</script>

{#if show}
	<canvas bind:this={canvas} class="pointer-events-none absolute inset-0 z-[5] h-full w-full" aria-hidden="true"></canvas>
{/if}
