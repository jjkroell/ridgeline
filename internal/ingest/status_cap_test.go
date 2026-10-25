package ingest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// An oversized /status is dropped, and a normal one has its free-text fields
// clipped before they are stored and served.
func TestStatusPayloadAndFieldsCapped(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "status.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	in := testIngestor()
	in.store = st

	huge, small := strings.Repeat("AB", 32), strings.Repeat("CD", 32)
	in.handleStatus(fakeMsg{topic: "meshcore/YVR/" + huge + "/status",
		payload: `{"status":"online","origin":"x","model":"` + strings.Repeat("M", 70000) + `"}`})
	in.handleStatus(fakeMsg{topic: "meshcore/YVR/" + small + "/status",
		payload: `{"status":"online","origin":"` + strings.Repeat("N", 500) + `","model":"` + strings.Repeat("M", 500) + `"}`})

	if ok, _ := st.ObserverExists(huge); ok {
		t.Error("oversized status was stored")
	}
	obs, err := st.ListObservers()
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("want only the normal observer stored, got %d rows", len(obs))
	}
	o := obs[0]
	if len(o.Name) != maxLabelLen || o.Status == nil || len(o.Status.Model) != maxStatusFieldLen {
		t.Errorf("fields not clipped: name %d, status %+v", len(o.Name), o.Status)
	}
}
