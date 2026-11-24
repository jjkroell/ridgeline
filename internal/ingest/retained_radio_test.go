package ingest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

func radioStatus(topic, radio string, retained bool) fakeMsg {
	return fakeMsg{topic: topic, payload: `{"status":"online","origin":"LSE","radio":"` + radio + `"}`, retained: retained}
}

// The broker replays an old retained status on every reconnect. One from before
// the observer changed frequency must not turn it out or cost it its packets.
func TestOutdatedRetainedStatusIgnored(t *testing.T) {
	const obs, good, old = "EBD5FE9AC359AE49920EC4056750429488276A7A144450278D60403CE94DB27B", "909.0,62.5,7,5", "910.425,62.5,7,5"
	topic := "meshcore/YCD/" + obs + "/status"
	path := filepath.Join(t.TempDir(), "retained.db")
	open := func() (*store.Store, *Ingestor) {
		st, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		st.SetAllowedRadios([]string{good})
		in := testIngestor()
		in.store = st
		return st, in
	}
	radioOf := func(st *store.Store) string {
		list, err := st.ListObservers()
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range list {
			if o.ID == obs {
				return o.Radio
			}
		}
		return ""
	}

	st, in := open()
	in.handleStatus(radioStatus(topic, good, false), time.Now())
	pkt, err := meshcore.DecodeHex(retractAdvertHex)
	if err != nil || pkt.Advert == nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if err := st.Record(store.Observation{Packet: pkt, RawHex: retractAdvertHex, ObserverID: obs, ReceivedAt: time.Now().Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	stored := func(st *store.Store) bool {
		nodes, err := st.ListNodes()
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			if n.PublicKey == pkt.Advert.PublicKey {
				return true
			}
		}
		return false
	}
	check := func(when string, st *store.Store) {
		t.Helper()
		if st.ObserverRadioQuarantined(obs) || !st.ObserverRadioConfirmed(obs) {
			t.Errorf("%s: an outdated retained status turned the observer out", when)
		}
		if !stored(st) {
			t.Errorf("%s: its packets were retracted", when)
		}
		if got := radioOf(st); got != good {
			t.Errorf("%s: stored radio = %q, want %q", when, got, good)
		}
	}

	in.handleStatus(radioStatus(topic, old, true), time.Now())
	check("reconnect", st)
	st.Close()
	st, in = open()
	defer st.Close()
	in.handleStatus(radioStatus(topic, old, true), time.Now())
	check("restart", st)

	// A live report of the same preset is the observer speaking now: it still
	// quarantines, and takes back what was stored since the last good status.
	in.handleStatus(radioStatus(topic, old, false), time.Now())
	if !st.ObserverRadioQuarantined(obs) || stored(st) {
		t.Error("a live foreign status must quarantine and retract")
	}
	// And a retained good one still readmits.
	in.handleStatus(radioStatus(topic, good, true), time.Now())
	if st.ObserverRadioQuarantined(obs) {
		t.Error("a retained good status must readmit")
	}
}

// An observer we have never vouched for gets no benefit of the doubt.
func TestRetainedForeignStatusStillQuarantinesUnconfirmed(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "unconfirmed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const obs = "obs-new"
	at := time.Now().UTC().Format(time.RFC3339)
	if err := st.UpsertObserverStatus(obs, obs, "YVR", "", "{}", "", at); err != nil {
		t.Fatal(err)
	}
	st.SetAllowedRadios([]string{"909.0,62.5,7,5"})
	in := testIngestor()
	in.store = st
	in.handleStatus(radioStatus("meshcore/YVR/"+obs+"/status", "910.425,62.5,7,5", true), time.Now())
	if !st.ObserverRadioQuarantined(obs) {
		t.Error("a retained foreign status must still quarantine an unconfirmed observer")
	}
}
