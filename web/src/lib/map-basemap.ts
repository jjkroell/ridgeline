// Shared basemap source for the maps. OpenFreeMap flat vector styles (the same
// dark basemap dev.meshcore.ca uses) — clean, no hillshade. MapLibre auto-adds
// the "OpenFreeMap © OpenMapTiles Data from OpenStreetMap" attribution. Theme
// aware: dark for the dark UI, positron for the light theme.
export function basemapStyleUrl(light: boolean): string {
	return light
		? 'https://tiles.openfreemap.org/styles/positron'
		: 'https://tiles.openfreemap.org/styles/dark';
}
