package store

import (
	"path/filepath"
	"testing"
)

// On a single-band mesh the per-node radio label is hidden (the site shows the
// mesh's settings once instead); with several presets, or none, it is kept.
func TestSingleBandHidesPerNodeRadio(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "singleband.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	mustStatus(t, st, "obs-909", "909.0,62.5,7,5")
	node := seedAdvert(t, st, "obs-909", 0) // direct: label = 909

	radioOf := func() string {
		t.Helper()
		nodes, err := st.ListNodes()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, n := range nodes {
			if n.PublicKey == node {
				return n.Radio
			}
		}
		t.Fatalf("node missing from list")
		return ""
	}

	if got := radioOf(); got == "" {
		t.Fatalf("with no preset configured the label must be kept, got empty")
	}
	if got := st.MeshRadio(); got != "" {
		t.Errorf("MeshRadio = %q with no preset, want empty", got)
	}

	st.SetAllowedRadios([]string{"909.000,62.5,7,5"})
	if got := radioOf(); got != "" {
		t.Errorf("single-band mesh: radio = %q, want hidden", got)
	}
	if got := st.MeshRadio(); got != "909.0,62.5,7,5" {
		t.Errorf("MeshRadio = %q, want the one preset", got)
	}

	st.SetAllowedRadios([]string{"909.000,62.5,7,5", "910.425,62.5,7,5"})
	if got := radioOf(); got == "" {
		t.Errorf("two presets: the per-node label still distinguishes bands and must be kept")
	}
	if got := st.MeshRadio(); got != "" {
		t.Errorf("MeshRadio = %q with two presets, want empty", got)
	}
}

// A node beyond a bridge keeps its declared far-segment radio even on a
// single-band mesh: that segment is another band by definition.
func TestSingleBandKeepsFarSideRadio(t *testing.T) {
	nodes := []Node{
		{PublicKey: "A", Radio: "909.0,62.5,7,5"},
		{PublicKey: "B", Radio: "", ViaBridge: "X", ViaBridgeRadio: "910.425,62.5,7,5"},
		{PublicKey: "C", Radio: "910.425,62.5,7,5", ViaBridge: "X"}, // measured over there
	}
	st, err := Open(filepath.Join(t.TempDir(), "farside.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	st.SetAllowedRadios([]string{"909.000,62.5,7,5"})
	st.hideSingleBandRadios(nodes)
	if nodes[0].Radio != "" {
		t.Errorf("ordinary node: radio = %q, want hidden", nodes[0].Radio)
	}
	if nodes[1].ViaBridgeRadio == "" {
		t.Errorf("far-side node lost its declared radio")
	}
	if nodes[2].Radio == "" {
		t.Errorf("far-side node lost its measured radio")
	}
}

// With no bridge recorded the sweep replaces membership with nothing, which
// must clear marks left over from when one was (prod kept 12 after the 909
// cutover because the sweep returned before ever doing this).
func TestEmptySegmentApplyClearsStaleMarks(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "clear.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	mustStatus(t, st, "obs", "909.0,62.5,7,5")
	node := seedAdvert(t, st, "obs", 0)
	if _, err := st.ApplySegments([]SegmentMember{{NodeKey: node, BridgeKey: "B", Confidence: "confirmed"}}); err != nil {
		t.Fatalf("mark: %v", err)
	}
	var via string
	st.db.QueryRow(`SELECT COALESCE(via_bridge,'') FROM nodes WHERE pubkey = ?`, node).Scan(&via)
	if via == "" {
		t.Fatalf("setup: node was not marked")
	}
	if _, err := st.ApplySegments(nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	st.db.QueryRow(`SELECT COALESCE(via_bridge,'') FROM nodes WHERE pubkey = ?`, node).Scan(&via)
	if via != "" {
		t.Errorf("via_bridge = %q after an empty sweep, want cleared", via)
	}
}
