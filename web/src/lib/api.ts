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
	/** Actual advert transmissions (re-flood / multi-observer copies of one
	 *  broadcast collapsed by a ~90s gap) — vs advertCount which counts every
	 *  observation. */
	advertTxCount: number;
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
	/** Set when the node is quarantined as suspected injected traffic. */
	quarantined?: boolean;
	block?: BlockEntry;
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
	/** Transmissions in the slice that were relayed (≥1 hop). */
	relayTx: number;
	/** Mean per-reception link score in the slice (relay-health trend). */
	avgLinkScore?: number;
}
export interface TopologyNode {
	publicKey: string;
	name: string;
	role: string;
	relayed: number;
}
export interface TopologyEdge {
	a: string;
	b: string;
	weight: number;
}
export interface Topology {
	nodes: TopologyNode[];
	edges: TopologyEdge[];
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
	topology: Topology;
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

export interface TelemetryPoint {
	recordedAt: string;
	batteryMv?: number;
	uptimeSecs?: number;
	noiseFloor?: number;
	txAirSecs?: number;
	rxAirSecs?: number;
	recvErrors?: number;
	queueLen?: number;
}

export interface TelemetrySummary {
	samples: number;
	spanHours: number;
	batteryMv?: number;
	batteryTrendMvHr?: number;
	batteryDir?: string; // charging | discharging | stable
	reboots: number;
	noiseFloor?: number;
	noiseTrendDbHr?: number;
	noiseMin?: number;
	noiseMax?: number;
	noiseAvg?: number;
}

export interface ObserverTelemetry {
	id: string;
	points: TelemetryPoint[];
	summary: TelemetrySummary;
}

async function get<T>(path: string): Promise<T> {
	const res = await fetch(path, { headers: { accept: 'application/json' } });
	if (!res.ok) throw new Error(`${path}: ${res.status}`);
	return res.json() as Promise<T>;
}

// ---- Admin (auth-gated injection detection + quarantine/purge) ----

export interface ForeignNode {
	key: string;
	name: string;
	role?: string;
	latitude?: number;
	longitude?: number;
	transitPct?: number; // % of this node's observed paths through the candidate
	captive?: boolean; // transitPct >= 95% (no alternative route)
}
export interface BridgeCandidate {
	nodeKey: string;
	name: string;
	captiveCount: number; // foreign nodes ≥95% captive to this node
	foreignThrough: number; // foreign nodes routed through it at all
	captiveFraction: number; // captiveCount / foreignThrough
	foreignKm: number; // geographic displacement — shown as a hint, not ranked
	foreign: ForeignNode[];
}
export interface InjectorCandidate {
	observer: string;
	exclusiveCount: number;
	exclusive: ForeignNode[];
}
export interface InjectionReport {
	windowHours: number;
	bridges: BridgeCandidate[];
	injectors: InjectorCandidate[];
}
export interface BlockEntry {
	kind: string; // observer | bridge | node
	key: string;
	name?: string;
	reason?: string;
	createdAt: string;
}
export interface PurgeResult {
	observations: number;
	nodes: number;
}

async function adminReq<T>(token: string, path: string, method = 'GET', body?: unknown): Promise<T> {
	const res = await fetch(path, {
		method,
		headers: {
			accept: 'application/json',
			authorization: `Bearer ${token}`,
			...(body ? { 'content-type': 'application/json' } : {})
		},
		body: body ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		let msg = `${res.status}`;
		try {
			msg = (await res.json()).error ?? msg;
		} catch {
			/* ignore */
		}
		throw new Error(msg);
	}
	return res.json() as Promise<T>;
}

export const admin = {
	/** Validate the admin token; throws on failure. */
	check: (token: string) => adminReq<{ ok: boolean }>(token, '/api/admin/check'),
	detect: (token: string, sinceSec = 86400) =>
		adminReq<InjectionReport>(token, `/api/admin/detect?since=${sinceSec}`),
	blocklist: (token: string) => adminReq<BlockEntry[]>(token, '/api/admin/blocklist'),
	/** Quarantine (reversible): drop at ingest + hide; does not delete stored rows.
	 *  `nodes` optionally blocks extra node pubkeys (a bridge's foreign cluster).
	 *  kind "allow" dismisses a detection candidate without blocking it. */
	block: (
		token: string,
		body: { kind: string; key: string; name?: string; reason?: string; nodes?: string[] }
	) => adminReq<{ ok: boolean }>(token, '/api/admin/block', 'POST', body),
	unblock: (token: string, kind: string, key: string) =>
		adminReq<{ ok: boolean }>(
			token,
			`/api/admin/block?kind=${encodeURIComponent(kind)}&key=${encodeURIComponent(key)}`,
			'DELETE'
		),
	/** Purge: delete stored data; blocks the INGRESS points (bridges/observers)
	 *  but deletes `nodes` permanently with no block. */
	purge: (token: string, body: { observers?: string[]; bridges?: string[]; nodes?: string[] }) =>
		adminReq<PurgeResult>(token, '/api/admin/purge', 'POST', body),
	/** Permanently delete nodes (adverts + rows) with no blocklist entry. */
	deleteNodes: (token: string, nodes: string[]) =>
		adminReq<PurgeResult>(token, '/api/admin/delete', 'POST', { nodes })
};

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
	/** One observer's device-telemetry time series + derived health summary (default 24h, max 7d). */
	observerTelemetry: (id: string, sinceSec = 86400) =>
		get<ObserverTelemetry>(`/api/observers/${encodeURIComponent(id)}/telemetry?since=${sinceSec}`),
	observations: (limit = 100) => get<Observation[]>(`/api/observations?limit=${limit}`),
	/** Recent history (default last hour) in the live-event shape, newest first. */
	recent: (sinceSec = 3600) => get<LiveEvent[]>(`/api/recent?since=${sinceSec}`),
	/**
	 * Channel (GroupText) message history, newest first, one row per distinct
	 * message. Default & max 24h — for the channel chat reader.
	 */
	channelHistory: (sinceSec = 86400) =>
		get<LiveEvent[]>(`/api/channels/recent?since=${sinceSec}`),
	/** Mesh-wide analytics over the last sinceSec seconds (default 6h, max 24h). */
	meshAnalytics: (sinceSec = 21600, bucketMin = 10) =>
		get<MeshAnalytics>(`/api/mesh-analytics?since=${sinceSec}&bucket=${bucketMin}`)
};

// ---- Accounts / auth ----

export interface AuthUser {
	id: number;
	email: string;
	displayName: string;
	/** Site administrator: manages members and (later) moderation. */
	isAdmin: boolean;
	/** Admin-granted gate for claiming nodes and storing private locations. */
	canClaim: boolean;
	/** Suspended: cannot log in; existing sessions are void. */
	blocked: boolean;
	/** The protected initial admin — cannot be demoted, blocked, or removed. */
	isOwner: boolean;
	createdAt: string;
	lastLogin?: string;
}

/** Response from register/login/me: the user (null when signed out) plus the
 *  session's CSRF token, echoed on authenticated mutations via X-CSRF-Token. */
export interface AuthResponse {
	user: AuthUser | null;
	csrfToken?: string;
}

// Session cookies are HttpOnly and set by the server; same-origin fetches send
// them automatically, so the client never handles the session token directly.
async function authReq(path: string, body?: unknown): Promise<AuthResponse> {
	const res = await fetch(path, {
		method: 'POST',
		headers: { accept: 'application/json', ...(body ? { 'content-type': 'application/json' } : {}) },
		body: body ? JSON.stringify(body) : undefined
	});
	const data = (await res.json().catch(() => ({}))) as AuthResponse & { error?: string };
	if (!res.ok) throw new Error(data.error ?? `${res.status}`);
	return data;
}

export const authApi = {
	me: () => get<AuthResponse>('/api/auth/me'),
	register: (email: string, password: string, displayName: string) =>
		authReq('/api/auth/register', { email, password, displayName }),
	login: (email: string, password: string) => authReq('/api/auth/login', { email, password }),
	logout: () => authReq('/api/auth/logout')
};

// mutate is the shared helper for authenticated, CSRF-protected state changes
// (used by the account features). It relies on the same-origin session cookie
// and sends the session's CSRF token in the header (double-submit).
export async function mutate<T>(
	path: string,
	method: string,
	csrf: string,
	body?: unknown
): Promise<T> {
	const res = await fetch(path, {
		method,
		headers: {
			accept: 'application/json',
			'x-csrf-token': csrf,
			...(body ? { 'content-type': 'application/json' } : {})
		},
		body: body ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		let msg = `${res.status}`;
		try {
			msg = (await res.json()).error ?? msg;
		} catch {
			/* ignore */
		}
		throw new Error(msg);
	}
	return res.json() as Promise<T>;
}

// ---- Node ownership claims ----

export interface Claim {
	id: number;
	nodePubkey: string;
	userId: number;
	/** Verification code (present only on your own pending claim). */
	code?: string;
	status: 'pending' | 'verified';
	createdAt: string;
	expiresAt?: string;
	verifiedAt?: string;
}

export interface ClaimStatus {
	/** The verified owner (public), if any. */
	owner?: { userId: number; displayName: string };
	ownedByMe: boolean;
	/** The requesting user's own claim on this node, if any. */
	mine?: Claim;
	loggedIn: boolean;
	/** Whether the requester is allowed to start a claim on this node. */
	canClaim: boolean;
	/** True when you own the node but its advertised name still contains the
	 *  verification code — restore the real name and re-advert to clear it. */
	nameNeedsReset?: boolean;
}

export interface ClaimWithNode extends Claim {
	nodeName: string;
	nodeRole: string;
}

export const claims = {
	/** Public: ownership + the caller's own claim status for a node. */
	status: (pubkey: string) => get<ClaimStatus>(`/api/nodes/${encodeURIComponent(pubkey)}/claim`),
	/** Open or refresh a pending claim; returns the code to embed in the advert name. */
	create: (csrf: string, pubkey: string) => mutate<Claim>('/api/claims', 'POST', csrf, { pubkey }),
	/** Cancel a pending claim or release ownership. */
	release: (csrf: string, pubkey: string) =>
		mutate<{ ok: boolean }>(`/api/claims/${encodeURIComponent(pubkey)}`, 'DELETE', csrf),
	/** The caller's own claims (pending + owned) with node display info. */
	mine: () => get<ClaimWithNode[]>('/api/claims/mine')
};

// ---- Node notes ----

export interface Note {
	id: number;
	nodePubkey: string;
	userId: number;
	authorName: string;
	visibility: 'public' | 'private';
	body: string;
	createdAt: string;
	updatedAt: string;
	/** The requester may edit/delete this note (author, or owner/admin for delete). */
	mine: boolean;
}

export const notes = {
	/** Public notes + the caller's own private notes for a node, newest first. */
	list: (pubkey: string) => get<Note[]>(`/api/nodes/${encodeURIComponent(pubkey)}/notes`),
	create: (csrf: string, pubkey: string, body: string, visibility: 'public' | 'private') =>
		mutate<Note>(`/api/nodes/${encodeURIComponent(pubkey)}/notes`, 'POST', csrf, { body, visibility }),
	update: (csrf: string, id: number, body: string, visibility: 'public' | 'private') =>
		mutate<Note>(`/api/notes/${id}`, 'PATCH', csrf, { body, visibility }),
	remove: (csrf: string, id: number) => mutate<{ ok: boolean }>(`/api/notes/${id}`, 'DELETE', csrf)
};

/** Admin member management (session-admin gated). */
export const adminUsers = {
	list: () => get<AuthUser[]>('/api/admin/users'),
	setFlags: (csrf: string, id: number, isAdmin: boolean, canClaim: boolean) =>
		mutate<{ ok: boolean }>('/api/admin/users/flags', 'POST', csrf, { id, isAdmin, canClaim }),
	/** Suspend (blocked=true) or restore (blocked=false) an account. */
	setBlocked: (csrf: string, id: number, blocked: boolean) =>
		mutate<{ ok: boolean }>('/api/admin/users/block', 'POST', csrf, { id, blocked }),
	/** Permanently delete an account. */
	remove: (csrf: string, id: number) =>
		mutate<{ ok: boolean }>('/api/admin/users/delete', 'POST', csrf, { id })
};
