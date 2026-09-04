package store

import (
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// A real captured advert. Recording it is what creates the node row, so the
// same hex reported by two observers is exactly the "who evidenced this node"
// situation the orphan pass has to judge.
const orphanAdvertHex = "10B76000008A654F144C4A43D07F024E8E0A59120F9A3C2E825453F88F861C48C2F2BA245CD672C56C5BE3F52CB4470337904147543724983C2978DCCE234CE41898674714704C6A642E674B47350417E441B17CF5CC4BF444792266EDC3F4FE7970F4DF977CA9344CE16A45FEA7FCD25A85E9653FD2F1DB63666B0A792290A37F398341E70B61099279FFED021081B5F8F09F8D924368657272792048696C6C20F09F8D92"

// heardBy records the fixture advert as reported by one observer, and returns
// the node's pubkey.
func heardBy(t *testing.T, st *Store, observer string) string {
	t.Helper()
	pkt, err := meshcore.DecodeHex(orphanAdvertHex)
	if err != nil || pkt == nil || pkt.Advert == nil {
		t.Fatalf("decode fixture advert: %v", err)
	}
	if err := st.Record(Observation{
		Packet: pkt, RawHex: orphanAdvertHex,
		ObserverID: observer, ObserverName: observer,
		ReceivedAt: time.Now(),
	}); err != nil {
		t.Fatalf("record advert for %s: %v", observer, err)
	}
	return pkt.Advert.PublicKey
}

func nodeExists(t *testing.T, st *Store, pubkey string) bool {
	t.Helper()
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE UPPER(pubkey) = UPPER(?)`, pubkey).Scan(&n); err != nil {
		t.Fatalf("count node: %v", err)
	}
	return n > 0
}

// Deleting an observer is meant to read as if it had never connected to the
// broker. Before this, it deleted the observer's packets and left every node
// row those packets had created — nodes nothing could account for any more,
// frozen at the moment of the delete and lingering until the 7-day retention
// sweep happened to collect them.
func TestDeleteObserverScrubsNodesOnlyItHeard(t *testing.T) {
	st := testStore(t)
	node := heardBy(t, st, "obs-A")

	if !nodeExists(t, st, node) {
		t.Fatal("fixture advert did not create the node row")
	}

	res, err := st.ScrubNodes([]string{"obs-A"}, nil, nil)
	if err != nil {
		t.Fatalf("delete observer: %v", err)
	}
	if res.Observations != 1 {
		t.Errorf("observations deleted = %d, want 1", res.Observations)
	}
	if res.OrphanNodes != 1 || res.Nodes != 1 {
		t.Errorf("orphan sweep: nodes=%d orphans=%d, want 1 and 1", res.Nodes, res.OrphanNodes)
	}
	if nodeExists(t, st, node) {
		t.Error("node row survived the delete of the only observer that heard it")
	}
}

// The other half of the same rule: a node two observers heard is evidenced by
// more than the one being removed, so it stays. Deleting an observer must never
// take the mesh's own nodes with it.
func TestDeleteObserverKeepsNodesHeardElsewhere(t *testing.T) {
	st := testStore(t)
	node := heardBy(t, st, "obs-A")
	heardBy(t, st, "obs-B")

	res, err := st.ScrubNodes([]string{"obs-A"}, nil, nil)
	if err != nil {
		t.Fatalf("delete observer: %v", err)
	}
	if res.OrphanNodes != 0 {
		t.Errorf("orphans=%d, want 0 — obs-B still evidences the node", res.OrphanNodes)
	}
	if !nodeExists(t, st, node) {
		t.Fatal("node heard by obs-B was deleted with obs-A")
	}

	// Remove the last witness and it goes.
	res, err = st.ScrubNodes([]string{"obs-B"}, nil, nil)
	if err != nil {
		t.Fatalf("delete second observer: %v", err)
	}
	if res.OrphanNodes != 1 {
		t.Errorf("orphans=%d, want 1 once the last observer is gone", res.OrphanNodes)
	}
	if nodeExists(t, st, node) {
		t.Error("node row survived the delete of its last observer")
	}
}

// A claim outranks the sweep: the operator claimed a real radio, and only one
// receiver ever hearing it is a coverage accident rather than evidence it was
// fictional. The node is kept and reported, not silently deleted along with
// somebody's ownership.
func TestDeleteObserverKeepsClaimedOrphan(t *testing.T) {
	st := testStore(t)
	node := heardBy(t, st, "obs-A")

	st.CreateUser("owner@example.com", "h", "Owner") // first = admin/owner
	u, _ := st.CreateUser("claimer@example.com", "h", "Claimer")
	if _, err := st.CreateVerifiedClaim(node, u.ID); err != nil {
		t.Fatalf("claim: %v", err)
	}

	res, err := st.ScrubNodes([]string{"obs-A"}, nil, nil)
	if err != nil {
		t.Fatalf("delete observer: %v", err)
	}
	if res.OrphanNodes != 0 {
		t.Errorf("orphans=%d, want 0 — the node is claimed", res.OrphanNodes)
	}
	if len(res.SkippedClaimed) != 1 {
		t.Fatalf("skippedClaimed = %v, want the claimed orphan reported", res.SkippedClaimed)
	}
	if !nodeExists(t, st, node) {
		t.Error("claimed node was deleted with its observer")
	}
	if owner, ok, _ := st.NodeOwner(node); !ok || owner.UserID != u.ID {
		t.Error("claim did not survive the observer delete")
	}
}

// The automatic retention sweep calls PurgeTargets with node keys and no
// observers. It must not acquire the orphan behaviour by accident: a node
// pruned for going silent is expected back on its next advert, and inferring
// further removals from its deleted adverts would delete nodes nobody asked
// about.
func TestRetentionPurgeDoesNotSweepOrphans(t *testing.T) {
	st := testStore(t)
	node := heardBy(t, st, "obs-A")

	res, err := st.PurgeTargets(nil, nil, []string{node})
	if err != nil {
		t.Fatalf("retention purge: %v", err)
	}
	if res.OrphanNodes != 0 {
		t.Errorf("orphans=%d, want 0 — retention purges only what it names", res.OrphanNodes)
	}
	if res.Nodes != 1 {
		t.Errorf("nodes=%d, want the 1 targeted row", res.Nodes)
	}
	// The observer itself is untouched by a node purge.
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM observers WHERE id = 'obs-A'`).Scan(&n); err != nil {
		t.Fatalf("count observers: %v", err)
	}
	if n != 1 {
		t.Errorf("observer rows = %d, want 1", n)
	}
}
