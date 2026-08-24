package store

import (
	"path/filepath"
	"testing"
)

// One channel must not read as two. Before normalization the live database held
// both "910.4249877,62.5,7,5" and "910.425,62.5,7,5" — the same 910.425 MHz —
// which split one mesh into two groups everywhere radio was grouped or compared.
func TestNormalizeStoredRadio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "radio.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	seed := []struct{ id, radio string }{
		{"obs-synth", "910.4249877,62.5,7,5"},
		{"obs-round", "910.425,62.5,7,5"},
		{"obs-909", "909.0,62.5,8,5"},
		{"obs-none", ""},
	}
	for _, o := range seed {
		if _, err := st.db.Exec(
			`INSERT INTO observers (id, first_seen, last_seen, packet_count, radio) VALUES (?,?,?,0,?)`,
			o.id, "2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z", nullStr(o.radio)); err != nil {
			t.Fatalf("seed observer %s: %v", o.id, err)
		}
	}
	if _, err := st.db.Exec(
		`INSERT INTO nodes (pubkey, first_seen, last_seen, advert_count, radio) VALUES (?,?,?,1,?)`,
		"AABB", "2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z", "910.4249877,62.5,7,5"); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	st.Close()

	// Reopening runs the migration.
	st, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()

	var groups int
	if err := st.db.QueryRow(
		`SELECT COUNT(DISTINCT radio) FROM observers WHERE radio IS NOT NULL AND radio <> '' AND radio LIKE '910%'`,
	).Scan(&groups); err != nil {
		t.Fatal(err)
	}
	if groups != 1 {
		t.Errorf("910.425 still reads as %d different configs, want 1", groups)
	}

	var nodeRadio, obs909 string
	st.db.QueryRow(`SELECT radio FROM nodes WHERE pubkey = 'AABB'`).Scan(&nodeRadio)
	if nodeRadio != "910.425,62.5,7,5" {
		t.Errorf("nodes.radio = %q, want %q", nodeRadio, "910.425,62.5,7,5")
	}
	// An already-canonical value must survive untouched.
	st.db.QueryRow(`SELECT radio FROM observers WHERE id = 'obs-909'`).Scan(&obs909)
	if obs909 != "909.0,62.5,8,5" {
		t.Errorf("obs-909 radio = %q, want it unchanged", obs909)
	}
}

// Every write path must normalize, or the next status message reintroduces the
// un-rounded spelling the migration just cleaned up.
func TestObserverStatusNormalizesRadio(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "radio2.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	if err := st.UpsertObserverStatus("obs-1", "Obs One", "YVR", "", "{}", "910.4249877,62.5,7,5", "2026-08-01T00:00:00Z"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var got string
	st.db.QueryRow(`SELECT radio FROM observers WHERE id = 'obs-1'`).Scan(&got)
	if got != "910.425,62.5,7,5" {
		t.Errorf("after UpsertObserverStatus radio = %q, want normalized", got)
	}

	if _, err := st.UpdateObserverStatusIfPresent("obs-1", "Obs One", "YVR", "", "{}", "910.4249999,62.5,7,5", "2026-08-01T00:05:00Z"); err != nil {
		t.Fatalf("update: %v", err)
	}
	st.db.QueryRow(`SELECT radio FROM observers WHERE id = 'obs-1'`).Scan(&got)
	if got != "910.425,62.5,7,5" {
		t.Errorf("after UpdateObserverStatusIfPresent radio = %q, want normalized", got)
	}

	// ListObservers must surface it, since segment detection sorts receivers by it.
	obs, err := st.ListObservers()
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].Radio != "910.425,62.5,7,5" {
		t.Errorf("ListObservers radio = %+v, want the normalized config", obs)
	}
}
