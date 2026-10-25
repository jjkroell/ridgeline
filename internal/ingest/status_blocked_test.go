package ingest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// A blocklisted observer's /status must not re-create its observer row, while
// an unblocked one is recorded as usual.
func TestStatusFromBlockedObserverIgnored(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "status.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	in := testIngestor()
	in.store = st

	blocked, open := strings.Repeat("AB", 32), strings.Repeat("CD", 32)
	if err := st.AddBlock("observer", blocked, "rogue", "test"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{blocked, open} {
		in.handleStatus(fakeMsg{
			topic:   "meshcore/YVR/" + key + "/status",
			payload: `{"status":"online","origin":"obs","radio":"910.425,62.5,7,5"}`,
		})
	}
	if ok, _ := st.ObserverExists(blocked); ok {
		t.Error("blocked observer's status re-created its row")
	}
	if ok, _ := st.ObserverExists(open); !ok {
		t.Error("unblocked observer's status was not recorded")
	}
}
