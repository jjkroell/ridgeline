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
	/** "freq,bw,sf,cr" config, inherited from the observer that heard it. */
	radio?: string;
	/** Most recent time this node relayed a packet (within the analytics window). */
	lastRelayed?: string;
	/** Packets this node relayed in the last hour. */
	relayCount1h?: number;
}

export interface ObserverStatus {
	state?: string; // online | offline
	radio?: string; // raw "freq,bw,sf,cr"
	freqMhz?: number;
	bandwidthKhz?: number;
	spreadingFactor?: number;
	codingRate?: number;
	model?: string;
	firmware?: string;
	clientVersion?: string;
	batteryMv?: number;
	uptimeSecs?: number;
	noiseFloor?: number;
	txAirSecs?: number;
	rxAirSecs?: number;
	recvErrors?: number;
	queueLen?: number;
}

export interface Observer {
	id: string;
	region: string;
	firstSeen: string;
	lastSeen: string;
	packetCount: number;
	/** Latest self-reported device telemetry from the observer's /status message. */
	status?: ObserverStatus;
	lastStatusAt?: string;
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

// --- Mesh-wide analytics (GET /api/mesh-analytics) ---
export interface RadioParams {
	SpreadingFactor: number;
	BandwidthHz: number;
	CodingRate: number;
	PreambleSymbols: number;
}
export interface MeshKPIs {
	activeNodes: number;
	transmissions: number;
	observations: number;
	avgLinkScore?: number;
	floodRedundancy?: number;
	channelUtilPct: number;
	congestionTier: string;
}
export interface NameCount {
	label: string;
	count: number;
}
export interface HistogramBin {
	label: string;
	count: number;
}
export interface AirtimeBucket {
	timestamp: string;
	airtimeMs: number;
	utilPct: number;
	transmissions: number;
}
export interface RelayRank {
	publicKey: string;
	name: string;
	role: string;
	relayed: number;
	airtimeMs: number;
}
export interface ObserverCoverage {
	id: string;
	region?: string;
	observations: number;
	distinctNodes: number;
	directNodes: number;
	/** Median receive-time deviation from consensus (ms) — clock-drift signal. */
	clockSkewMs?: number;
}
export interface DirectLink {
	observer: string;
	nodeKey: string;
	nodeName: string;
	role: string;
	count: number;
}
export interface MeshAnalytics {
	generatedAt: string;
	windowHours: number;
	radio: RadioParams;
	kpis: MeshKPIs;
	payloadTypes: NameCount[];
	routeTypes: NameCount[];
	linkScoreHist: HistogramBin[];
	snrHist: HistogramBin[];
	airtime: AirtimeBucket[];
	topRelays: RelayRank[];
	observers: ObserverCoverage[];
	directLinks: DirectLink[];
	directReach: HistogramBin[];
	hashSizes: NameCount[];
}

export interface NodeActivity {
	grid: number[][]; // [weekday 0=Sun][hour 0-23]
	max: number;
	total: number;
	days: number;
}

export interface ObserverAnalytics {
	id: string;
	region?: string;
	windowHours: number;
	totalPackets: number;
	packetsPerHour: number;
	activity: number[]; // per-hour receptions, oldest bucket first
	payloadTypes: NameCount[];
	snrHist: HistogramBin[];
	avgSnr?: number;
	distinctNodes: number;
	directNodes: number;
	clockSkewMs?: number;
	neighbors: DirectLink[];
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
	/** A node's weekday×hour activity heatmap over the last `days` days. */
	nodeHeatmap: (pubkey: string, days = 7) =>
		get<NodeActivity>(`/api/nodes/${encodeURIComponent(pubkey)}/heatmap?days=${days}`),
	observers: () => get<Observer[]>('/api/observers'),
	/** One observer's feed metrics over the last sinceSec seconds (default 24h, max 7d). */
	observerAnalytics: (id: string, sinceSec = 86400) =>
		get<ObserverAnalytics>(`/api/observers/${encodeURIComponent(id)}/analytics?since=${sinceSec}`),
	observations: (limit = 100) => get<Observation[]>(`/api/observations?limit=${limit}`),
	/** Recent history (default last hour) in the live-event shape, newest first. */
	recent: (sinceSec = 3600) => get<LiveEvent[]>(`/api/recent?since=${sinceSec}`),
	/** Mesh-wide analytics over the last sinceSec seconds (default 6h, max 24h). */
	meshAnalytics: (sinceSec = 21600, bucketMin = 10) =>
		get<MeshAnalytics>(`/api/mesh-analytics?since=${sinceSec}&bucket=${bucketMin}`)
};
