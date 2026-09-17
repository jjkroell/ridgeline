package ingest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// A real captured advert (the same fixture the store's orphan tests use).
const retractAdvertHex = "10B76000008A654F144C4A43D07F024E8E0A59120F9A3C2E825453F88F861C48C2F2BA245CD672C56C5BE3F52CB4470337904147543724983C2978DCCE234CE41898674714704C6A642E674B47350417E441B17CF5CC4BF444792266EDC3F4FE7970F4DF977CA9344CE16A45FEA7FCD25A85E9653FD2F1DB63666B0A792290A37F398341E70B61099279FFED021081B5F8F09F8D924368657272792048696C6C20F09F8D92"

// The status path end to end: a confirmed observer that reports a foreign
// preset is quarantined AND what it stored since its last good status is gone.
func TestStatusPathRetractsOnQuarantine(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "retract.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.SetAllowedRadios([]string{"910.425,62.5,7,5"})
	in := testIngestor()
	in.store = st

	const obs = "obs-A"
	good := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	if err := st.UpsertObserverStatus(obs, obs, "YVR", "", "{}", "910.425,62.5,7,5", good); err != nil {
		t.Fatal(err)
	}
	in.evaluateRadio(obs, obs, "910.425,62.5,7,5", good, false)
	if !st.ObserverRadioConfirmed(obs) {
		t.Fatal("precondition: observer confirmed")
	}

	pkt, err := meshcore.DecodeHex(retractAdvertHex)
	if err != nil || pkt == nil || pkt.Advert == nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if err := st.Record(store.Observation{Packet: pkt, RawHex: retractAdvertHex, ObserverID: obs, ReceivedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := st.UpsertObserverStatus(obs, obs, "YVR", "", "{}", "906.875,250,11,5", now); err != nil {
		t.Fatal(err)
	}
	in.evaluateRadio(obs, obs, "906.875,250,11,5", now, false)

	if !st.ObserverRadioQuarantined(obs) {
		t.Fatal("observer should be quarantined")
	}
	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.PublicKey == pkt.Advert.PublicKey {
			t.Errorf("node %s invented during the foreign window survived the quarantine", n.PublicKey)
		}
	}
}
