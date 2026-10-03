package store

import (
	"fmt"
	"testing"
	"time"
)

// TestRawSincePagingMatchesTheWindow covers the paged read of the observation
// window. The window feeds the 90s analytics recompute, and before paging it was
// a single query whose ORDER BY id DESC made SQLite SCAN the whole table — which,
// on a pool pinned to one connection, blocked every other query (API reads AND
// ingest writes) for the duration. The contract to protect is that paging did not
// change WHICH rows come back, or their order.
func TestRawSincePagingMatchesTheWindow(t *testing.T) {
	st := testStore(t)

	// Enough rows to cross several page boundaries, plus rows outside the window
	// that must not appear however the scan is bounded.
	const inWindow = rawPageSize*2 + 137
	const outOfWindow = 500

	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	tx, err := st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ins := func(raw, at string) {
		if _, err := tx.Exec(`INSERT INTO observations
			(message_hash, raw_hex, route_type, payload_type, path_hops, observer_id, region, snr, rssi, received_at)
			VALUES (?,?,'flood','Advert',0,'obs-1','YVR',1.0,-100.0,?)`, raw, raw, at); err != nil {
			t.Fatal(err)
		}
	}
	// Oldest first, so ids ascend with time the way real ingest produces them.
	for i := 0; i < outOfWindow; i++ {
		ins(fmt.Sprintf("OLD%06d", i), old)
	}
	for i := 0; i < inWindow; i++ {
		ins(fmt.Sprintf("NEW%06d", i), now.Add(-time.Duration(inWindow-i)*time.Second).Format(time.RFC3339Nano))
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	cutoff := now.Add(-24 * time.Hour).Format(time.RFC3339Nano)
	got, err := st.RawWindow(cutoff, 0)
	if err != nil {
		t.Fatalf("RawWindow: %v", err)
	}
	if len(got) != inWindow {
		t.Fatalf("RawWindow returned %d rows, want %d — a page boundary dropped or duplicated rows", len(got), inWindow)
	}
	// Newest first, and nothing from outside the window.
	for i, o := range got {
		want := fmt.Sprintf("NEW%06d", inWindow-1-i)
		if o.RawHex != want {
			t.Fatalf("row %d = %q, want %q — paging changed the order", i, o.RawHex, want)
		}
	}

	// A limit smaller than one page must still be honoured exactly.
	short, err := st.RecentRaw(cutoff, 50)
	if err != nil {
		t.Fatalf("RecentRaw: %v", err)
	}
	if len(short) != 50 {
		t.Errorf("RecentRaw(50) returned %d rows, want 50", len(short))
	}
	if short[0].RawHex != fmt.Sprintf("NEW%06d", inWindow-1) {
		t.Errorf("RecentRaw first row = %q, want the newest", short[0].RawHex)
	}

	// An empty window returns no rows rather than erroring on the id floor.
	empty, err := st.RawWindow(now.Add(time.Hour).Format(time.RFC3339Nano), 0)
	if err != nil {
		t.Fatalf("RawWindow on an empty window: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty window returned %d rows", len(empty))
	}
}
