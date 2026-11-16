package store

import (
	"fmt"
	"testing"
	"time"
)

// RawWindowEach must deliver exactly RawWindow's rows, in the same order, no
// matter where the slice boundaries fall — including rows on a whole second
// (RFC3339Nano drops the zero fraction, which sorts AFTER ".5Z" as a string).
func TestRawWindowEachMatchesRawWindow(t *testing.T) {
	st := testStore(t)
	now := time.Now().UTC().Truncate(time.Hour)
	var stamps []string
	// Rows every 7 minutes across 30h, plus rows exactly on hour boundaries and
	// just either side of them.
	for d := -30 * time.Hour; d < time.Hour; d += 7 * time.Minute {
		stamps = append(stamps, now.Add(d).Add(13*time.Second+250*time.Millisecond).Format(time.RFC3339Nano))
	}
	for h := -26; h <= 0; h++ {
		b := now.Add(time.Duration(h) * time.Hour)
		stamps = append(stamps,
			b.Format(time.RFC3339Nano),                           // exactly on the bound, "…:00Z"
			b.Add(time.Nanosecond).Format(time.RFC3339Nano),      // just after
			b.Add(-time.Nanosecond).Format(time.RFC3339Nano),     // just before
			b.Add(500*time.Millisecond).Format(time.RFC3339Nano)) // same second, ".5Z"
	}
	for i, ts := range stamps {
		if _, err := st.db.Exec(`INSERT INTO observations (message_hash, raw_hex, route_type, payload_type, received_at)
			VALUES (?, ?, 'Flood', 'Advert', ?)`, fmt.Sprintf("h%d", i), fmt.Sprintf("%04X", i), ts); err != nil {
			t.Fatal(err)
		}
	}
	since := now.Add(-24*time.Hour + 17*time.Minute + 333*time.Millisecond).Format(time.RFC3339Nano)

	for _, max := range []int{1000000, 100, 1} {
		want, err := st.RawWindow(since, max)
		if err != nil {
			t.Fatal(err)
		}
		var got []RawObservation
		n, err := st.RawWindowEach(since, time.Hour, max, func(o RawObservation) { got = append(got, o) })
		if err != nil {
			t.Fatal(err)
		}
		if n != len(got) || len(got) != len(want) {
			t.Fatalf("max=%d: got %d rows (n=%d), want %d", max, len(got), n, len(want))
		}
		for i := range want {
			if got[i].RawHex != want[i].RawHex {
				t.Fatalf("max=%d: row %d = %s (%s), want %s (%s)", max, i, got[i].RawHex, got[i].ReceivedAt, want[i].RawHex, want[i].ReceivedAt)
			}
		}
	}
}
