// OpenLayers basemap foundation. OL renders to a 2D <canvas> (no WebGL), so the
// maps work for users who keep WebGL disabled for fingerprinting reasons — the
// whole point of moving off MapLibre GL. Vector basemaps are kept by feeding the
// existing OpenFreeMap GL style JSON through ol-mapbox-style; raster bases and
// the terrain hillshade are plain XYZ tile layers.
//
// Importing OL's stylesheet here means every component that builds a map (they
// all import this module) gets the control/attribution styling without each
// route having to remember the import — the lesson from the MapLibre CSS bug.
import 'ol/ol.css';

import type Map from 'ol/Map';
import VectorTileLayer from 'ol/layer/VectorTile';
import TileLayer from 'ol/layer/Tile';
import ImageLayer from 'ol/layer/Image';
import type BaseLayer from 'ol/layer/Base';
import XYZ from 'ol/source/XYZ';
import Static from 'ol/source/ImageStatic';
import Overlay from 'ol/Overlay';
import { fromLonLat } from 'ol/proj';
import { applyStyle } from 'ol-mapbox-style';
import type { CoverageResult } from '$lib/coverage';

import { BASEMAPS, DEFAULT_BASEMAP, basemapHasHillshade } from '$lib/map-basemap';
export { BASEMAPS, DEFAULT_BASEMAP, basemapHasHillshade };

// Theme-aware flat OpenFreeMap vector style — the default base and inset maps.
export function vectorStyleUrl(light: boolean): string {
	return light
		? 'https://tiles.openfreemap.org/styles/positron'
		: 'https://tiles.openfreemap.org/styles/dark';
}

// A vector-tile base layer styled from an OpenFreeMap GL style. ol-mapbox-style
// sets the layer's source + canvas style functions from the style JSON.
export function vectorBaseLayer(light: boolean, id = 'minimal'): VectorTileLayer {
	const layer = new VectorTileLayer({ declutter: true, properties: { base: true } });
	applyStyle(layer, glStyleUrl(id, light)).catch(() => {});
	return layer;
}

// Re-style an existing vector base in place (theme swap / basemap change) without
// tearing the layer down — ol-mapbox-style replaces the source + style functions.
export function restyleVectorBase(layer: VectorTileLayer, id: string, light: boolean): void {
	applyStyle(layer, glStyleUrl(id, light)).catch(() => {});
}

function glStyleUrl(id: string, light: boolean): string {
	if (id === 'street') return 'https://tiles.openfreemap.org/styles/liberty';
	return vectorStyleUrl(light); // topo + minimal share the flat themed base
}

export function isVectorBase(id: string): boolean {
	return id === 'topo' || id === 'minimal' || id === 'street';
}

// Raster imagery bases (satellite/terrain). Liberty/street is vector, handled above.
export function rasterBaseLayer(id: string): TileLayer<XYZ> | null {
	if (id === 'satellite') {
		return new TileLayer({
			properties: { base: true },
			source: new XYZ({
				url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
				maxZoom: 19,
				attributions:
					'Imagery © <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a>, Maxar, Earthstar Geographics'
			})
		});
	}
	if (id === 'terrain') {
		return new TileLayer({
			properties: { base: true },
			source: new XYZ({
				urls: [
					'https://a.tile.opentopomap.org/{z}/{x}/{y}.png',
					'https://b.tile.opentopomap.org/{z}/{x}/{y}.png',
					'https://c.tile.opentopomap.org/{z}/{x}/{y}.png'
				],
				maxZoom: 17,
				attributions:
					'© <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors, SRTM | © <a href="https://opentopomap.org" target="_blank" rel="noopener">OpenTopoMap</a> (CC-BY-SA)'
			})
		});
	}
	return null;
}

// Terrain relief as a PRE-BAKED hillshade raster (Esri World Hillshade) overlaid
// on the flat vector base for the "topo"/Hillshade basemap. This replaces the GPU
// `raster-dem` hillshade MapLibre computed — a grayscale shaded-relief tile set,
// blended by opacity so it reads on both themes (no WebGL, no tile pipeline of
// our own). Fidelity upgrade path: an ol/source/Raster pixel-op from the terrarium
// DEM in a worker to restore per-theme tinting.
export function hillshadeLayer(light: boolean): TileLayer<XYZ> {
	return new TileLayer({
		properties: { hillshade: true },
		opacity: light ? 0.32 : 0.28,
		source: new XYZ({
			url: 'https://services.arcgisonline.com/arcgis/rest/services/Elevation/World_Hillshade/MapServer/tile/{z}/{y}/{x}',
			maxZoom: 16,
			attributions:
				'Hillshade: <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a>, USGS, NGA, NASA'
		})
	});
}

