package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// NoNewNode keeps the observation but creates no node for an unknown key, and
// still updates a node that already exists.
func TestNoNewNodeSkipsCreationOnly(t *testing.T) {
	st := testStore(t)
	advert := func(pubkey, name, hash string, noNew bool) {
		t.Helper()
		if err := st.Record(Observation{
			Packet: &meshcore.Packet{
				MessageHash: hash,
				Advert:      &meshcore.Advert{PublicKey: pubkey, HasName: true, Name: name, SignatureValid: true},
			},
			RawHex: "00", ObserverID: "obs1", ReceivedAt: time.Now(), NoNewNode: noNew,
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	known := fmt.Sprintf("%064X", 0xA1)
	fresh := fmt.Sprintf("%064X", 0xB2)
	advert(known, "before", "h1", false)
	advert(fresh, "minted", "h2", true)
	advert(known, "after", "h3", true)

	if ok, _ := st.NodeExists(fresh); ok {
		t.Fatal("NoNewNode advert created a node row")
	}
	nodes, err := st.ListNodes()
	if err != nil || len(nodes) != 1 || nodes[0].Name != "after" {
		t.Fatalf("want the known node updated to 'after', got %+v (err %v)", nodes, err)
	}
	var n int
	st.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n)
	if n != 3 {
		t.Fatalf("want all 3 observations stored, got %d", n)
	}
}
