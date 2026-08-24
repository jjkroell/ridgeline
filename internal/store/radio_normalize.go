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
