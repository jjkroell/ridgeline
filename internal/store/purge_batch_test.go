package store

import (
	"fmt"
	"testing"
	"time"
)

// insertObs writes one observation row directly (fast enough for thousands).
func insertObs(t *testing.T, st *Store, raw, payload string, hops int, at time.Time) {
	t.Helper()
	if _, err := st.db.Exec(`INSERT INTO observations
		(message_hash, raw_hex, route_type, payload_type, path_hops, observer_id, region, received_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		fmt.Sprintf("h%d", at.UnixNano()), raw, "Flood", payload, hops, "obs", "YVR",
		at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func countObs(t *testing.T, st *Store, where string, args ...any) int {
	t.Helper()
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// The purge walks observations in batches (it used to hold the only DB
// connection for one multi-million-row transaction and freeze the site). A
// node's adverts must all go wherever they fall — first batch, across a batch
// boundary, last batch — and nothing else may.
func TestPurgeSpansBatches(t *testing.T) {
	st := testStore(t)
	node := heardBy(t, st, "obs-A") // first advert row (id 1) + the node row
	base := time.Now().Add(-time.Hour)

	filler := 2*purgeBatchSize + 1234
	for i := 0; i < filler; i++ {
		// Undecodable filler, with the node's advert planted at a batch boundary.
		if i == purgeBatchSize-1 || i == purgeBatchSize || i == purgeBatchSize+1 {
			insertObs(t, st, orphanAdvertHex, "Advert", 0, base.Add(time.Duration(i)*time.Millisecond))
			continue
		}
		insertObs(t, st, "00", "GroupText", 2, base.Add(time.Duration(i)*time.Millisecond))
	}
	insertObs(t, st, orphanAdvertHex, "Advert", 0, base.Add(time.Duration(filler)*time.Millisecond)) // last row

	wantDeleted := 1 + 3 + 1
	fillerLeft := countObs(t, st, "raw_hex = '00'")

	res, err := st.PurgeTargets(nil, nil, []string{node})
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if res.Observations != int64(wantDeleted) {
		t.Errorf("observations deleted = %d, want %d", res.Observations, wantDeleted)
	}
	if got := countObs(t, st, "raw_hex = ?", orphanAdvertHex); got != 0 {
		t.Errorf("%d of the node's adverts survived the purge", got)
	}
	if got := countObs(t, st, "raw_hex = '00'"); got != fillerLeft {
		t.Errorf("unrelated rows: %d left, want %d untouched", got, fillerLeft)
	}
	if nodeExists(t, st, node) {
		t.Errorf("node row survived the purge")
	}
}

// The relay-hop scan reads its window an hour at a time; hops from every hour
// must be found, and rows before the window must not be.
func TestRelayHopScanCoversWholeWindow(t *testing.T) {
	st := testStore(t)
	now := time.Now().UTC()
	pkt := func(a, b string) string { return "1502" + a + b + "11223344556677889900AABBCCDDEEFF" }
	insertObs(t, st, pkt("01", "02"), "GroupText", 2, now.Add(-30*time.Hour)) // before the window
	insertObs(t, st, pkt("AB", "CD"), "GroupText", 2, now.Add(-23*time.Hour-59*time.Minute))
	insertObs(t, st, pkt("EF", "10"), "GroupText", 2, now.Add(-7*time.Hour-30*time.Minute))
	insertObs(t, st, pkt("11", "12"), "GroupText", 2, now.Add(-time.Minute))
	insertObs(t, st, "1500", "GroupText", 0, now.Add(-time.Minute)) // zero-hop: no relays

	got, err := st.RelayHopPrefixesSince(now.Add(-24 * time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, h := range []string{"AB", "CD", "EF", "10", "11", "12"} {
		if !got[h] {
			t.Errorf("hop %s missing from the scan", h)
		}
	}
	for _, h := range []string{"01", "02"} {
		if got[h] {
			t.Errorf("hop %s is from before the window", h)
		}
	}
}
