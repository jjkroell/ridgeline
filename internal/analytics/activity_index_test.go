package analytics

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// The in-memory index must give exactly the grid the per-view scan does, keep
// doing so as new rows arrive, and drop hours that leave the window.
func TestActivityIndexMatchesNodeHeatmap(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "act.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	adv, err := meshcore.DecodeHex(advertFixture)
	if err != nil || adv.Advert == nil {
		t.Fatalf("decode advert: %v", err)
	}
	pubkey := adv.Advert.PublicKey
	relayHex := "0901" + strings.ToLower(pubkey[:2]) + "AABBCCDD" // relayed by the node
	otherHex := "0901" + "ee" + "AABBCCDD"                          // relayed by nobody we know
	relay, _ := meshcore.DecodeHex(relayHex)
	other, _ := meshcore.DecodeHex(otherHex)

	now := time.Date(2026, 10, 7, 12, 30, 0, 0, time.UTC)
	rec := func(p *meshcore.Packet, raw string, at time.Time) {
		t.Helper()
		if err := st.Record(store.Observation{Packet: p, RawHex: raw, ObserverID: "obs", Region: "R", ReceivedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	rec(adv, advertFixture, now.Add(-9*24*time.Hour)) // outside a 7-day window
	for i := 0; i < 40; i++ {
		at := now.Add(-time.Duration(i*97) * time.Minute) // spread across ~2.7 days of hours
		if i%3 == 0 {
			rec(adv, advertFixture, at)
		} else {
			rec(relay, relayHex, at)
		}
		rec(other, otherHex, at)
	}

	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	scan := func() *NodeActivity {
		t.Helper()
		g, err := NodeHeatmap(st, nodes, pubkey, now.Add(-7*24*time.Hour).Format(time.RFC3339Nano), 0, 7)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	idx := NewActivityIndex(7 * 24 * time.Hour)
	if _, ok := idx.Heatmap(pubkey, 7, now); ok {
		t.Fatal("index answered before its first update")
	}
	if err := idx.Update(context.Background(), st, nodes, now); err != nil {
		t.Fatal(err)
	}
	check := func(stage string) {
		t.Helper()
		got, ok := idx.Heatmap(pubkey, 7, now)
		if !ok {
			t.Fatalf("%s: index not ready", stage)
		}
		want := scan()
		if got.Grid != want.Grid || got.Total != want.Total || got.Max != want.Max {
			t.Errorf("%s: index total=%d max=%d, scan total=%d max=%d (grids differ: %v)",
				stage, got.Total, got.Max, want.Total, want.Max, got.Grid != want.Grid)
		}
		if got.Total == 0 {
			t.Errorf("%s: empty heatmap — the fixture should count", stage)
		}
	}
	check("after backfill")

	// New rows after the first pass are added once — not re-counted.
	for i := 0; i < 10; i++ {
		rec(relay, relayHex, now.Add(-time.Duration(i)*time.Minute))
	}
	if err := idx.Update(context.Background(), st, nodes, now); err != nil {
		t.Fatal(err)
	}
	check("after incremental update")
	if err := idx.Update(context.Background(), st, nodes, now); err != nil {
		t.Fatal(err)
	}
	check("after an update with nothing new")

	if _, ok := idx.Heatmap(pubkey, 30, now); ok {
		t.Error("a 30-day request must fall back to the scan (index holds 7 days)")
	}

	// Ten days on, everything has left the window.
	later := now.Add(10 * 24 * time.Hour)
	if err := idx.Update(context.Background(), st, nodes, later); err != nil {
		t.Fatal(err)
	}
	if got, _ := idx.Heatmap(pubkey, 7, later); got.Total != 0 {
		t.Errorf("after the window passed: total=%d, want 0", got.Total)
	}
}
