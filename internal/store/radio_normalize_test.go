package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
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

// A node's radio may only be inherited from a DIRECT reception. A relayed copy
// says nothing about the sender's PHY — it may have crossed a bridge onto
// another band entirely, which is exactly how a 909 receiver came to stamp 909
// onto a dozen 910.425 nodes.
func TestRadioInheritedOnlyFromZeroHop(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "inherit.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	// Two receivers on different bands.
	mustStatus(t, st, "obs-910", "910.425,62.5,7,5")
	mustStatus(t, st, "obs-909", "909.0,62.5,8,5")

	t.Run("a relayed advert sets nothing", func(t *testing.T) {
		node := seedAdvert(t, st, "obs-909", 6)
		if got := nodeRadio(t, st, node); got != "" {
			t.Errorf("radio = %q after a 6-hop copy, want it unset", got)
		}
	})

	t.Run("a direct advert sets it", func(t *testing.T) {
		node := seedAdvert(t, st, "obs-910", 0)
		if got := nodeRadio(t, st, node); got != "910.425,62.5,7,5" {
			t.Errorf("radio = %q after a zero-hop advert, want the receiver's config", got)
		}
	})

	t.Run("a relayed advert cannot overwrite a measured value", func(t *testing.T) {
		node := seedAdvert(t, st, "obs-910", 0)
		reAdvert(t, st, node, "obs-909", 7)
		if got := nodeRadio(t, st, node); got != "910.425,62.5,7,5" {
			t.Errorf("radio = %q after a far-side relayed copy, want the direct measurement kept", got)
		}
	})

	// Coding rate is carried in the LoRa header, so a receiver decodes any rate
	// and its own says nothing about the sender's. Rewriting on that basis would
	// flip a node between ",7,5" and ",7,8" by whoever heard it last.
	t.Run("same network, different coding rate does not churn", func(t *testing.T) {
		mustStatus(t, st, "obs-cr8", "910.425,62.5,7,8")
		node := seedAdvert(t, st, "obs-910", 0)
		reAdvert(t, st, node, "obs-cr8", 0)
		if got := nodeRadio(t, st, node); got != "910.425,62.5,7,5" {
			t.Errorf("radio = %q, want the first value kept rather than churned to the other rate", got)
		}
	})

	t.Run("a genuinely different network does overwrite", func(t *testing.T) {
		node := seedAdvert(t, st, "obs-910", 0)
		reAdvert(t, st, node, "obs-909", 0) // heard directly on 909: it moved
		if got := nodeRadio(t, st, node); got != "909.0,62.5,8,5" {
			t.Errorf("radio = %q, want the new direct measurement", got)
		}
	})
}

// --- helpers for the inheritance tests ---

func mustStatus(t *testing.T, st *Store, id, radio string) {
	t.Helper()
	if err := st.UpsertObserverStatus(id, id, "YVR", "", "{}", radio, "2026-08-01T00:00:00Z"); err != nil {
		t.Fatalf("status for %s: %v", id, err)
	}
}

var seedCounter int

// seedAdvert records one signature-valid advert for a fresh node, heard by the
// named observer at the given hop count, and returns the node's key.
func seedAdvert(t *testing.T, st *Store, observerID string, hops int) string {
	t.Helper()
	seedCounter++
	pubkey := fmt.Sprintf("%064X", seedCounter)
	reAdvert(t, st, pubkey, observerID, hops)
	return pubkey
}

// reAdvert records another advert for an existing node from a different receiver.
func reAdvert(t *testing.T, st *Store, pubkey, observerID string, hops int) {
	t.Helper()
	seedCounter++
	obs := Observation{
		Packet: &meshcore.Packet{
			MessageHash:  fmt.Sprintf("hash%08x", seedCounter),
			PathHopCount: hops,
			Advert:       &meshcore.Advert{PublicKey: pubkey, HasName: true, Name: "n", SignatureValid: true},
		},
		RawHex:     "00",
		ObserverID: observerID,
		Region:     "YVR",
		ReceivedAt: time.Now().Add(time.Duration(seedCounter) * time.Minute),
	}
	if err := st.Record(obs); err != nil {
		t.Fatalf("record: %v", err)
	}
}

func nodeRadio(t *testing.T, st *Store, pubkey string) string {
	t.Helper()
	var radio string
	st.db.QueryRow(`SELECT COALESCE(radio,'') FROM nodes WHERE pubkey = ?`, pubkey).Scan(&radio)
	return radio
}

