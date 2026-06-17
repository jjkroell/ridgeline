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
}

async function get<T>(path: string): Promise<T> {
	const res = await fetch(path, { headers: { accept: 'application/json' } });
	if (!res.ok) throw new Error(`${path}: ${res.status}`);
	return res.json() as Promise<T>;
}

export const api = {
	stats: () => get<Stats>('/api/stats'),
	nodes: () => get<Node[]>('/api/nodes'),
	observers: () => get<Observer[]>('/api/observers'),
	observations: (limit = 100) => get<Observation[]>(`/api/observations?limit=${limit}`)
};
