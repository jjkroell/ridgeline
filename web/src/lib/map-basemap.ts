// Shared basemap source for the maps. OpenFreeMap vector tiles (© OpenMapTiles,
// data from OpenStreetMap) — MapLibre auto-adds the attribution from the style.
// Theme-aware: a dark style for the dark UI, positron for the light theme.
export function basemapStyleUrl(light: boolean): string {
	return light
		? 'https://tiles.openfreemap.org/styles/positron'
		: 'https://tiles.openfreemap.org/styles/dark';
}
