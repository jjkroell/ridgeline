package ingest

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

type fakeMsg struct {
	topic    string
	payload  string
	retained bool
}

func (m fakeMsg) Duplicate() bool   { return false }
func (m fakeMsg) Qos() byte         { return 1 }
func (m fakeMsg) Retained() bool    { return m.retained }
func (m fakeMsg) Topic() string     { return m.topic }
func (m fakeMsg) MessageID() uint16 { return 0 }
func (m fakeMsg) Payload() []byte   { return []byte(m.payload) }
func (m fakeMsg) Ack()              {}

var (
	nbReporter = strings.Repeat("99", 32)
	nbA        = strings.Repeat("ec", 32)
	nbB        = strings.Repeat("85", 32)
	nbC        = strings.Repeat("c0", 32)
	nbD        = strings.Repeat("d0", 32)
)

// Shape of a real openHop publish (captured 2026-10-20), trimmed to two rows,
// plus rows that must be refused.
func nbPayload() string {
	return `{"timestamp":"2026-10-20T21:11:18+00:00","origin":"G3","origin_id":"` + strings.Repeat("11", 32) + `",
	"total_neighbors":5,"queried_neighbors":5,"self":{"scopes":"*","default_scope":"*"},
	"neighbors":[
	 {"pubkey":"` + nbA + `","snr":7.2,"heard_secs_ago":79,"scopes":"*,bc,swbc","status":"responded"},
	 {"pubkey":"` + nbB + `","snr":-8.0,"heard_secs_ago":18981,"scopes":"","status":"timeout"},
	 {"pubkey":"` + nbA + `","snr":1,"heard_secs_ago":5,"status":"responded"},
	 {"pubkey":"` + nbC + `","snr":2,"heard_secs_ago":null,"status":"responded"},
	 {"pubkey":"` + nbD + `","snr":2,"heard_secs_ago":99999999},
	 {"pubkey":"` + nbReporter + `","snr":1,"heard_secs_ago":5},
	 {"pubkey":"abcd","snr":1,"heard_secs_ago":5}
	]}`
}

func nbIngestor(t *testing.T) (*Ingestor, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nb.db"))
	if err != nil {
		t.Fatal(err)
	}
	in := testIngestor()
	in.store = st
	return in, st
}

func TestNeighborsStoredUnderTopicKey(t *testing.T) {
	in, st := nbIngestor(t)
	in.handle(nil, fakeMsg{topic: "meshcore/YCD/" + nbReporter + "/neighbors", payload: nbPayload()})

	r, ok := st.NeighborReportOf(nbReporter)
	if !ok {
		t.Fatal("report not stored under the topic's key")
	}
	if _, ok := st.NeighborReportOf(strings.Repeat("11", 32)); ok {
		t.Error("payload origin_id must not be trusted as the reporter")
	}
	if len(r.Neighbors) != 3 {
		t.Fatalf("want 3 rows (duplicate, self, short key, absurd age dropped), got %d: %+v", len(r.Neighbors), r.Neighbors)
	}
	if got := r.Neighbors[2]; got.PublicKey != strings.ToUpper(nbC) || got.HeardSecsAgo != -1 {
		t.Errorf("null heard_secs_ago should be kept as unknown (-1), got %+v", got)
	}
	if got := r.Neighbors[0]; got.PublicKey != strings.ToUpper(nbA) || got.SNR == nil || *got.SNR != 7.2 || got.HeardSecsAgo != 79 {
		t.Errorf("row 0 = %+v", got)
	}
	if r.SelfScopes != "*" {
		t.Errorf("self scopes = %q", r.SelfScopes)
	}
	if hb := st.ReportersListing(nbB); len(hb) != 1 || hb[0].Reporter != strings.ToUpper(nbReporter) {
		t.Errorf("reverse lookup = %+v", hb)
	}
}

func TestNeighborsRefusedWhenNotTrusted(t *testing.T) {
	cases := map[string]func(*Ingestor, *store.Store) fakeMsg{
		"retained": func(_ *Ingestor, _ *store.Store) fakeMsg {
			return fakeMsg{topic: "meshcore/YCD/" + nbReporter + "/neighbors", payload: nbPayload(), retained: true}
		},
		"short topic key": func(_ *Ingestor, _ *store.Store) fakeMsg {
			return fakeMsg{topic: "meshcore/YCD/ABCD/neighbors", payload: nbPayload()}
		},
		"radio not yet confirmed": func(_ *Ingestor, st *store.Store) fakeMsg {
			st.SetAllowedRadios([]string{"909.0,62.5,7,5"})
			return fakeMsg{topic: "meshcore/YCD/" + nbReporter + "/neighbors", payload: nbPayload()}
		},
		"radio quarantined": func(in *Ingestor, st *store.Store) fakeMsg {
			st.SetAllowedRadios([]string{"909.0,62.5,7,5"})
			now := time.Now().UTC().Format(time.RFC3339)
			_ = st.UpsertObserverStatus(nbReporter, "G3", "YCD", "", "{}", "910.525,62.5,7,5", now)
			in.evaluateRadio(nbReporter, "G3", "910.525,62.5,7,5", now, false)
			return fakeMsg{topic: "meshcore/YCD/" + nbReporter + "/neighbors", payload: nbPayload()}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			in, st := nbIngestor(t)
			in.handle(nil, setup(in, st))
			if _, ok := st.NeighborReportOf(nbReporter); ok {
				t.Error("report should have been refused")
			}
		})
	}
}

func TestNeighborsReloadedOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nb.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	in := testIngestor()
	in.store = st
	in.handle(nil, fakeMsg{topic: "meshcore/YCD/" + nbReporter + "/neighbours", payload: nbPayload()})
	st.Close()
	st2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	if r, ok := st2.NeighborReportOf(nbReporter); !ok || len(r.Neighbors) != 3 {
		t.Fatalf("after reopen: ok=%v %+v", ok, r)
	}
}
