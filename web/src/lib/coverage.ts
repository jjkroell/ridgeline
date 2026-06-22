// Terrain-based RF coverage prediction, computed client-side from the AWS Open
// Data "terrarium" elevation tiles (the same DEM used for the map hillshade).
//
// v1 model: terrain line-of-sight from the transmitter (ground + antenna height)
// to a receiver (ground + rx height), corrected for earth curvature with the
// standard 4/3 effective-earth radius. This is the dominant factor for VHF/UHF
// mesh coverage. A future upgrade swaps the visibility test for an ITM/
// Longley-Rice path-loss model (public-domain NTIA core) without changing this
// module's interface.
import type { Feature, Polygon } from 'geojson';

const TILE = 256;
const DEM_URL = (z: number, x: number, y: number) =>
	`https://s3.amazonaws.com/elevation-tiles-prod/terrarium/${z}/${x}/${y}.png`;
const EARTH_R = 6371008.8; // mean earth radius (m)
const EFF_EARTH_R = (4 / 3) * EARTH_R; // 4/3-earth for standard atmospheric refraction

function lonToTileX(lon: number, z: number): number {
	return ((lon + 180) / 360) * 2 ** z;
}
function latToTileY(lat: number, z: number): number {
	const r = (lat * Math.PI) / 180;
	return ((1 - Math.log(Math.tan(r) + 1 / Math.cos(r)) / Math.PI) / 2) * 2 ** z;
}

/** Loads + caches terrarium DEM tiles and samples elevation (metres) at lon/lat. */
export class DemSampler {
	readonly z: number;
	private tiles = new Map<string, Float32Array | null>();
	private pending = new Map<string, Promise<void>>();

	constructor(z = 12) {
		this.z = z;
	}

	private key(x: number, y: number) {
		return `${x}/${y}`;
	}

	private async loadTile(x: number, y: number): Promise<void> {
		const k = this.key(x, y);
		if (this.tiles.has(k)) return;
		if (this.pending.has(k)) return this.pending.get(k);
		const p = new Promise<void>((resolve) => {
			const img = new Image();
			img.crossOrigin = 'anonymous';
			img.onload = () => {
				try {
					const c = document.createElement('canvas');
					c.width = c.height = TILE;
					const ctx = c.getContext('2d', { willReadFrequently: true })!;
					ctx.drawImage(img, 0, 0);
					const d = ctx.getImageData(0, 0, TILE, TILE).data;
					const elev = new Float32Array(TILE * TILE);
					for (let i = 0; i < TILE * TILE; i++) {
						const r = d[i * 4],
							g = d[i * 4 + 1],
							b = d[i * 4 + 2];
						elev[i] = r * 256 + g + b / 256 - 32768;
					}
					this.tiles.set(k, elev);
				} catch {
					this.tiles.set(k, null); // tainted/decoded failure → treat as no data
				}
				resolve();
			};
			img.onerror = () => {
				this.tiles.set(k, null);
				resolve();
			};
			img.src = DEM_URL(this.z, x, y);
		});
		this.pending.set(k, p);
		return p;
	}

	/** Pre-fetch every tile covering the given bbox [minLon, minLat, maxLon, maxLat]. */
	async ensure(minLon: number, minLat: number, maxLon: number, maxLat: number): Promise<void> {
		const x0 = Math.floor(lonToTileX(minLon, this.z));
		const x1 = Math.floor(lonToTileX(maxLon, this.z));
		const y0 = Math.floor(latToTileY(maxLat, this.z)); // note: y grows southward
		const y1 = Math.floor(latToTileY(minLat, this.z));
		const jobs: Promise<void>[] = [];
		for (let x = x0; x <= x1; x++) for (let y = y0; y <= y1; y++) jobs.push(this.loadTile(x, y));
		await Promise.all(jobs);
	}

	/** Elevation in metres at lon/lat (bilinear within the containing tile), or NaN. */
	elev(lon: number, lat: number): number {
		const fx = lonToTileX(lon, this.z);
		const fy = latToTileY(lat, this.z);
		const tx = Math.floor(fx),
			ty = Math.floor(fy);
		const grid = this.tiles.get(this.key(tx, ty));
		if (!grid) return NaN;
		const px = (fx - tx) * TILE,
			py = (fy - ty) * TILE;
		const x0 = Math.min(TILE - 1, Math.max(0, Math.floor(px)));
		const y0 = Math.min(TILE - 1, Math.max(0, Math.floor(py)));
		const x1 = Math.min(TILE - 1, x0 + 1),
			y1 = Math.min(TILE - 1, y0 + 1);
		const dx = px - x0,
			dy = py - y0;
		const e = (xx: number, yy: number) => grid[yy * TILE + xx];
		return (
			e(x0, y0) * (1 - dx) * (1 - dy) +
			e(x1, y0) * dx * (1 - dy) +
			e(x0, y1) * (1 - dx) * dy +
			e(x1, y1) * dx * dy
		);
	}
}

