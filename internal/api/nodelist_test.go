package api

import (
	"fmt"
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

// /api/nodes reuses one scan for nodeListTTL, and any write request drops it
// so the writer sees its own change at once.
func TestNodeListCachedAndDroppedOnWrite(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	addNode := func(i int) {
		t.Helper()
		if err := st.Record(store.Observation{
			Packet: &meshcore.Packet{
				MessageHash: fmt.Sprintf("h%d", i),
				Advert:      &meshcore.Advert{PublicKey: fmt.Sprintf("%064X", i), SignatureValid: true},
			},
			RawHex: "00", ObserverID: "obs", ReceivedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	count := func() int {
		t.Helper()
		nodes, err := srv.cachedNodes()
		if err != nil {
			t.Fatal(err)
		}
		return len(nodes)
	}

	addNode(1)
	if n := count(); n != 1 {
		t.Fatalf("want 1 node, got %d", n)
	}
	addNode(2)
	if n := count(); n != 1 {
		t.Fatalf("want the cached list (1 node) within the TTL, got %d", n)
	}
	resp, err := http.Post(ts.URL+"/api/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if n := count(); n != 2 {
		t.Fatalf("want a fresh list (2 nodes) after a write request, got %d", n)
	}
}
