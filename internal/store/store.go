// Package store is ridgelined's SQLite persistence layer. A single writer (the
// ingest loop) holds the write mutex; API reads run concurrently against the
// same WAL-mode database.
package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS nodes (
	pubkey       TEXT PRIMARY KEY,
	name         TEXT,
	role         TEXT,
	latitude     REAL,
	longitude    REAL,
	has_location INTEGER NOT NULL DEFAULT 0,
	first_seen   TEXT NOT NULL,
	last_seen    TEXT NOT NULL,
	last_advert  TEXT,
	advert_count INTEGER NOT NULL DEFAULT 0,
	advert_tx_count INTEGER NOT NULL DEFAULT 0,
	hash_size    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS observers (
	id           TEXT PRIMARY KEY,
	region       TEXT,
	pubkey       TEXT,
	first_seen   TEXT NOT NULL,
	last_seen    TEXT NOT NULL,
	packet_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS observations (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	message_hash TEXT NOT NULL,
	raw_hex      TEXT NOT NULL,
	route_type   TEXT NOT NULL,
	payload_type TEXT NOT NULL,
	path_hops    INTEGER NOT NULL DEFAULT 0,
	observer_id  TEXT,
	region       TEXT,
	snr          REAL,
	rssi         REAL,
	received_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_obs_hash ON observations(message_hash);
CREATE INDEX IF NOT EXISTS idx_obs_received ON observations(received_at DESC);

-- observer_telemetry is an append-only time series of each observer's
-- self-reported device telemetry. The observers row only keeps the LATEST
-- /status (overwritten in place); this log preserves history so battery/noise/
-- airtime can be trended. Only the non-reconstructable device fields live here
-- (radio config is static and stays on the observers row).
CREATE TABLE IF NOT EXISTS observer_telemetry (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	observer_id  TEXT NOT NULL,
	recorded_at  TEXT NOT NULL,
	battery_mv   INTEGER,
	uptime_secs  INTEGER,
	noise_floor  REAL,
	tx_air_secs  REAL,
	rx_air_secs  REAL,
	recv_errors  INTEGER,
	queue_len    INTEGER
);
CREATE INDEX IF NOT EXISTS idx_tel_obs_time ON observer_telemetry(observer_id, recorded_at DESC);

-- blocklist holds nodes/observers/bridges an admin has quarantined as injected
-- traffic (RF bridge or rogue MQTT publisher). Entries drop matching data at
-- ingest and hide it from the API; purging additionally hard-deletes stored rows.
CREATE TABLE IF NOT EXISTS blocklist (
	kind       TEXT NOT NULL,            -- observer | bridge | node | allow (allow = dismissed candidate)
	key        TEXT NOT NULL,            -- observer id, or node/bridge pubkey
	name       TEXT,                     -- friendly label captured at block time
	reason     TEXT,
	created_at TEXT NOT NULL,
	PRIMARY KEY (kind, key)
);
`

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
	mu sync.Mutex // serializes writes (single-writer model)

	// needAdvertTxBackfill is set on open when the advert_tx_count column was
	// just added, so a caller can seed it once from history.
	needAdvertTxBackfill bool

	// Blocklist cache, consulted on the hot ingest path. Guarded separately
	// from mu so reads don't contend with writes. Refreshed from the table on
	// open and after every mutation.
	blockMu          sync.RWMutex
	blockedObservers map[string]bool // observer id (exact)
	blockedNodes     map[string]bool // node/bridge pubkey (UPPER) — origin-advert block
	blockedBridges   []string        // bridge pubkeys (UPPER) — path-prefix block
	allowedNodes     map[string]bool // node pubkey (UPPER) — dismissed detection candidates
}

// Open opens (creating if needed) the SQLite database at path, enables WAL
// mode, and applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// Pin to a single connection. SQLite pragmas (busy_timeout, WAL) are
	// per-connection, so with database/sql's default pool the timeout never
	// reaches the extra connections it opens for concurrent queries — which is
	// what produced spurious SQLITE_BUSY errors. One connection serializes all
	// access (writes are already single-writer via s.mu, and the workload is
	// low-volume), and guarantees every statement runs with the pragmas below.
	db.SetMaxOpenConns(1)
	// WAL keeps reads fast; busy_timeout is belt-and-suspenders for any retry.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("store: %s: %w", pragma, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	// Lightweight migrations for databases created before a column existed.
	// Errors are expected (and ignored) when the column is already present.
	db.Exec(`ALTER TABLE observers ADD COLUMN pubkey TEXT`)
	db.Exec(`ALTER TABLE nodes ADD COLUMN hash_size INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE observers ADD COLUMN status_json TEXT`)
	db.Exec(`ALTER TABLE observers ADD COLUMN last_status_at TEXT`)
	db.Exec(`ALTER TABLE observers ADD COLUMN radio TEXT`)
	db.Exec(`ALTER TABLE nodes ADD COLUMN radio TEXT`)
	// advert_tx_count counts actual advert *transmissions* (re-flood/multi-observer
	// copies of one advert collapsed by a 30s gap), vs advert_count which counts
	// raw observations. If the column is new on an existing DB, flag a one-time
	// backfill from the stored observation history.
	needAdvertTxBackfill := !columnExists(db, "nodes", "advert_tx_count")
	db.Exec(`ALTER TABLE nodes ADD COLUMN advert_tx_count INTEGER NOT NULL DEFAULT 0`)
	s := &Store{db: db, needAdvertTxBackfill: needAdvertTxBackfill}
	if err := s.loadBlocklist(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: load blocklist: %w", err)
	}
	return s, nil
}

// UpsertObserverStatus records an observer's latest self-reported status (radio
// config + device telemetry) from its /status message, creating the observer row
// if a status arrives before any packet. statusJSON is the marshalled
// ObserverStatus; receivedAt is the server's receipt time (RFC3339).
func (s *Store) UpsertObserverStatus(id, region, pubkey, statusJSON, radio, receivedAt string) error {
	_, err := s.db.Exec(`
		INSERT INTO observers (id, region, pubkey, first_seen, last_seen, packet_count, status_json, last_status_at, radio)
		VALUES (?,?,?,?,?,0,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status_json    = excluded.status_json,
			last_status_at = excluded.last_status_at,
			radio          = COALESCE(NULLIF(excluded.radio,''), observers.radio),
			region         = COALESCE(NULLIF(excluded.region,''), observers.region),
			pubkey         = COALESCE(NULLIF(excluded.pubkey,''), observers.pubkey)`,
		id, region, pubkey, receivedAt, receivedAt, statusJSON, receivedAt, radio)
	return err
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Observation is one observer's sighting of one packet, ready to persist.
type Observation struct {
	Packet         *meshcore.Packet
	RawHex         string // full raw packet hex as received
	ObserverID     string
	ObserverPubkey string // observer node public key (origin_id), for geo-locating
	Region         string
	SNR            *float64
	RSSI           *float64
	ReceivedAt     time.Time // server ingest time — authoritative for ordering
}

// Record persists an observation: it inserts the observation row, updates the
// observer, and (for Adverts) upserts the announcing node.
func (s *Store) Record(o Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ts := o.ReceivedAt.UTC().Format(time.RFC3339Nano)
	p := o.Packet

	rawHex := o.RawHex
	if rawHex == "" {
		rawHex = p.PayloadRaw
	}
	if _, err := tx.Exec(`
		INSERT INTO observations
			(message_hash, raw_hex, route_type, payload_type, path_hops,
			 observer_id, region, snr, rssi, received_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		p.MessageHash, rawHex, p.RouteType.String(), p.PayloadType.String(),
		p.PathHopCount, nullStr(o.ObserverID), nullStr(o.Region),
		o.SNR, o.RSSI, ts,
	); err != nil {
		return fmt.Errorf("store: insert observation: %w", err)
	}

	if o.ObserverID != "" {
		if _, err := tx.Exec(`
			INSERT INTO observers (id, region, pubkey, first_seen, last_seen, packet_count)
			VALUES (?,?,?,?,?,1)
			ON CONFLICT(id) DO UPDATE SET
				last_seen    = excluded.last_seen,
				region       = COALESCE(NULLIF(excluded.region,''), observers.region),
				pubkey       = COALESCE(NULLIF(excluded.pubkey,''), observers.pubkey),
				packet_count = observers.packet_count + 1`,
			o.ObserverID, nullStr(o.Region), nullStr(o.ObserverPubkey), ts, ts,
		); err != nil {
			return fmt.Errorf("store: upsert observer: %w", err)
		}
	}

	if a := p.Advert; a != nil && a.PublicKey != "" {
		var lat, lon interface{}
		if a.HasLocation {
			lat, lon = a.Latitude, a.Longitude
		}
		// A node inherits the radio config of the observer that heard it — its
		// own freq/bw/sf/cr aren't in the packet (they're a PHY setting), but the
		// observing observer reports them via /status. Prep for network filtering.
		var observerRadio string
		if o.ObserverID != "" {
			tx.QueryRow(`SELECT COALESCE(radio,'') FROM observers WHERE id = ?`, o.ObserverID).Scan(&observerRadio)
		}
		// Count actual advert transmissions, not raw observations: re-floods and
		// multi-observer copies of one advert all arrive within a few seconds, so
		// this counts a new transmission only when it lands >30s after the node's
		// previous advert. (Mirrors analytics.summarizeAdverts.)
		txInc := advertTxIncrement(tx, a.PublicKey, o.ReceivedAt)
		// The advert's path-length byte carries the originating node's own
		// hash size (1, 2, or 3 bytes) — the length of the key prefix by which
		// this node is identified in packet paths.
		if _, err := tx.Exec(`
			INSERT INTO nodes
				(pubkey, name, role, latitude, longitude, has_location,
				 first_seen, last_seen, last_advert, advert_count, advert_tx_count, hash_size, radio)
			VALUES (?,?,?,?,?,?,?,?,?,1,?,?,?)
			ON CONFLICT(pubkey) DO UPDATE SET
				name         = COALESCE(NULLIF(excluded.name,''), nodes.name),
				role         = excluded.role,
				latitude     = COALESCE(excluded.latitude, nodes.latitude),
				longitude    = COALESCE(excluded.longitude, nodes.longitude),
				has_location = nodes.has_location | excluded.has_location,
				last_seen    = excluded.last_seen,
				last_advert  = excluded.last_advert,
				advert_count = nodes.advert_count + 1,
				advert_tx_count = nodes.advert_tx_count + ?,
				-- Only set hash_size from an advert while it's still unknown. A
				-- single advert with a corrupt path-length byte must not flip an
				-- established size; the periodic consensus pass (analytics) owns
				-- corrections from there, voting over many adverts.
				hash_size    = CASE WHEN nodes.hash_size = 0 THEN excluded.hash_size ELSE nodes.hash_size END,
				radio        = COALESCE(NULLIF(excluded.radio,''), nodes.radio)`,
			a.PublicKey, nullStr(a.Name), a.DeviceRole.String(),
			lat, lon, boolInt(a.HasLocation), ts, ts, ts, txInc, p.PathHashSize, nullStr(observerRadio),
			txInc,
		); err != nil {
			return fmt.Errorf("store: upsert node: %w", err)
		}
	}

	return tx.Commit()
}

// advertTxGap is how far apart two adverts must land to count as separate
// transmissions; closer ones are re-flood / multi-observer copies of the same
// broadcast (late reflood through distant hops can trail the first copy by a
// minute). Keep in sync with analytics.advertTxGap (backfill + cadence grouping).
const advertTxGap = 90 * time.Second

// advertTxIncrement returns 1 when this advert (at receivedAt) starts a new
// transmission for the node — a brand-new node, or an advert landing more than
// advertTxGap after the node's previous one — and 0 when it's just another
// observation (re-flood / different observer) of the current transmission.
func advertTxIncrement(tx *sql.Tx, pubkey string, receivedAt time.Time) int {
	var prev string
	tx.QueryRow(`SELECT COALESCE(last_advert,'') FROM nodes WHERE pubkey = ?`, pubkey).Scan(&prev)
	if prev == "" {
		return 1
	}
	pt, err := time.Parse(time.RFC3339Nano, prev)
	if err != nil {
		return 1
	}
	if receivedAt.UTC().Sub(pt) > advertTxGap {
		return 1
	}
	return 0
}

// columnExists reports whether a table already has a column (used to gate
// one-time backfills on a freshly-migrated column).
func columnExists(db *sql.DB, table, col string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == col {
			return true
		}
	}
	return false
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
