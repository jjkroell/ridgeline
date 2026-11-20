<script lang="ts">
	// Interactive force-directed view of the mesh relay backbone, on a canvas
	// with a hand-rolled simulation so it stays smooth at scale.
	//
	// Legibility rules (the old view drew all ~400 links equally and coloured
	// by role, which on this mesh is 58 repeaters = one red blob):
	//  - only links seen ≥ minWeight times are drawn and simulated (plus each
	//    node's strongest, so nobody is stranded); width/opacity follow weight
	//  - colour = how much the node forwarded; role is the marker SHAPE
	//  - focusing a node (hover or select) shows ALL of its links, weak ones too
	//  - labels: top hubs + focus neighbourhood, with screen-space collision
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { shortKey } from '$lib/format';
	import type { TopologyNode, TopologyEdge } from '$lib/api';
	import { visibleEdges, relayNorm, relayRanks, topoPalette, rampAt, ROLE_SHAPE } from '$lib/topology';

	let {
		nodes,
		edges,
		nodePath = '/nodes',
		minWeight = 1,
		selected = null,
		onselect
	}: {
		nodes: TopologyNode[];
		edges: TopologyEdge[];
		nodePath?: string;
		minWeight?: number;
		selected?: string | null;
		/** When given, a click selects instead of navigating (desktop side panel). */
		onselect?: (key: string | null) => void;
	} = $props();

	let canvas: HTMLCanvasElement;
	let raf = 0;
	let resetView: () => void = () => {};
	let applyFilter: (w: number) => void = () => {};
	let applySelected: (k: string | null) => void = () => {};

	$effect(() => applyFilter(minWeight));
	$effect(() => applySelected(selected));

	onMount(() => {
		// only nodes that participate in an edge (isolated nodes carry no topology)
		const connected = new Set<string>();
		for (const e of edges) {
			connected.add(e.a);
			connected.add(e.b);
		}
		const gNodes = nodes.filter((n) => connected.has(n.publicKey));

		const dpr = window.devicePixelRatio || 1;
		const ctx = canvas.getContext('2d')!;
		const pal = topoPalette();
		const maxRelayed = Math.max(1, ...gNodes.map((n) => n.relayed));
		const ranks = relayRanks(gNodes);

		const fit = () => {
			const W = canvas.clientWidth,
				H = canvas.clientHeight;
			canvas.width = W * dpr;
			canvas.height = H * dpr;
			return { W, H };
		};
		let { W, H } = fit();

		const idx = new Map(gNodes.map((n, i) => [n.publicKey, i]));
		type PN = { n: TopologyNode; x: number; y: number; vx: number; vy: number; fixed: boolean; color: string; r: number };
		const P: PN[] = gNodes.map((n, i) => {
			const t = relayNorm(n, maxRelayed);
			return {
				n,
				x: W / 2 + Math.cos(i * 2.4) * (60 + (i % 180)),
				y: H / 2 + Math.sin(i * 2.4) * (60 + (i % 180)),
				vx: 0,
				vy: 0,
				fixed: false,
				color: rampAt(pal, ranks.get(n.publicKey) ?? 0),
				r: 3.5 + t * 9
			};
		});
		type PE = { a: number; b: number; w: number; inf: boolean; t: number };
		const allE: PE[] = edges
			.filter((e) => idx.has(e.a) && idx.has(e.b))
			.map((e) => ({ a: idx.get(e.a)!, b: idx.get(e.b)!, w: e.weight, inf: !!e.inferred, t: 0 }));
		const maxW = Math.max(1, ...allE.map((e) => e.w));
		for (const e of allE) e.t = Math.sqrt(e.w / maxW);
		allE.sort((x, y) => x.w - y.w); // paint weak first so strong links sit on top
		// full adjacency (focus shows every link, even filtered-out ones)
		const adjAll: Set<number>[] = P.map(() => new Set());
		for (const e of allE) {
			adjAll[e.a].add(e.b);
			adjAll[e.b].add(e.a);
		}

		// E = the drawn + simulated subset
		let E: PE[] = allE;
		applyFilter = (w: number) => {
			const keep = new Set(visibleEdges(edges, w).map((e) => `${e.a}|${e.b}`));
			E = allE.filter((e) => keep.has(`${P[e.a].n.publicKey}|${P[e.b].n.publicKey}`) || keep.has(`${P[e.b].n.publicKey}|${P[e.a].n.publicKey}`));
			alpha = Math.max(alpha, 0.6); // re-settle around the new link set
		};

		// Persistent labels: the 12 busiest forwarders.
		const hubs = new Set(
			P.map((p, i) => [p.n.relayed, i] as const)
				.sort((a, b) => b[0] - a[0])
				.slice(0, 12)
				.map(([, i]) => i)
		);

		let scale = 1,
			tx = 0,
			ty = 0,
			alpha = 1;
		let hover = -1,
			sel = -1,
			dragNode = -1,
			panning = false,
			moved = false;
		applySelected = (k: string | null) => {
			sel = k ? (idx.get(k) ?? -1) : -1;
		};
		applyFilter(untrack(() => minWeight));
		applySelected(untrack(() => selected));

		let lastX = 0,
			lastY = 0,
			downX = 0,
			downY = 0;
		// Active touch/mouse pointers → two of them = pinch-zoom (mobile).
		const pointers = new Map<number, { x: number; y: number }>();
		let pinchDist = 0;
		const pinch = () => {
			const p = [...pointers.values()];
			const dx = p[0].x - p[1].x,
				dy = p[0].y - p[1].y;
			return { dist: Math.hypot(dx, dy), cx: (p[0].x + p[1].x) / 2, cy: (p[0].y + p[1].y) / 2 };
		};
		const wx = (sx: number) => (sx - tx) / scale;
		const wy = (sy: number) => (sy - ty) / scale;

		function nodeAt(sx: number, sy: number) {
			const x = wx(sx),
				y = wy(sy);
			let best = -1,
				bd = 1e9;
			for (let i = 0; i < P.length; i++) {
				const d = (P[i].x - x) ** 2 + (P[i].y - y) ** 2;
				if (d < bd) {
					bd = d;
					best = i;
				}
			}
			if (best < 0) return -1;
			const r = P[best].r + 6 / scale;
			return bd <= r * r ? best : -1;
		}
		function physics() {
			if (alpha < 0.005) return;
			for (let i = 0; i < P.length; i++)
				for (let j = i + 1; j < P.length; j++) {
					const dx = P[i].x - P[j].x,
						dy = P[i].y - P[j].y,
						d2 = dx * dx + dy * dy + 0.01,
						d = Math.sqrt(d2),
						f = 1400 / d2;
					const fx = (dx / d) * f,
						fy = (dy / d) * f;
					P[i].vx += fx;
					P[i].vy += fy;
					P[j].vx -= fx;
					P[j].vy -= fy;
				}
			for (const e of E) {
				const a = P[e.a],
					b = P[e.b],
					dx = b.x - a.x,
					dy = b.y - a.y,
					d = Math.sqrt(dx * dx + dy * dy) + 0.01,
					// strong links pull tighter and shorter, so busy corridors read as
					// compact chains and weak cross-links stretch out of the way
					rest = 110 - e.t * 50,
					f = (d - rest) * (0.01 + e.t * 0.03);
				const fx = (dx / d) * f,
					fy = (dy / d) * f;
				a.vx += fx;
				a.vy += fy;
				b.vx -= fx;
				b.vy -= fy;
			}
			for (const p of P) {
				if (p.fixed) {
					p.vx = 0;
					p.vy = 0;
					continue;
				}
				p.vx += (W / 2 - p.x) * 0.003;
				p.vy += (H / 2 - p.y) * 0.003;
				p.x += p.vx * alpha;
				p.y += p.vy * alpha;
				p.vx *= 0.85;
				p.vy *= 0.85;
			}
			alpha *= 0.99;
		}

		const [lr, lg, lb] = pal.link;
		function shape(role: string, x: number, y: number, r: number) {
			const s = ROLE_SHAPE[role] ?? 'circle';
			ctx.beginPath();
			if (s === 'square') ctx.rect(x - r * 0.9, y - r * 0.9, r * 1.8, r * 1.8);
			else if (s === 'diamond') {
				ctx.moveTo(x, y - r * 1.25);
				ctx.lineTo(x + r * 1.25, y);
				ctx.lineTo(x, y + r * 1.25);
				ctx.lineTo(x - r * 1.25, y);
				ctx.closePath();
			} else if (s === 'triangle') {
				ctx.moveTo(x, y - r * 1.2);
				ctx.lineTo(x + r * 1.1, y + r * 0.8);
				ctx.lineTo(x - r * 1.1, y + r * 0.8);
				ctx.closePath();
			} else ctx.arc(x, y, r, 0, 7);
		}

		function drawEdge(e: PE, style: string, width: number) {
			ctx.strokeStyle = style;
			ctx.lineWidth = width / scale;
			// An edge resolved only from 1-byte hops is an inference, not a
			// measurement — dashed, matching PacketPathMap's ambiguous hops.
			ctx.setLineDash(e.inf ? [4 / scale, 3 / scale] : []);
			ctx.beginPath();
			ctx.moveTo(P[e.a].x, P[e.a].y);
			ctx.lineTo(P[e.b].x, P[e.b].y);
			ctx.stroke();
		}

		function draw() {
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			ctx.clearRect(0, 0, W, H);
			ctx.translate(tx, ty);
			ctx.scale(scale, scale);
			const f = hover >= 0 ? hover : sel;
			const hl = f >= 0;

			// background links (thin → thick, so strong links paint on top)
			for (const e of E) {
				if (hl && (e.a === f || e.b === f)) continue;
				const a = hl ? 0.05 : 0.1 + e.t * 0.55;
				drawEdge(e, `rgba(${lr},${lg},${lb},${a})`, 0.6 + e.t * 3.2);
			}
			// focus: every link of the focused node, filtered-out ones included
			if (hl)
				for (const e of allE)
					if (e.a === f || e.b === f) drawEdge(e, pal.focus, 0.8 + e.t * 3.2);
			ctx.setLineDash([]);

			for (let i = 0; i < P.length; i++) {
				const p = P[i],
					dim = hl && i !== f && !adjAll[f].has(i);
				ctx.globalAlpha = dim ? 0.18 : 1;
				shape(p.n.role, p.x, p.y, p.r);
				ctx.fillStyle = p.color;
				ctx.fill();
				ctx.lineWidth = 1.2 / scale;
				ctx.strokeStyle = pal.ink;
				ctx.stroke();
				if (i === f) {
					ctx.lineWidth = 2.2 / scale;
					ctx.strokeStyle = pal.label;
					ctx.stroke();
				}
			}
			ctx.globalAlpha = 1;

			// labels in screen space so text stays crisp and a fixed size at any zoom
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			const cand: { i: number; pri: number }[] = [];
			for (let i = 0; i < P.length; i++) {
				if (i === f) cand.push({ i, pri: 3 });
				else if (hl && adjAll[f].has(i)) cand.push({ i, pri: 2 });
				else if (!hl && hubs.has(i)) cand.push({ i, pri: 1 });
			}
			cand.sort((a, b) => b.pri - a.pri || P[b.i].n.relayed - P[a.i].n.relayed);
			const placed: [number, number, number, number][] = [];
			for (const { i, pri } of cand) {
				const p = P[i];
				const text = (p.n.name || shortKey(p.n.publicKey)).slice(0, 24);
				ctx.font = `${pri >= 2 ? 600 : 500} ${pri === 3 ? 13 : 11.5}px ui-sans-serif, system-ui`;
				const w = ctx.measureText(text).width;
				const sx = p.x * scale + tx + p.r * scale + 4,
					sy = p.y * scale + ty + 4;
				const box: [number, number, number, number] = [sx - 2, sy - 12, sx + w + 2, sy + 4];
				if (pri < 3 && placed.some((b) => box[0] < b[2] && box[2] > b[0] && box[1] < b[3] && box[3] > b[1]))
					continue;
				placed.push(box);
				// halo keeps names readable over links
				ctx.lineWidth = 3;
				ctx.strokeStyle = pal.ink;
				ctx.globalAlpha = 0.85;
				ctx.strokeText(text, sx, sy);
				ctx.globalAlpha = 1;
				ctx.fillStyle = pri === 1 ? pal.labelDim : pal.label;
				ctx.fillText(text, sx, sy);
			}
		}
		function loop() {
			physics();
			draw();
			raf = requestAnimationFrame(loop);
		}

		const onWheel = (ev: WheelEvent) => {
			ev.preventDefault();
			const rect = canvas.getBoundingClientRect(),
				mx = ev.clientX - rect.left,
				my = ev.clientY - rect.top;
			const ns = Math.max(0.15, Math.min(6, scale * (ev.deltaY < 0 ? 1.12 : 1 / 1.12)));
			tx = mx - (mx - tx) * (ns / scale);
			ty = my - (my - ty) * (ns / scale);
			scale = ns;
		};
		const onDown = (ev: PointerEvent) => {
			canvas.setPointerCapture(ev.pointerId);
			const rect = canvas.getBoundingClientRect();
			const mx = ev.clientX - rect.left,
				my = ev.clientY - rect.top;
			pointers.set(ev.pointerId, { x: mx, y: my });
			if (pointers.size === 2) {
				// second finger down → start pinch, abandon any single-touch gesture
				if (dragNode >= 0) {
					P[dragNode].fixed = false;
					dragNode = -1;
				}
				panning = false;
				pinchDist = pinch().dist;
				return;
			}
			lastX = downX = mx;
			lastY = downY = my;
			moved = false;
			const hit = nodeAt(mx, my);
			if (hit >= 0) {
				dragNode = hit;
				P[hit].fixed = true;
			} else {
				panning = true;
				canvas.style.cursor = 'grabbing';
			}
		};
		const onMove = (ev: PointerEvent) => {
			const rect = canvas.getBoundingClientRect(),
				mx = ev.clientX - rect.left,
				my = ev.clientY - rect.top;
			if (pointers.has(ev.pointerId)) pointers.set(ev.pointerId, { x: mx, y: my });
			if (pointers.size === 2) {
				const { dist, cx, cy } = pinch();
				if (pinchDist > 0 && dist > 0) {
					const ns = Math.max(0.15, Math.min(6, scale * (dist / pinchDist)));
					tx = cx - (cx - tx) * (ns / scale);
					ty = cy - (cy - ty) * (ns / scale);
					scale = ns;
				}
				pinchDist = dist;
				return;
			}
			if (Math.abs(mx - downX) + Math.abs(my - downY) > 3) moved = true;
			if (dragNode >= 0) {
				P[dragNode].x = wx(mx);
				P[dragNode].y = wy(my);
				P[dragNode].vx = 0;
				P[dragNode].vy = 0;
				alpha = Math.max(alpha, 0.3);
			} else if (panning) {
				tx += mx - lastX;
				ty += my - lastY;
			} else if (ev.pointerType === 'mouse') {
				hover = nodeAt(mx, my);
				canvas.style.cursor = hover >= 0 ? 'pointer' : 'grab';
			}
			lastX = mx;
			lastY = my;
		};
		const onUp = (ev: PointerEvent) => {
			try {
				canvas.releasePointerCapture(ev.pointerId);
			} catch {
				/* ignore */
			}
			pointers.delete(ev.pointerId);
			if (pointers.size < 2) pinchDist = 0;
			if (dragNode >= 0) {
				P[dragNode].fixed = false;
				if (!moved) {
					const key = P[dragNode].n.publicKey;
					if (onselect) onselect(sel === dragNode ? null : key);
					else goto(`${nodePath}/${key}`);
				}
				dragNode = -1;
			} else if (panning && !moved && onselect) {
				onselect(null); // tap on empty space clears the selection
			}
			panning = false;
			canvas.style.cursor = 'grab';
		};
		const onLeave = () => {
			hover = -1;
		};

		canvas.addEventListener('wheel', onWheel, { passive: false });
		canvas.addEventListener('pointerdown', onDown);
		canvas.addEventListener('pointermove', onMove);
		canvas.addEventListener('pointerup', onUp);
		canvas.addEventListener('pointercancel', onUp);
		canvas.addEventListener('pointerleave', onLeave);
		const ro = new ResizeObserver(() => {
			const d = fit();
			W = d.W;
			H = d.H;
		});
		ro.observe(canvas);
		resetView = () => {
			scale = 1;
			tx = 0;
			ty = 0;
			alpha = 1;
			for (const p of P) p.fixed = false;
		};
		loop();

		return () => {
			cancelAnimationFrame(raf);
			applyFilter = () => {};
			applySelected = () => {};
			ro.disconnect();
			canvas.removeEventListener('wheel', onWheel);
			canvas.removeEventListener('pointerdown', onDown);
			canvas.removeEventListener('pointermove', onMove);
			canvas.removeEventListener('pointerup', onUp);
			canvas.removeEventListener('pointercancel', onUp);
			canvas.removeEventListener('pointerleave', onLeave);
		};
	});
</script>

<div class="relative h-full w-full overflow-hidden">
	<canvas bind:this={canvas} class="h-full w-full" style="cursor:grab;touch-action:none"></canvas>
	<button
		onclick={() => resetView()}
		class="border-line bg-panel/85 text-fg-dim hover:border-signal/50 hover:text-signal absolute top-3 right-3 rounded-[var(--radius)] border px-2.5 py-1 font-mono text-[0.68rem] font-600 backdrop-blur transition-colors"
		>⟲ Reset view</button
	>
</div>
