// Raster-tile equivalents of the MapLibre basemaps, for the WebGL-free Leaflet
// maps. MapLibre's vector styles need WebGL, so each basemap id maps here to a
// free, key-less raster source (XYZ tiles). The ids + labels come from the shared
// BASEMAPS list (map-basemap.ts) so the selector UI and persistence are identical;
// only the rendering differs. `topo` (the default "Hillshade") layers an Esri
// shaded-relief overlay on the themed Esri Gray Canvas base to approximate the GL
// hillshade.
//
// The themed clean base used to be CARTO's positron/dark-matter, but CARTO
// deprecated key-less access and now serves those tiles stamped with an "API key
// required" watermark. Esri's Light/Dark Gray Canvas is the closest key-less,
// theme-aware replacement — and it is on the same arcgisonline host already used
// (key-less) for the imagery and hillshade layers. Base tiles carry no labels;
// the matching Reference layer carries transparent place/road labels, exactly the
// base+labels split the terrain layer already relies on.

export interface TileSpec {
	url: string;
	attribution: string;
	subdomains?: string;
	maxZoom: number;
	/** Highest zoom with real tiles; beyond it Leaflet overzooms (stretches) the
	 * last level instead of going blank. Used by the self-hosted terrain (z14). */
	maxNativeZoom?: number;
	/** Overlay opacity (hillshade only). */
	opacity?: number;
}
export interface LeafletBasemap {
	base: TileSpec;
	/** Shaded-relief overlay drawn (multiply-blended) above the base. */
	hillshade?: TileSpec;
	/** Transparent road/place labels drawn above the base (crisp at any zoom). */
	labels?: TileSpec;
}

export const ESRI_CANVAS_ATTR =
	'Tiles © <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a> — Esri, HERE, Garmin, © <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors';
const ESRI_IMAGERY_ATTR =
	'Imagery © <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a>, Maxar, Earthstar Geographics';
const ESRI_STREET_ATTR =
	'© <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a>, HERE, Garmin, © <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors';
const ESRI_HILLSHADE_ATTR =
	'Hillshade © <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a>';
const OTM_ATTR =
	'map data © <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors, SRTM | © <a href="https://opentopomap.org" target="_blank" rel="noopener">OpenTopoMap</a>';

// Esri Light/Dark Gray Canvas — key-less, theme-aware. Native to z16 (Leaflet
// overzooms past that rather than blanking). Base carries no labels; pair it with
// the Reference overlay for transparent place/road names. Exported so the small
// Leaflet insets (private-location picker, mini-map) share one source of truth.
export const esriGrayBaseUrl = (light: boolean): string =>
	`https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_${light ? 'Light' : 'Dark'}_Gray_Base/MapServer/tile/{z}/{y}/{x}`;
export const esriGrayLabelsUrl = (light: boolean): string =>
	`https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_${light ? 'Light' : 'Dark'}_Gray_Reference/MapServer/tile/{z}/{y}/{x}`;
export const ESRI_GRAY_MAX_NATIVE = 16;

const esriGrayBase = (light: boolean): TileSpec => ({
	url: esriGrayBaseUrl(light),
	maxZoom: 19,
	maxNativeZoom: ESRI_GRAY_MAX_NATIVE,
	attribution: ESRI_CANVAS_ATTR
});
const esriGrayLabels = (light: boolean): TileSpec => ({
	url: esriGrayLabelsUrl(light),
	maxZoom: 19,
	maxNativeZoom: ESRI_GRAY_MAX_NATIVE,
	attribution: ESRI_CANVAS_ATTR
});

// Opacity + blend mode are applied theme-aware in FallbackMap (screen on dark,
// multiply on light) so the relief stays visible on either base.
const ESRI_HILLSHADE: TileSpec = {
	url: 'https://server.arcgisonline.com/ArcGIS/rest/services/Elevation/World_Hillshade/MapServer/tile/{z}/{y}/{x}',
	maxZoom: 16,
	attribution: ESRI_HILLSHADE_ATTR
};

/** Tile config for the given basemap id, theme-aware where the GL base is. */
export function leafletBasemap(id: string, light: boolean): LeafletBasemap {
	switch (id) {
		case 'minimal':
			return { base: esriGrayBase(light), labels: esriGrayLabels(light) };
		case 'street':
			return {
				base: {
					url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}',
					maxZoom: 19,
					attribution: ESRI_STREET_ATTR
				}
			};
		case 'satellite':
			return {
				base: {
					url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
					maxZoom: 19,
					attribution: ESRI_IMAGERY_ATTR
				}
			};
		case 'terrain':
			return {
				base: {
					url: 'https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png',
					subdomains: 'abc',
					maxZoom: 17,
					attribution: OTM_ATTR
				}
			};
		case 'localterrain':
			// Self-hosted terrain (Copernicus GLO-30) via the maps.ve7kod.ca tunnel;
			// two themed renders follow the UI theme. Real tiles to z14 (overzoomed
			// past that). Themed Esri gray labels ride on top so place/road names
			// stay crisp at any zoom (full vector roads need WebGL → MapLibre path).
			return {
				base: {
					// ?v= cache-buster — bump on re-render (v3 = OSM-coastline sea mask). See map-basemap.ts.
					url: `https://maps.ve7kod.ca/terrain-${light ? 'light' : 'dark'}/{z}/{x}/{y}.png?v=3`,
					maxZoom: 19,
					maxNativeZoom: 14,
					attribution:
						'Terrain: <a href="https://github.com/tilezen/joerd" target="_blank" rel="noopener">Tilezen Joerd</a> recipe · Copernicus GLO-30 · self-hosted'
				},
				labels: esriGrayLabels(light)
			};
		case 'topo':
		default:
			// Shaded relief over the themed base — the closest raster match to the
			// MapLibre "Hillshade" default. Gray-canvas labels ride on top so names
			// stay readable over the relief.
			return {
				base: esriGrayBase(light),
				hillshade: ESRI_HILLSHADE,
				labels: esriGrayLabels(light)
			};
	}
}
