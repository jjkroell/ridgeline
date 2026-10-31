package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticHandlerAssetsAndFallback(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<html>shell</html>")
	write("_app/immutable/assets/app.abc.css", "body{}")
	write("service-worker.js", "//sw")
	h := staticHandler(dir)

	for _, tc := range []struct {
		path, cache string
		code        int
	}{
		{"/_app/immutable/assets/app.abc.css", "public, max-age=31536000, immutable", 200},
		{"/service-worker.js", "no-cache", 200},
		{"/live-map", "no-cache", 200}, // SPA route → shell
		{"/nodes/ABCDEF", "no-cache", 200},
		// A deleted build's stylesheet must 404, not come back as the HTML shell.
		{"/_app/immutable/assets/app.old.css", "no-store", 404},
		{"/_app/immutable/chunks/gone.js", "no-store", 404},
		{"/seasonal/missing.svg", "no-store", 404},
	} {
		rr := httptest.NewRecorder()
		h(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rr.Code != tc.code || rr.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: got %d %q, want %d %q", tc.path, rr.Code, rr.Header().Get("Cache-Control"), tc.code, tc.cache)
		}
	}
}
