package store

import "testing"

func hsNode(t *testing.T, st *Store, pubkey string, size int) {
	t.Helper()
	if _, err := st.db.Exec(
		`INSERT INTO nodes (pubkey, first_seen, last_seen, advert_count, hash_size)
		 VALUES (?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 1, ?)`, pubkey, size); err != nil {
		t.Fatal(err)
	}
}

func hsOf(t *testing.T, st *Store, pubkey string) (int, string) {
	t.Helper()
	var size int
	var src string
	if err := st.db.QueryRow(
		`SELECT hash_size, COALESCE(hash_size_source,'auto') FROM nodes WHERE pubkey = ?`,
		pubkey).Scan(&size, &src); err != nil {
		t.Fatal(err)
	}
	return size, src
}

// An owner pin sets the value and marks it manual; the consensus writer
// (SetHashSizes) must then leave it alone, even when the vote disagrees.
func TestHashSizeManualPinBeatsConsensus(t *testing.T) {
	st := testStore(t)
	const node = "AA11"
	hsNode(t, st, node, 1)

	if err := st.SetHashSizeManual(node, 3); err != nil {
		t.Fatal(err)
	}
	if size, src := hsOf(t, st, node); size != 3 || src != "manual" {
		t.Fatalf("after pin: size=%d src=%q, want 3/manual", size, src)
	}

	// The consensus tries to push it back to 1 — must be refused.
	if err := st.SetHashSizes(map[string]int{node: 1}); err != nil {
		t.Fatal(err)
	}
	if size, src := hsOf(t, st, node); size != 3 || src != "manual" {
		t.Errorf("consensus overrode a manual pin: size=%d src=%q, want 3/manual", size, src)
	}
}

// An auto (unpinned) node is still corrected by the consensus as before.
func TestHashSizeConsensusStillCorrectsAuto(t *testing.T) {
	st := testStore(t)
	const node = "BB22"
	hsNode(t, st, node, 1) // default source 'auto'
	if err := st.SetHashSizes(map[string]int{node: 3}); err != nil {
		t.Fatal(err)
	}
	if size, src := hsOf(t, st, node); size != 3 || src != "auto" {
		t.Errorf("auto node not corrected: size=%d src=%q, want 3/auto", size, src)
	}
}

// Clearing the pin returns the node to auto without changing the value, so the
// next consensus pass can move it.
func TestHashSizeClearManual(t *testing.T) {
	st := testStore(t)
	const node = "CC33"
	hsNode(t, st, node, 2)
	if err := st.SetHashSizeManual(node, 3); err != nil {
		t.Fatal(err)
	}
	if err := st.ClearHashSizeManual(node); err != nil {
		t.Fatal(err)
	}
	if size, src := hsOf(t, st, node); size != 3 || src != "auto" {
		t.Fatalf("after clear: size=%d src=%q, want 3/auto (value kept, source auto)", size, src)
	}
	// Now the consensus can move it again.
	if err := st.SetHashSizes(map[string]int{node: 1}); err != nil {
		t.Fatal(err)
	}
	if size, _ := hsOf(t, st, node); size != 1 {
		t.Errorf("consensus should move an unpinned node: size=%d, want 1", size)
	}
}

// ListNodes surfaces the source so the API/UI can flag manual vs auto.
func TestListNodesReportsHashSizeSource(t *testing.T) {
	st := testStore(t)
	hsNode(t, st, "DD44", 1)
	st.SetHashSizeManual("DD44", 2)
	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, n := range nodes {
		if n.PublicKey == "DD44" {
			found = true
			if n.HashSize != 2 || n.HashSizeSource != "manual" {
				t.Errorf("ListNodes: size=%d src=%q, want 2/manual", n.HashSize, n.HashSizeSource)
			}
		}
	}
	if !found {
		t.Fatal("node not returned by ListNodes")
	}
}
