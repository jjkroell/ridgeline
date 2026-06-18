// Typed client for the ridgelined REST API.

export interface Stats {
	nodes: number;
	observers: number;
	observations: number;
	lastPacketAt?: string;
}

export interface Node {
	publicKey: string;
	name: string;
	role: string;
	latitude?: number;
	longitude?: number;
	hasLocation: boolean;
	firstSeen: string;
	lastSeen: string;
	lastAdvert?: string;
	advertCount: number;
	/** Path-hash length in bytes (1, 2, or 3), from the node's advert; 0 = unknown. */
	hashSize: number;
	/** Coordinates are a statistical outlier — likely corrupt GPS. */
	gpsSuspect?: boolean;
}

export interface Observer {
	id: string;
	region: string;
	firstSeen: string;
	lastSeen: string;
	packetCount: number;
}

export interface Observation {
	messageHash: string;
	routeType: string;
	payloadType: string;
	pathHops: number;
	observerId?: string;
	region?: string;
	snr?: number;
	rssi?: number;
	receivedAt: string;
}

export interface LiveNode {
	publicKey: string;
	name: string;
	role: string;
	latitude?: number;
	longitude?: number;
	timestamp?: number;
}

export interface LiveEvent extends Observation {
	node?: LiveNode;
	/** Per-hop relay key prefixes the packet accumulated as it flooded. */
	path?: string[];
	payloadVersion?: number;
	hashSize?: number;
	transportCodes?: [number, number];
	payloadRaw?: string;
	raw?: string;
	/** GroupText channel fields. channelHash is always set; the rest only when decrypted. */
	channelHash?: string;
	channel?: string;
	sender?: string;
	text?: string;
}

// --- Per-node analytics (GET /api/nodes/{pubkey}) ---
export interface NodeObserverStat {
	id: string;
	region?: string;
	count: number;
	avgSnr?: number;
	avgRssi?: number;
}
export interface NodeNeighbor {
	publicKey: string;
	name: string;
	role: string;
	count: number;
}
export interface NodePacketRef {
	messageHash: string;
	payloadType: string;
	receivedAt: string;
	observerId?: string;
	snr?: number;
	rssi?: number;
	pathHops: number;
}
export interface NodeRelay {
	lastRelayed?: string;
	count1h: number;
	count24h: number;
	active: boolean;
}
export interface NodeAnalytics {
	publicKey: string;
	windowHours: number;
	totalPackets: number; // advert transmissions in window
	totalObservations: number;
	packetsToday: number;
	avgSnr?: number;
	avgHops?: number;
	firstHeard?: string;
	lastHeard?: string;
	observers: NodeObserverStat[];
	recentPackets: NodePacketRef[];
	neighbors: NodeNeighbor[];
	relay: NodeRelay;
	trafficShare: number;
	bridge: number;
	/** Median seconds between the node's advert transmissions (heartbeat cadence). */
	advertIntervalSec?: number;
	/** Per-hour advert counts over the window, oldest bucket first. */
	activity: number[];
}
export interface NodeDetailResponse {
	node: Node | null;
	detail: NodeAnalytics | null;
	generatedAt?: string;
}

// One stored observation attributable to a node (GET /api/nodes/{pubkey}/history).
export interface NodeHistoryEntry {
	messageHash: string;
	payloadType: string;
	routeType: string;
	kind: 'advert' | 'relay';
	receivedAt: string;
	observerId?: string;
	region?: string;
	snr?: number;
	rssi?: number;
	pathHops: number;
	hopIndex: number;
}

async function get<T>(path: string): Promise<T> {
	const res = await fetch(path, { headers: { accept: 'application/json' } });
	if (!res.ok) throw new Error(`${path}: ${res.status}`);
	return res.json() as Promise<T>;
}

export const api = {
	stats: () => get<Stats>('/api/stats'),
	nodes: () => get<Node[]>('/api/nodes'),
	/** One node's row plus its computed analytics snapshot. */
	nodeDetail: (pubkey: string) => get<NodeDetailResponse>(`/api/nodes/${encodeURIComponent(pubkey)}`),
	/** A node's stored observations (own adverts + relayed packets) over the last sinceSec seconds, newest first. */
	nodeHistory: (pubkey: string, sinceSec = 86400, limit = 300) =>
		get<NodeHistoryEntry[]>(`/api/nodes/${encodeURIComponent(pubkey)}/history?since=${sinceSec}&limit=${limit}`),
	observers: () => get<Observer[]>('/api/observers'),
	observations: (limit = 100) => get<Observation[]>(`/api/observations?limit=${limit}`),
	/** Recent history (default last hour) in the live-event shape, newest first. */
	recent: (sinceSec = 3600) => get<LiveEvent[]>(`/api/recent?since=${sinceSec}`)
};
