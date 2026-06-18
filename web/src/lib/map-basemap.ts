// Shared basemap source for the maps. OpenFreeMap terrain vector styles, which
// carry shaded relief (Tilezen Joerd) — MapLibre auto-adds the attribution
// "Tilezen Joerd | OpenFreeMap © OpenMapTiles Data from OpenStreetMap". Theme
// aware: Fiord's slate blue-grey for the dark UI, Liberty's lighter palette for
// the light theme.
export function basemapStyleUrl(light: boolean): string {
	return light
		? 'https://tiles.openfreemap.org/styles/liberty'
		: 'https://tiles.openfreemap.org/styles/fiord';
}