// The repair for data written under the old rule: a near-side node carrying the
// far segment's config can only have got it by being overheard from the wrong
// side, so it is dropped. A node genuinely ON that segment keeps its value, and
// so does every ordinary near-side node.
func TestClearMisattributedRadio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "misattributed.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// A sanctioned bridge whose far side runs 909.
	if _, err := st.db.Exec(
		`INSERT INTO blocklist (kind, key, name, created_at, peer, peer_radio) VALUES (?,?,?,?,?,?)`,
		BlockKnown, "AAAA", "bridge", "2026-08-01T00:00:00Z", "BBBB", "909.000,62.5,8,5"); err != nil {
		t.Fatalf("seed bridge: %v", err)
	}

	seed := []struct{ pubkey, radio, viaBridge string }{
		{"NEAR909", "909.0,62.5,8,5", ""},     // overheard from the far side: wrong
		{"FAR909", "909.0,62.5,8,5", "AAAA"},  // genuinely over there: correct
		{"NEAR910", "910.425,62.5,7,5", ""},   // ordinary near-side node
		{"NEAR910CR", "910.425,62.5,7,8", ""}, // same network, other coding rate
		// The bridge's own two ends. Neither is "beyond" the bridge, so both have
		// via_bridge NULL and nothing downstream blanks what is stored here.
		{"AAAA", "910.425,62.5,7,5", ""}, // far end, holding a near-side config: wrong
		{"BBBB", "910.425,62.5,7,5", ""}, // near end, on the near segment: correct
	}
	for _, n := range seed {
		if _, err := st.db.Exec(
			`INSERT INTO nodes (pubkey, first_seen, last_seen, advert_count, radio, via_bridge)
			 VALUES (?,?,?,1,?,?)`,
			n.pubkey, "2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z", n.radio, nullStr(n.viaBridge)); err != nil {
			t.Fatalf("seed node %s: %v", n.pubkey, err)
		}
	}
	st.Close()

	st, err = Open(path) // the repair runs here
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()

	if n := st.MisattributedRadioCleared(); n != 2 {
		t.Errorf("MisattributedRadioCleared = %d, want 2", n)
	}
	for _, c := range []struct{ pubkey, want string }{
		{"NEAR909", ""},
		{"FAR909", "909.0,62.5,8,5"},
		{"NEAR910", "910.425,62.5,7,5"},
		{"NEAR910CR", "910.425,62.5,7,8"},
		{"AAAA", ""},
		{"BBBB", "910.425,62.5,7,5"},
	} {
		if got := nodeRadio(t, st, c.pubkey); got != c.want {
			t.Errorf("%s radio = %q, want %q", c.pubkey, got, c.want)
		}
	}

	// Idempotent: a second open has nothing left to clear.
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	if n := st.MisattributedRadioCleared(); n != 0 {
		t.Errorf("second run cleared %d more rows, want 0", n)
	}
}

// The far end of a bridge is the one node whose radio SHOULD read as the far
// segment, so the misattribution rule has to run backwards for it. Reading it
// forwards would delete the only measured value that end can ever have — a
// far-side receiver hearing it zero-hop — and would do so at every open.
func TestBridgeFarEndKeepsFarSegmentRadio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "farend.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := st.db.Exec(
		`INSERT INTO blocklist (kind, key, name, created_at, peer, peer_radio) VALUES (?,?,?,?,?,?)`,
		BlockKnown, "AAAA", "bridge", "2026-08-01T00:00:00Z", "BBBB", "909.000,62.5,8,5"); err != nil {
		t.Fatalf("seed bridge: %v", err)
	}
	// AAAA is the far end (see BridgeLink.FarEnd) and a far-side receiver has
	// heard it directly, so 909 here is measured fact, not a misattribution.
	if _, err := st.db.Exec(
		`INSERT INTO nodes (pubkey, first_seen, last_seen, advert_count, radio)
		 VALUES (?,?,?,1,?)`,
		"AAAA", "2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z", "909.0,62.5,8,5"); err != nil {
		t.Fatalf("seed far end: %v", err)
	}
	st.Close()

	st, err = Open(path) // the repair runs here
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()

	if n := st.MisattributedRadioCleared(); n != 0 {
		t.Errorf("cleared %d rows, want 0 — the far end's own segment is not a misattribution", n)
	}
	if got := nodeRadio(t, st, "AAAA"); got != "909.0,62.5,8,5" {
		t.Errorf("far end radio = %q, want it kept", got)
	}
}
