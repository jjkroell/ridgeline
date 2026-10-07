package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// /api/health reports the mesh's radio preset only when exactly one is
// configured; the UI shows it site-wide in place of per-node labels.
func TestHealthReportsMeshRadio(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	health := func() map[string]string {
		t.Helper()
		resp, err := http.Get(ts.URL + "/api/health")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer resp.Body.Close()
		var h map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return h
	}

	if _, ok := health()["meshRadio"]; ok {
		t.Errorf("meshRadio present with no preset configured")
	}
	st.SetAllowedRadios([]string{"909.000,62.5,7,5"})
	if got := health()["meshRadio"]; got != "909.0,62.5,7,5" {
		t.Errorf("meshRadio = %q, want the single preset", got)
	}
	st.SetAllowedRadios([]string{"909.000,62.5,7,5", "910.425,62.5,7,5"})
	if _, ok := health()["meshRadio"]; ok {
		t.Errorf("meshRadio present with two presets")
	}
}
