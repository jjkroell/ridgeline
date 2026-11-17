package store

import (
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// Real node-discover replies from the mesh (2026-11-08): the same repeater
// answering once with its full key and once with an 8-byte prefix.
const (
	heardKey        = "ECF7F5CD71801B0096FC86C16F69552FD5B751657694D50079FC2CA1EFEF5FCB"
	discoverFullHex = "2E00920114B596BCECF7F5CD71801B0096FC86C16F69552FD5B751657694D50079FC2CA1EFEF5FCB"
	discoverPrefHex = "2E0092E5C07447D3ECF7F5CD71801B00"
)

func recordHex(t *testing.T, st *Store, raw string, at time.Time) {
	t.Helper()
	pkt, err := meshcore.DecodeHex(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := st.Record(Observation{Packet: pkt, RawHex: raw, ObserverID: "OBS1", ReceivedAt: at}); err != nil {
		t.Fatalf("record: %v", err)
	}
}

func lastHeardOf(t *testing.T, st *Store, key string) string {
	t.Helper()
	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.PublicKey == key {
			return n.LastHeard
		}
	}
	t.Fatalf("node %s missing", key[:8])
	return ""
}

func TestDiscoverReplyUpdatesLastHeard(t *testing.T) {
	st := testStore(t)
	old := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	st.db.Exec(`INSERT INTO nodes(pubkey,name,role,first_seen,last_seen,advert_count,advert_tx_count,hash_size)
		VALUES(?,'Mt Test','Repeater',?,?,1,1,0)`, heardKey, old, old)

	t1 := time.Date(2026, 11, 8, 2, 38, 8, 0, time.UTC)
	recordHex(t, st, discoverFullHex, t1)
	if got := lastHeardOf(t, st, heardKey); got != t1.Format(time.RFC3339Nano) {
		t.Fatalf("full-key reply: last_heard=%q", got)
	}
	// An 8-byte prefix reply credits the one node it matches.
	t2 := t1.Add(10 * time.Hour)
	recordHex(t, st, discoverPrefHex, t2)
	if got := lastHeardOf(t, st, heardKey); got != t2.Format(time.RFC3339Nano) {
		t.Fatalf("prefix reply: last_heard=%q", got)
	}
	// Within the throttle window a second copy doesn't write (nor go backwards).
	recordHex(t, st, discoverPrefHex, t2.Add(20*time.Second))
	if got := lastHeardOf(t, st, heardKey); got != t2.Format(time.RFC3339Nano) {
		t.Fatalf("throttled copy changed last_heard to %q", got)
	}
}

func TestDiscoverPrefixAmbiguousCreditsNoOne(t *testing.T) {
	st := testStore(t)
	old := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	twin := "ECF7F5CD71801B00" + "11111111111111111111111111111111111111111111111111"[:48]
	for _, k := range []string{heardKey, twin} {
		st.db.Exec(`INSERT INTO nodes(pubkey,name,role,first_seen,last_seen,advert_count,advert_tx_count,hash_size)
			VALUES(?,'n','Repeater',?,?,1,1,0)`, k, old, old)
	}
	recordHex(t, st, discoverPrefHex, time.Date(2026, 11, 8, 12, 9, 0, 0, time.UTC))
	if got := lastHeardOf(t, st, heardKey); got != "" {
		t.Fatalf("ambiguous prefix credited a node: %q", got)
	}
}

// Retention removes a claimed node: the claim keeps its name, role and last
// heard time, and a later discover reply advances last_heard on the claim.
func TestRetentionKeepsClaimedNodeName(t *testing.T) {
	st := testStore(t)
	st.CreateUser("owner@example.com", "h", "Owner")
	u, _ := st.CreateUser("claimer@example.com", "h", "Claimer")
	advertAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	st.db.Exec(`INSERT INTO nodes(pubkey,name,role,first_seen,last_seen,advert_count,advert_tx_count,hash_size)
		VALUES(?,'Mt Shepherd','Repeater',?,?,1,1,0)`, heardKey, advertAt, advertAt)
	if _, err := st.CreateVerifiedClaim(heardKey, u.ID); err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2026, 11, 8, 12, 9, 26, 0, time.UTC)
	recordHex(t, st, discoverFullHex, t1)

	if _, err := st.PurgeTargets(nil, nil, []string{heardKey}); err != nil {
		t.Fatal(err)
	}
	cs, err := st.ListUserClaims(u.ID)
	if err != nil || len(cs) != 1 {
		t.Fatalf("claims: %v %d", err, len(cs))
	}
	c := cs[0]
	if c.NodePresent || c.NodeName != "Mt Shepherd" || c.NodeRole != "Repeater" || c.LastHeard != t1.Format(time.RFC3339Nano) {
		t.Fatalf("dormant claim lost its identity: %+v", c)
	}

	// Heard again while dormant (no node row): the claim's time advances.
	t2 := t1.Add(48 * time.Hour)
	recordHex(t, st, discoverPrefHex, t2)
	cs, _ = st.ListUserClaims(u.ID)
	if cs[0].LastHeard != t2.Format(time.RFC3339Nano) {
		t.Fatalf("dormant claim last_heard=%q, want %q", cs[0].LastHeard, t2.Format(time.RFC3339Nano))
	}

	// A scrub (deliberate deletion) still removes the claim outright.
	st.db.Exec(`INSERT INTO nodes(pubkey,name,role,first_seen,last_seen,advert_count,advert_tx_count,hash_size)
		VALUES(?,'Mt Shepherd','Repeater',?,?,1,1,0)`, heardKey, advertAt, advertAt)
	if _, err := st.ScrubNodes(nil, nil, []string{heardKey}); err != nil {
		t.Fatal(err)
	}
	if cs, _ = st.ListUserClaims(u.ID); len(cs) != 0 {
		t.Fatalf("scrub left %d claims", len(cs))
	}
}
