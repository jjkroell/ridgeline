package analytics

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// seedReceptions records the same transmissions at several observers with
// small per-observer clock offsets, plus adverts (direct and relayed), so both
// the observer summary and "Heard by" have real data, incl. clock skew.
func seedReceptions(t *testing.T, st *store.Store, now time.Time) (advertKey string) {
	t.Helper()
	adv, err := meshcore.DecodeHex(advertFixture)
	if err != nil || adv.Advert == nil {
		t.Fatalf("decode advert: %v", err)
	}
	advertKey = adv.Advert.PublicKey
	hop := strings.ToLower(advertKey[:2])
	observers := []struct {
		id, region string
		skew       time.Duration
	}{{"obs-A", "YVR", 0}, {"obs-B", "YVR", 400 * time.Millisecond}, {"obs-C", "YXX", -250 * time.Millisecond}}
	rec := func(p *meshcore.Packet, raw, obs, region string, at time.Time, snr, rssi float64) {
		t.Helper()
		if err := st.Record(store.Observation{Packet: p, RawHex: raw, ObserverID: obs, Region: region,
			SNR: &snr, RSSI: &rssi, ReceivedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		raw := fmt.Sprintf("0901%s%08X", hop, 0xAABB0000+i) // distinct transmissions, relayed by the node
		pkt, err := meshcore.DecodeHex(raw)
		if err != nil {
			t.Fatal(err)
		}
		at := now.Add(-time.Duration(i*37) * time.Minute)
		for j, o := range observers {
			if i%4 == 3 && j == 2 {
				continue // not everyone hears everything
			}
			rec(pkt, raw, o.id, o.region, at.Add(o.skew), float64(-5+j*3+i%5), float64(-110+j*4))
		}
		if i%3 == 0 { // the node's own advert, heard by two observers
			rec(adv, advertFixture, "obs-A", "YVR", at.Add(time.Second), 7, -95)
			rec(adv, advertFixture, "obs-B", "YVR", at.Add(time.Second+300*time.Millisecond), 2, -101)
		}
	}
	return advertKey
}

// The decode-one-observer ObserverSummary must equal the old full scan.
func TestObserverSummaryMatchesFullScan(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "obs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	seedReceptions(t, st, now)
	nodes, _ := st.ListNodes()
	since := now.Add(-24 * time.Hour).Format(time.RFC3339Nano)
	for _, id := range []string{"obs-A", "obs-B", "obs-C", "nobody"} {
		got, err := ObserverSummary(st, nodes, id, since, 0)
		if err != nil {
			t.Fatal(err)
		}
		want, err := observerSummaryScan(st, nodes, id, since, 0)
		if err != nil {
			t.Fatal(err)
		}
		got.WindowHours, want.WindowHours = 0, 0 // computed from time.Since at each call
		got.PacketsPerHour, want.PacketsPerHour = 0, 0
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: fast summary differs from the full scan\n got  %+v\n want %+v", id, *got, *want)
		}
		if id == "obs-B" && got.ClockSkewMs == nil {
			t.Errorf("obs-B: expected a clock-skew estimate from shared transmissions")
		}
	}
}

// "Heard by" from the activity index must equal NodeObservers' scan.
func TestHeardByMatchesNodeObservers(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "heard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	key := seedReceptions(t, st, now)
	nodes, _ := st.ListNodes()
	names, _ := st.ObserverNames()

	idx := NewActivityIndex(7 * 24 * time.Hour)
	if err := idx.Update(context.Background(), st, nodes, now); err != nil {
		t.Fatal(err)
	}
	since := now.Truncate(time.Hour).Add(-72 * time.Hour) // the index answers to the hour
	got, ok := idx.HeardBy(key, since, now, names)
	if !ok {
		t.Fatal("index not ready")
	}
	want, err := NodeObservers(st, nodes, key, since.Format(time.RFC3339Nano), 0)
	if err != nil {
		t.Fatal(err)
	}
	byID := func(l []ObserverStat) map[string]ObserverStat {
		m := map[string]ObserverStat{}
		for _, o := range l {
			m[o.ID] = o
		}
		return m
	}
	g, w := byID(got), byID(want)
	if len(g) != len(w) || len(g) == 0 {
		t.Fatalf("observers: got %d, want %d (non-zero)", len(g), len(w))
	}
	for id, wo := range w {
		go_ := g[id]
		if go_.Count != wo.Count || go_.Region != wo.Region || go_.Name != wo.Name ||
			!sameFloat(go_.AvgSNR, wo.AvgSNR) || !sameFloat(go_.AvgRSSI, wo.AvgRSSI) {
			t.Errorf("%s: got %+v, want %+v", id, go_, wo)
		}
	}
	if _, ok := idx.HeardBy(key, now.Add(-8*24*time.Hour), now, names); ok {
		t.Error("an 8-day window must fall back to the scan")
	}
}

func sameFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	d := *a - *b
	return d < 1e-9 && d > -1e-9
}