// Layers for an inset location thumbnail: the flat vector base + a light terrain
// relief. Returns a `setTheme` so the caller can swap dark↔light tiles live when
// the UI theme toggles (the base tiles must be re-styled — a CSS filter can't do
// that). Keeps OL layer types out of the Svelte components.
const insetHillOpacity = (light: boolean) => (light ? 0.32 : 0.28);
export function insetLayers(light: boolean): { layers: BaseLayer[]; setTheme: (light: boolean) => void } {
	const base = vectorBaseLayer(light);
	const hill = hillshadeLayer(light);
	hill.setOpacity(insetHillOpacity(light));
	return {
		layers: [base, hill],
		setTheme: (l: boolean) => {
			restyleVectorBase(base, 'minimal', l);
			hill.setOpacity(insetHillOpacity(l));
		}
	};
}

// Swap the basemap (and its hillshade) in place for a new id/theme. Base layers
// are tagged `base`/`hillshade` and kept at the bottom of the stack so the
// overlays added by the page (coverage, nodes) sit above them and survive the
// swap — the OL analogue of MapLibre's setStyle + ensureOverlays dance, but
// without the style-reload race (we just add/remove layers).
export function applyBasemap(map: Map, id: string, light: boolean): void {
	const layers = map.getLayers();
	layers
		.getArray()
		.slice()
		.filter((l) => l.get('base') || l.get('hillshade'))
		.forEach((l) => layers.remove(l));

	const base: BaseLayer[] = [];
	if (isVectorBase(id)) base.push(vectorBaseLayer(light, id));
	else {
		const r = rasterBaseLayer(id);
		if (r) base.push(r);
	}
	if (basemapHasHillshade(id)) base.push(hillshadeLayer(light));
	// Insert in order at the bottom: base at 0, hillshade at 1, overlays above.
	base.forEach((l, i) => layers.insertAt(i, l));
}

// Coverage viewshed overlay: a teal terrain-shadow PNG positioned by its geo
// bbox. Empty + hidden until a coverage is computed; sits above the basemap but
// below the node markers.
export function createCoverageLayer(): ImageLayer<Static> {
	return new ImageLayer({ properties: { coverage: true }, opacity: 0.85, visible: false });
}

export function setCoverage(layer: ImageLayer<Static>, cov: CoverageResult | null): void {
	if (!cov) {
		layer.setVisible(false);
		return;
	}
	// imageCoords is an axis-aligned lon/lat bbox [TL, TR, BR, BL]; Web Mercator
	// keeps it axis-aligned, so derive the extent from opposite corners.
	const c = cov.imageCoords;
	const [minX, minY] = fromLonLat([c[3][0], c[3][1]]); // BL
	const [maxX, maxY] = fromLonLat([c[1][0], c[1][1]]); // TR
	layer.setSource(
		new Static({ url: cov.dataUrl, imageExtent: [minX, minY, maxX, maxY], interpolate: true })
	);
	layer.setVisible(true);
}

// A teal map pin as a DOM overlay (crisp, WebGL-free) anchored at a lon/lat.
export function markerPin(color = '#34e3c4'): { element: HTMLElement; overlay: Overlay } {
	const element = document.createElement('div');
	element.className = 'ol-pin';
	element.innerHTML =
		`<svg width="22" height="30" viewBox="0 0 22 30" xmlns="http://www.w3.org/2000/svg">` +
		`<path d="M11 0C4.9 0 0 4.9 0 11c0 7.7 11 19 11 19s11-11.3 11-19C22 4.9 17.1 0 11 0z" ` +
		`fill="${color}" stroke="rgba(0,0,0,.35)" stroke-width="1"/>` +
		`<circle cx="11" cy="11" r="4" fill="rgba(0,0,0,.45)"/></svg>`;
	const overlay = new Overlay({ element, positioning: 'bottom-center', stopEvent: false, offset: [0, 2] });
	return { element, overlay };
}

export function lonLat(lng: number, lat: number): [number, number] {
	return fromLonLat([lng, lat]) as [number, number];
}
