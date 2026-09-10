// Hover labels for the MapLibre node layers (/map, /live-map).
//
// Kept out of map-util.ts on purpose: that module is also imported by the
// Leaflet views (FallbackMap, LeafletInset) and by theme.svelte.ts, none of
// which should drag maplibre-gl into their bundle.
//
// The Leaflet fallback map does the same job with bindTooltip; this is the
// WebGL equivalent, styled to match in app.css.
import maplibregl from 'maplibre-gl';

/**
 * Show the node's name while the pointer is over `layer`.
 *
 * A hover affordance, not a popup: no close button, no pointer events of its
 * own (so it can never swallow the click that opens node detail), and it never
 * outlives the pointer leaving the layer. Clicking clears it too, otherwise it
 * would sit behind the node modal with no mouseleave to dismiss it.
 *
 * Bound with `mousemove` rather than `mouseenter` so that sliding straight from
 * one dot to an adjacent one relabels, instead of keeping the first name until
 * the pointer happens to cross a gap.
 *
 * setText, never setHTML: node names come off the mesh and are attacker
 * controlled.
 *
 * Returns the popup so a caller can remove it on teardown.
 */
export function attachHoverLabel(map: maplibregl.Map, layer: string): maplibregl.Popup {
	const popup = new maplibregl.Popup({
		closeButton: false,
		closeOnClick: false,
		focusAfterOpen: false,
		offset: 12,
		className: 'node-hover'
	});

	map.on('mousemove', layer, (e) => {
		const f = e.features?.[0];
		if (!f) return;
		const p = f.properties as { name?: string; pubkey?: string };
		// live-map stores the raw name, which can be empty; fall back to the
		// short key the way /map already does when building its features.
		const label = p?.name || (p?.pubkey ? p.pubkey.slice(0, 10) : '');
		if (!label) {
			popup.remove();
			return;
		}
		popup
			.setLngLat((f.geometry as GeoJSON.Point).coordinates as [number, number])
			.setText(label)
			.addTo(map);
	});
	map.on('mouseleave', layer, () => popup.remove());
	map.on('click', layer, () => popup.remove());

	return popup;
}
