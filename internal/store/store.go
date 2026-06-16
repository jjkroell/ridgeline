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
`

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
	mu sync.Mutex // serializes writes (single-writer model)
}

// Open opens (creating if needed) the SQLite database at path, enables WAL
// mode, and applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// WAL allows concurrent readers alongside the single writer; busy_timeout
	// avoids spurious SQLITE_BUSY under brief contention.
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
	return &Store{db: db}, nil
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
		// The advert's path-length byte carries the originating node's own
		// hash size (1, 2, or 3 bytes) — the length of the key prefix by which
		// this node is identified in packet paths.
		if _, err := tx.Exec(`
			INSERT INTO nodes
				(pubkey, name, role, latitude, longitude, has_location,
				 first_seen, last_seen, last_advert, advert_count, hash_size)
			VALUES (?,?,?,?,?,?,?,?,?,1,?)
			ON CONFLICT(pubkey) DO UPDATE SET
				name         = COALESCE(NULLIF(excluded.name,''), nodes.name),
				role         = excluded.role,
				latitude     = COALESCE(excluded.latitude, nodes.latitude),
				longitude    = COALESCE(excluded.longitude, nodes.longitude),
				has_location = nodes.has_location | excluded.has_location,
				last_seen    = excluded.last_seen,
				last_advert  = excluded.last_advert,
				advert_count = nodes.advert_count + 1,
				hash_size    = excluded.hash_size`,
			a.PublicKey, nullStr(a.Name), a.DeviceRole.String(),
			lat, lon, boolInt(a.HasLocation), ts, ts, ts, p.PathHashSize,
		); err != nil {
			return fmt.Errorf("store: upsert node: %w", err)
		}
	}

	return tx.Commit()
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
