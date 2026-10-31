// A pulsing red ring on the maps over the node of whoever sent a #sos alert in
// the last 12 hours (the same window as the channel pulse), when that node has
// a position. Shared by /map, /live-map and their phone versions.
//
// A channel message carries only the sender's name, so the ring goes on the
// located node with that exact name — a companion first, then the most
// recently heard. Its position is the node's last advertised one, not
// necessarily where the sender is now.
//
// Tapping the "SOS · name" label opens a popup: open #sos, hide the ring on
// this browser's maps, or (admins) mark it resolved, which clears it for
// everyone. Either way a newer alert from that sender rings again.
//
// Rings are DOM markers, not style layers: they survive a basemap or theme
// swap and draw above clustered dots. Only the label takes clicks; the ring
// lets them through to the node beneath.

import maplibregl from 'maplibre-gl';
import { adminSos, type Node } from './api';
import { auth } from './auth.svelte';
import { sos } from './sos.svelte';
import { ago } from './format';

export interface SosSpot {
	key: string;
	name: string;
	lng: number;
	lat: number;
	/** When the alert was heard (epoch ms). */
	at: number;
}

/** Located nodes for the alert senders, one per sender. */
export function sosSpots(nodes: Node[], alerts: { sender: string; at: number }[]): SosSpot[] {
	const spots: SosSpot[] = [];
	const seen = new Set<string>();
	for (const a of alerts) {
		const name = a.sender.trim();
		const match = nodes
			.filter((n) => n.name?.trim() === name && n.latitude != null && n.longitude != null)
			.sort(
				(x, y) =>
					Number(y.role === 'ChatNode') - Number(x.role === 'ChatNode') || y.lastSeen.localeCompare(x.lastSeen)
			)[0];
		if (!match || seen.has(match.publicKey)) continue;
		seen.add(match.publicKey);
		spots.push({ key: match.publicKey, name: a.sender, lng: match.longitude!, lat: match.latitude!, at: a.at });
	}
	return spots;
}

/** Keeps one ring marker per spot on a map. */
export class SosRings {
	#map: maplibregl.Map;
	#markers = new Map<string, { marker: maplibregl.Marker; spot: SosSpot }>();
	#popup: maplibregl.Popup | null = null;

	constructor(map: maplibregl.Map) {
		this.#map = map;
	}

	update(spots: SosSpot[]) {
		const keep = new Set(spots.map((s) => s.key));
		for (const [key, m] of this.#markers) {
			if (!keep.has(key)) {
				m.marker.remove();
				this.#markers.delete(key);
			}
		}
		for (const s of spots) {
			const m = this.#markers.get(s.key);
			if (m) {
				m.spot = s;
				m.marker.setLngLat([s.lng, s.lat]);
				continue;
			}
			const el = document.createElement('div');
			el.className = 'sos-ring';
			el.innerHTML = '<span></span><span></span><b></b>';
			const label = document.createElement('button');
			label.type = 'button';
			label.textContent = `SOS · ${s.name.length > 22 ? s.name.slice(0, 21) + '…' : s.name}`;
			label.setAttribute('aria-label', `#sos alert from ${s.name} — options`);
			el.append(label);
			const entry = {
				marker: new maplibregl.Marker({ element: el, anchor: 'center' }).setLngLat([s.lng, s.lat]).addTo(this.#map),
				spot: s
			};
			label.addEventListener('click', (e) => {
				// Keep the map's own click (node sheet, deselect) out of it.
				e.stopPropagation();
				this.#open(entry.spot);
			});
			this.#markers.set(s.key, entry);
		}
	}

	#open(s: SosSpot) {
		this.#popup?.remove();
		const box = document.createElement('div');
		const title = document.createElement('div');
		title.className = 'sos-popup-title';
		title.textContent = `SOS · ${s.name}`;
		const meta = document.createElement('div');
		meta.className = 'sos-popup-meta';
		meta.textContent = `Alert heard ${ago(new Date(s.at).toISOString())} ago · shown at the node's last advertised position`;
		const actions = document.createElement('div');
		actions.className = 'sos-popup-actions';
		const open = document.createElement('a');
		open.href = `${location.pathname.startsWith('/m/') ? '/m' : ''}/channels?channel=sos`;
		open.textContent = 'Open #sos';
		const hide = document.createElement('button');
		hide.type = 'button';
		hide.textContent = 'Hide on my map';
		hide.addEventListener('click', () => {
			sos.hideRing(s.name, s.at);
			this.#popup?.remove();
		});
		actions.append(open, hide);
		box.append(title, meta, actions);
		if (auth.isAdmin) {
			const resolve = document.createElement('button');
			resolve.type = 'button';
			resolve.className = 'sos-popup-resolve';
			resolve.textContent = 'Resolved — clear for everyone';
			const err = document.createElement('div');
			err.className = 'sos-popup-meta';
			resolve.addEventListener('click', async () => {
				resolve.disabled = true;
				try {
					await adminSos.resolve(auth.csrf, s.name);
					await sos.load();
					this.#popup?.remove();
				} catch (e) {
					err.textContent = String((e as Error).message ?? e);
					resolve.disabled = false;
				}
			});
			box.append(resolve, err);
		}
		this.#popup = new maplibregl.Popup({ className: 'sos-popup', offset: 24, maxWidth: '260px' })
			.setLngLat([s.lng, s.lat])
			.setDOMContent(box)
			.addTo(this.#map);
	}

	destroy() {
		this.#popup?.remove();
		this.#markers.forEach((m) => m.marker.remove());
		this.#markers.clear();
	}
}
