package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// A node's radio must never be inferred at read time from whoever happened to
// hear it. The node-detail endpoint used to fill an empty value with the most
// common config among the observers that heard it at any hop count — the same
// unfounded claim ingest stopped making, and it silently undid the deliberate
// blanking of a far-side node's inherited radio.
func TestNodeDetailDoesNotInferRadio(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "detail.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// An observer with a known radio, hearing a node only via relays.
	if err := st.UpsertObserverStatus("obs-1", "Obs", "YVR", "", "{}",
		"910.425,62.5,7,5", "2026-08-01T00:00:00Z"); err != nil {
		t.Fatalf("status: %v", err)
	}
	const pk = "00000000000000000000000000000000000000000000000000000000000000AB"
	for i := 0; i < 3; i++ {
		obs := store.Observation{
			Packet: &meshcore.Packet{
				MessageHash:  string(rune('a'+i)) + "relayed",
				PathHopCount: 5, // never direct
				Advert:       &meshcore.Advert{PublicKey: pk, HasName: true, Name: "Relayed Only", SignatureValid: true},
			},
			RawHex:     "00",
			ObserverID: "obs-1",
			ReceivedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}
		if err := st.Record(obs); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/nodes/" + pk)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Node *store.Node `json:"node"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Node == nil {
		t.Fatal("node missing from the response")
	}
	if out.Node.Radio != "" {
		t.Errorf("radio = %q, want empty: nothing has heard this node directly, so its PHY is unknown", out.Node.Radio)
	}
}
