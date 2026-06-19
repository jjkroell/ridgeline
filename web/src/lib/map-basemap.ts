// Shared basemap source for the maps. OpenFreeMap flat vector styles (the same
// dark basemap dev.meshcore.ca uses) — clean, no hillshade. MapLibre auto-adds
// the "OpenFreeMap © OpenMapTiles Data from OpenStreetMap" attribution. Theme
// aware: dark for the dark UI, positron for the light theme.
export function basemapStyleUrl(light: boolean): string {
	return light
		? 'https://tiles.openfreemap.org/styles/positron'
		: 'https://tiles.openfreemap.org/styles/dark';
}

// MapLibre's compact AttributionControl renders expanded by default. Collapse it
// to the ⓘ button so the credits stay out of the way (still expandable on click).
// Safe to call once the control exists; it stays collapsed afterwards because
// _updateCompact only re-opens when the 'maplibregl-compact' class is absent.
export function collapseAttribution(map: { getContainer(): HTMLElement }): void {
	const el = map.getContainer().querySelector('.maplibregl-ctrl-attrib');
	el?.classList.remove('maplibregl-compact-show');
	el?.removeAttribute('open');
}