// Forward geodesic: point at distance d (m) and bearing brng (deg) from lat/lon.
function destination(lat: number, lon: number, brng: number, d: number): [number, number] {
	const R = EARTH_R;
	const δ = d / R;
	const θ = (brng * Math.PI) / 180;
	const φ1 = (lat * Math.PI) / 180,
		λ1 = (lon * Math.PI) / 180;
	const φ2 = Math.asin(Math.sin(φ1) * Math.cos(δ) + Math.cos(φ1) * Math.sin(δ) * Math.cos(θ));
	const λ2 =
		λ1 +
		Math.atan2(
			Math.sin(θ) * Math.sin(δ) * Math.cos(φ1),
			Math.cos(δ) - Math.sin(φ1) * Math.sin(φ2)
		);
	return [(λ2 * 180) / Math.PI, (φ2 * 180) / Math.PI];
}

export interface CoverageParams {
	lat: number;
	lon: number;
	txHeightM: number; // transmitter antenna height above ground
	rxHeightM: number; // assumed receiver antenna height above ground
	maxRangeKm: number;
	azimuths?: number; // ray count (default 360)
}

export interface CoverageResult {
	center: [number, number]; // [lon, lat]
	groundElevM: number;
	polygon: Feature<Polygon>;
	rangesKm: number[]; // max visible range per azimuth (km)
	maxRangeKm: number;
}

/** Compute a terrain line-of-sight coverage polygon for a transmitter. */
export async function computeCoverage(
	p: CoverageParams,
	onProgress?: (frac: number) => void
): Promise<CoverageResult> {
	const az = p.azimuths ?? 360;
	const maxRange = p.maxRangeKm * 1000;
	const sampler = new DemSampler(12);

	// pad the bbox by ~maxRange around the pin
	const dLat = (maxRange / EARTH_R) * (180 / Math.PI);
	const dLon = dLat / Math.cos((p.lat * Math.PI) / 180);
	await sampler.ensure(p.lon - dLon, p.lat - dLat, p.lon + dLon, p.lat + dLat);

	const ground = sampler.elev(p.lon, p.lat);
	const obs = (Number.isFinite(ground) ? ground : 0) + p.txHeightM;

	// step ≈ one DEM pixel on the ground at this latitude
	const step = Math.max(20, (Math.cos((p.lat * Math.PI) / 180) * 40075016.7) / (2 ** sampler.z * TILE));

	const ring: [number, number][] = [];
	const ranges: number[] = [];
	for (let a = 0; a < az; a++) {
		const brng = (a * 360) / az;
		let maxAng = -Infinity;
		let maxVis = 0;
		for (let r = step; r <= maxRange; r += step) {
			const [lon, lat] = destination(p.lat, p.lon, brng, r);
			const terr = sampler.elev(lon, lat);
			if (!Number.isFinite(terr)) continue;
			const drop = (r * r) / (2 * EFF_EARTH_R); // earth-curvature drop
			const effTerr = terr - drop;
			const rxAng = Math.atan2(effTerr + p.rxHeightM - obs, r);
			if (rxAng >= maxAng) maxVis = r; // receiver clears the horizon
			const terrAng = Math.atan2(effTerr - obs, r);
			if (terrAng > maxAng) maxAng = terrAng; // raise the horizon for farther points
		}
		const rr = maxVis > 0 ? maxVis : step * 0.5;
		ring.push(destination(p.lat, p.lon, brng, rr));
		ranges.push(rr / 1000);
		if (onProgress && a % 20 === 0) onProgress(a / az);
	}
	ring.push(ring[0]);

	return {
		center: [p.lon, p.lat],
		groundElevM: ground,
		polygon: {
			type: 'Feature',
			properties: {},
			geometry: { type: 'Polygon', coordinates: [ring] }
		},
		rangesKm: ranges,
		maxRangeKm: p.maxRangeKm
	};
}

/** Ray-casting point-in-polygon for the (single-ring) coverage polygon. */
export function inCoverage(cov: CoverageResult, lon: number, lat: number): boolean {
	const ring = cov.polygon.geometry.coordinates[0];
	let inside = false;
	for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
		const [xi, yi] = ring[i];
		const [xj, yj] = ring[j];
		if (yi > lat !== yj > lat && lon < ((xj - xi) * (lat - yi)) / (yj - yi) + xi) inside = !inside;
	}
	return inside;
}

/** Great-circle distance in km. */
export function distKm(lat1: number, lon1: number, lat2: number, lon2: number): number {
	const rad = Math.PI / 180;
	const dLat = (lat2 - lat1) * rad,
		dLon = (lon2 - lon1) * rad;
	const a =
		Math.sin(dLat / 2) ** 2 +
		Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2;
	return EARTH_R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a)) / 1000;
}
