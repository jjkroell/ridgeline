package store

import (
	"path/filepath"
	"testing"
)

// A far-side node's stored radio is only meaningless when it came from a
// near-side receiver. Once a receiver sits on the far segment, that same column
// can hold a real measurement of the far side, and the display has to tell the
// two apart instead of blanking both.
//
// The value itself is the discriminator: a config naming the FAR segment cannot
// have been written by a near-side receiver, because nothing over here
// transmits or demodulates on that channel.
func TestFarSideNodeKeepsAMeasuredFarSegmentRadio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fardisplay.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := st.db.Exec(
		`INSERT INTO blocklist (kind, key, name, created_at, peer, peer_radio) VALUES (?,?,?,?,?,?)`,
		BlockKnown, "AAAA", "bridge", "2026-08-01T00:00:00Z", "BBBB", "909.000,62.5,8,5"); err != nil {
		t.Fatalf("seed bridge: %v", err)
	}
	seed := []struct{ pubkey, radio, viaBridge string }{
		// Measured by a receiver on the far segment. Spelled differently from the
		// declared config on purpose: these are compared numerically, not as text.
		{"MEASURED", "909.0,62.5,8,5", "AAAA"},
		// The legacy case: a near-side receiver's config, inherited from a relayed
		// copy before v0.15.1. Describes the listener, not the node.
		{"STALE", "910.425,62.5,7,5", "AAAA"},
		// Nothing has ever measured it.
		{"BLANK", "", "AAAA"},
	}
	for _, n := range seed {
		if _, err := st.db.Exec(
			`INSERT INTO nodes (pubkey, first_seen, last_seen, advert_count, radio, via_bridge)
			 VALUES (?,?,?,1,?,?)`,
			n.pubkey, "2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z", nullStr(n.radio), n.viaBridge); err != nil {
			t.Fatalf("seed node %s: %v", n.pubkey, err)
		}
	}
	st.Close()

	st, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()

	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	got := map[string]Node{}
	for _, n := range nodes {
		got[n.PublicKey] = n
	}

	// Measured: the reading survives, and carries NO declared marker.
	if m := got["MEASURED"]; m.Radio != "909.0,62.5,8,5" {
		t.Errorf("measured far-side radio = %q, want it kept", m.Radio)
	} else if m.ViaBridgeRadio != "" {
		t.Errorf("measured far-side ViaBridgeRadio = %q, want empty: a measurement must not be marked declared", m.ViaBridgeRadio)
	}

	// Stale near-side config: suppressed, and the declared value stands in.
	if s := got["STALE"]; s.Radio != "" {
		t.Errorf("stale near-side radio = %q, want blanked", s.Radio)
	} else if s.ViaBridgeRadio != "909.000,62.5,8,5" {
		t.Errorf("stale ViaBridgeRadio = %q, want the declared far config", s.ViaBridgeRadio)
	}

	// Never measured: declared value, as before.
	if b := got["BLANK"]; b.Radio != "" || b.ViaBridgeRadio != "909.000,62.5,8,5" {
		t.Errorf("unmeasured node radio=%q viaBridgeRadio=%q, want blank + declared", b.Radio, b.ViaBridgeRadio)
	}

	// The bridge callout still names the link on every far-side node, measured
	// or not — being measured says which segment it is on, not how it reaches us.
	for _, k := range []string{"MEASURED", "STALE", "BLANK"} {
		if got[k].ViaBridgeName != "bridge" {
			t.Errorf("%s ViaBridgeName = %q, want the bridge's name", k, got[k].ViaBridgeName)
		}
	}
}
