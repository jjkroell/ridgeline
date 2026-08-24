package store

import (
	"database/sql"

	"github.com/jjkroell/ridgeline/internal/radio"
)

// normalizeStoredRadio rewrites every stored radio config to kHz precision, so
// the single channel that arrives as both "910.4249877" and "910.425" is one
// value everywhere it is grouped, compared or displayed.
//
// Run once at open and idempotent: a normalized string normalizes to itself, and
// only rows that actually change are written. It covers both tables because
// nodes.radio is copied from observers.radio at ingest, so a database opened
// before normalization holds the un-rounded form in both.
func normalizeStoredRadio(db *sql.DB) error {
	for _, t := range []struct{ table, key string }{
		{"observers", "id"},
		{"nodes", "pubkey"},
	} {
		rows, err := db.Query(`SELECT ` + t.key + `, radio FROM ` + t.table + ` WHERE radio IS NOT NULL AND radio <> ''`)
		if err != nil {
			return err
		}
		type fix struct{ key, radio string }
		var fixes []fix
		for rows.Next() {
			var key, r string
			if err := rows.Scan(&key, &r); err != nil {
				rows.Close()
				return err
			}
			if n := radio.Normalize(r); n != r {
				fixes = append(fixes, fix{key, n})
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, f := range fixes {
			if _, err := db.Exec(`UPDATE `+t.table+` SET radio = ? WHERE `+t.key+` = ?`, f.radio, f.key); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearMisattributedRadio removes node radio values that a far-side receiver
// wrote onto near-side nodes.
//
// Before radio inheritance was restricted to zero-hop receptions, a receiver on
// a bridge's far segment stamped its own config onto every node whose advert
// reached it — including the whole near-side mesh, whose flood traffic crosses
// the bridge and arrives there five to eight hops deep. Those values are
// provably wrong rather than merely doubtful: a node NOT on the far segment
// cannot be transmitting on the far segment's channel, so the only way it
// acquired that config was by being overheard from the wrong side.
//
// One-time and idempotent. It does not need to run again, because the ingest
// rule that produced these values is gone: a relayed copy no longer writes a
// radio at all. Nodes cleared here recover a real value the next time a receiver
// hears them directly, and stay blank until then — which is the honest state,
// since nothing has actually measured their PHY.
//
// The reverse case (a far-side node carrying a near-side receiver's config) is
// left alone: the API already blanks radio for far-side nodes and reports the
// bridge's declared value instead.
func (s *Store) clearMisattributedRadio() (int, error) {
	links, err := s.KnownBridgeLinks()
	if err != nil {
		return 0, err
	}
	cleared := 0
	for _, l := range links {
		if l.PeerRadio == "" {
			continue // nothing declared for the far side: nothing to attribute
		}
		rows, err := s.db.Query(
			`SELECT pubkey, radio FROM nodes WHERE radio IS NOT NULL AND radio <> '' AND via_bridge IS NULL`)
		if err != nil {
			return cleared, err
		}
		var wrong []string
		for rows.Next() {
			var pubkey, r string
			if err := rows.Scan(&pubkey, &r); err != nil {
				rows.Close()
				return cleared, err
			}
			if radio.SameSegmentString(r, l.PeerRadio) {
				wrong = append(wrong, pubkey)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return cleared, err
		}
		rows.Close()
		for _, pubkey := range wrong {
			if _, err := s.db.Exec(`UPDATE nodes SET radio = NULL WHERE pubkey = ?`, pubkey); err != nil {
				return cleared, err
			}
			cleared++
		}
	}
	return cleared, nil
}

// MisattributedRadioCleared reports how many node radio values were dropped at
// open by clearMisattributedRadio. Zero on every subsequent start.
func (s *Store) MisattributedRadioCleared() int { return s.misattributedRadioCleared }
