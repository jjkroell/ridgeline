// Node markers + clustering for the full maps, in OpenLayers (2D canvas, no
// WebGL). A Cluster source groups nearby points; the style function renders
// either a teal count bubble (clusters) or a role-coloured dot with an optional
// amber favourite ring (single nodes), with zoom-scaled radii matching the old
// MapLibre paint expressions.
import VectorLayer from 'ol/layer/Vector';
import VectorSource from 'ol/source/Vector';
import Cluster from 'ol/source/Cluster';
import Feature from 'ol/Feature';
import Point from 'ol/geom/Point';
import { Style, Circle as CircleStyle, Fill, Stroke, Text } from 'ol/style';
import { fromLonLat } from 'ol/proj';
import type { FeatureLike } from 'ol/Feature';
import type { Node } from '$lib/api';
import { roleLabel } from '$lib/format';
import { ROLE_HEX, FAV_COLOR, inkColor } from '$lib/map-util';

// Standard Web-Mercator zoom from an OL resolution (resolution is metres/px at
// the equator for EPSG:3857, so this is the slippy-map zoom level).
export function zoomFromResolution(resolution: number): number {
	return Math.log2(156543.03392804097 / resolution);
}

function lerp(zoom: number, stops: [number, number][]): number {
	if (zoom <= stops[0][0]) return stops[0][1];
	for (let i = 1; i < stops.length; i++) {
		if (zoom <= stops[i][0]) {
			const [z0, v0] = stops[i - 1];
			const [z1, v1] = stops[i];
			return v0 + ((v1 - v0) * (zoom - z0)) / (z1 - z0);
		}
	}
	return stops[stops.length - 1][1];
}

const REP_RADII: [number, number][] = [[6, 2.5], [11, 5], [15, 8]];
const OTHER_RADII: [number, number][] = [[6, 1.8], [11, 3.2], [15, 5.5]];
const nodeRadius = (role: string, zoom: number) =>
	lerp(zoom, role === 'Repeater' ? REP_RADII : OTHER_RADII);

const abbrev = (n: number) => (n < 1000 ? String(n) : (n / 1000).toFixed(1) + 'k');

function clusterStyle(count: number): Style {
	const radius = count >= 30 ? 24 : count >= 10 ? 18 : 13;
	return new Style({
		image: new CircleStyle({
			radius,
			fill: new Fill({ color: 'rgba(21,158,139,0.85)' }),
			stroke: new Stroke({ color: '#34e3c4', width: 1.5 })
		}),
		text: new Text({
			text: abbrev(count),
			font: '600 12px Space Mono, monospace',
			fill: new Fill({ color: '#04140f' })
		})
	});
}

function nodeStyle(member: FeatureLike, zoom: number): Style[] {
	const role = member.get('role') as string;
	const r = nodeRadius(role, zoom);
	const dot = new Style({
		image: new CircleStyle({
			radius: r,
			fill: new Fill({ color: member.get('color') as string }),
			stroke: new Stroke({ color: inkColor(), width: 1 })
		})
	});
	if (!member.get('fav')) return [dot];
	// Amber ring hugging the dot, drawn underneath it (array order = paint order).
	const halo = new Style({
		image: new CircleStyle({ radius: r + 2.5, stroke: new Stroke({ color: FAV_COLOR, width: 1.75 }) })
	});
	return [halo, dot];
}

export interface NodeLayerHandle {
	layer: VectorLayer;
	setNodes: (nodes: Node[], favHas: (pk: string) => boolean) => void;
}

export function createNodeLayer(opts?: { cluster?: boolean }): NodeLayerHandle {
	const source = new VectorSource();
	// Desktop clusters nearby nodes; mobile shows flat dots (its old behaviour).
	const layerSource = opts?.cluster === false ? source : new Cluster({ distance: 46, source });
	const layer = new VectorLayer({
		properties: { nodes: true },
		source: layerSource,
		style: (feature, resolution) => {
			const members = feature.get('features') as FeatureLike[] | undefined;
			if (!members) return nodeStyle(feature, zoomFromResolution(resolution)); // flat (mobile)
			if (members.length > 1) return clusterStyle(members.length);
			return nodeStyle(members[0], zoomFromResolution(resolution));
		}
	});

	return {
		layer,
		setNodes(nodes, favHas) {
			source.clear();
			source.addFeatures(
				nodes.map((n) => {
					const f = new Feature({ geometry: new Point(fromLonLat([n.longitude!, n.latitude!])) });
					f.setProperties(
						{
							role: n.role,
							color: ROLE_HEX[n.role] ?? '#8394a1',
							name: n.name || n.publicKey.slice(0, 10),
							roleLabel: roleLabel(n.role),
							pubkey: n.publicKey,
							fav: favHas(n.publicKey)
						},
						true
					);
					return f;
				})
			);
		}
	};
}

// Draggable amber transmitter pin for coverage mode. The page wires a Translate
// interaction over this layer and recomputes on drag end.
export interface PinLayerHandle {
	layer: VectorLayer;
	setPin: (lon: number, lat: number) => void;
	clear: () => void;
}

export function createPinLayer(): PinLayerHandle {
	const source = new VectorSource();
	const feature = new Feature({ geometry: new Point([0, 0]) });
	const layer = new VectorLayer({
		properties: { pin: true },
		source,
		style: new Style({
			image: new CircleStyle({
				radius: 7,
				fill: new Fill({ color: FAV_COLOR }),
				stroke: new Stroke({ color: 'rgba(0,0,0,.45)', width: 2 })
			})
		})
	});
	return {
		layer,
		setPin(lon, lat) {
			(feature.getGeometry() as Point).setCoordinates(fromLonLat([lon, lat]));
			if (!source.getFeatures().length) source.addFeature(feature);
		},
		clear() {
			source.clear();
		}
	};
}
