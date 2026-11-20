// Shared helpers for the topology views (force graph + geographic map).
//
// The raw backbone is dense: 60 relays and up to ~540 links in 24 h, most of
// them sighted only a handful of times. Drawing every link with equal weight is
// what turned the page into a hairball, so both views draw only links seen at
// least `minWeight` times — plus each node's single strongest link, so raising
// the threshold never strands a node off on its own.
import type { TopologyEdge, TopologyNode } from './api';

/** Threshold stops offered by the "links seen ≥ N" control. */
export const WEIGHT_STEPS = [1, 2, 3, 5, 10, 20, 50, 100, 200, 500];

/** Edges to draw at a threshold: everything at/above it, plus every node's
 *  strongest edge (ties → first in the server's weight-desc order). */
export function visibleEdges(edges: TopologyEdge[], minWeight: number): TopologyEdge[] {
	const best = new Map<string, TopologyEdge>();
	for (const e of edges) {
		for (const k of [e.a, e.b]) {
			const cur = best.get(k);
			if (!cur || e.weight > cur.weight) best.set(k, e);
		}
	}
	const keep = new Set<TopologyEdge>(best.values());
	return edges.filter((e) => e.weight >= minWeight || keep.has(e));
}

/** Steps that actually change something for this edge set. */
export function stepsFor(edges: TopologyEdge[]): number[] {
	const max = Math.max(1, ...edges.map((e) => e.weight));
	const s = WEIGHT_STEPS.filter((w) => w <= max);
	return s.length ? s : [1];
}

/** A default threshold that leaves roughly 2.5 links per node — enough to see
 *  the structure, few enough to read it. */
export function autoMinWeight(edges: TopologyEdge[], nodeCount: number): number {
	const steps = stepsFor(edges);
	const target = Math.max(nodeCount * 2.5, 20);
	for (const s of steps) {
		if (edges.filter((e) => e.weight >= s).length <= target) return s;
	}
	return steps[steps.length - 1];
}

/** Every edge touching `key`, strongest first. */
export function linksOf(key: string, edges: TopologyEdge[]): { other: string; weight: number; inferred: boolean }[] {
	return edges
		.filter((e) => e.a === key || e.b === key)
		.map((e) => ({ other: e.a === key ? e.b : e.a, weight: e.weight, inferred: !!e.inferred }))
		.sort((x, y) => y.weight - x.weight);
}

/** Forwarding volume → 0..1 on a sqrt scale (a few hubs carry most traffic,
 *  so a linear scale would paint everything else the same faint colour). */
export function relayNorm(n: TopologyNode, maxRelayed: number): number {
	return Math.sqrt(n.relayed / Math.max(1, maxRelayed));
}

/** Forwarding RANK → 0..1 (quietest 0, busiest 1). Colour uses rank so the
 *  ramp spreads evenly across the mesh; size keeps the real volume. */
export function relayRanks(nodes: TopologyNode[]): Map<string, number> {
	const s = [...nodes].sort((a, b) => a.relayed - b.relayed);
	const d = Math.max(1, s.length - 1);
	return new Map(s.map((n, i) => [n.publicKey, i / d]));
}

// ── colour ramp ──────────────────────────────────────────────────────────────
// Canvas and MapLibre both need literal colours, never var(--…). Resolve the
// theme tokens once; a throwaway 2D context normalises any CSS colour syntax
// (hex, rgb(), oklch()…) into #rrggbb we can interpolate.
function toRgb(css: string, fallback: string): [number, number, number] {
	const c = document.createElement('canvas').getContext('2d');
	if (!c) return [139, 155, 173];
	c.fillStyle = fallback;
	c.fillStyle = css || fallback;
	const h = String(c.fillStyle);
	if (h.startsWith('#') && h.length === 7)
		return [parseInt(h.slice(1, 3), 16), parseInt(h.slice(3, 5), 16), parseInt(h.slice(5, 7), 16)];
	const m = h.match(/[\d.]+/g);
	return m ? [+m[0], +m[1], +m[2]] : [139, 155, 173];
}

export interface TopoPalette {
	/** quiet → busy forwarding ramp stops (hex) */
	ramp: [string, string, string];
	link: [number, number, number];
	focus: string;
	label: string;
	labelDim: string;
	ink: string;
}

const hex = (r: [number, number, number]) => '#' + r.map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');

export function topoPalette(): TopoPalette {
	const cs = getComputedStyle(document.documentElement);
	const v = (n: string) => cs.getPropertyValue(n).trim();
	return {
		ramp: [
			hex(toRgb(v('--color-fg-faint'), '#5d6b7a')),
			hex(toRgb(v('--color-signal'), '#34e3c4')),
			hex(toRgb(v('--color-amber'), '#e8b454'))
		],
		link: toRgb(v('--color-fg-dim'), '#8b9bad'),
		focus: hex(toRgb(v('--color-signal'), '#34e3c4')),
		label: hex(toRgb(v('--color-fg'), '#e6edf3')),
		labelDim: hex(toRgb(v('--color-fg-dim'), '#c7d2de')),
		ink: hex(toRgb(v('--color-ink'), '#070a0e'))
	};
}

/** Interpolate the three-stop ramp at t ∈ [0,1]. */
export function rampAt(p: TopoPalette, t: number): string {
	const stops = p.ramp.map((h) => toRgb(h, h));
	const x = Math.max(0, Math.min(1, t)) * 2;
	const i = Math.min(1, Math.floor(x));
	const f = x - i;
	const a = stops[i],
		b = stops[i + 1];
	return hex([a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f, a[2] + (b[2] - a[2]) * f]);
}

/** Roles that get a non-circle marker so the rare ones stand out among the
 *  repeaters (colour now carries forwarding volume, not role). */
export const ROLE_SHAPE: Record<string, 'circle' | 'square' | 'diamond' | 'triangle'> = {
	Repeater: 'circle',
	RoomServer: 'square',
	ChatNode: 'diamond',
	Sensor: 'triangle'
};
export const ROLE_NAME: Record<string, string> = {
	Repeater: 'Repeater',
	RoomServer: 'Room',
	ChatNode: 'Companion',
	Sensor: 'Sensor'
};
