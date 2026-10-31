package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// staticHandler serves the built SPA from dir, falling back to index.html for
// any path that doesn't resolve to a real file so client-side routing works.
func staticHandler(dir string) http.HandlerFunc {
	index := filepath.Join(dir, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		// Never let a non-API path escape the web dir.
		clean := filepath.Clean(r.URL.Path)
		if strings.Contains(clean, "..") {
			http.NotFound(w, r)
			return
		}

		p := filepath.Join(dir, clean)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			switch {
			case strings.HasPrefix(clean, "/_app/immutable/"):
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			case clean == "/service-worker.js" || clean == "/_app/version.json" || strings.HasSuffix(clean, ".html"):
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFile(w, r, p)
			return
		}
		// A missing build asset or file (an old build's hashed CSS after a deploy,
		// say) is a real 404, never the SPA shell: served as HTML under a .css
		// name, the browser drops the stylesheet and the page renders unstyled,
		// and Cloudflare cached that 200 for hours. no-store keeps the 404 out of
		// the edge cache too.
		if strings.HasPrefix(clean, "/_app/") || (path.Ext(clean) != "" && path.Ext(clean) != ".html") {
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
		// Prerendered pages (adapter-static) land as "<route>.html" — e.g. /about ->
		// about.html. Serve those before falling back so their real, crawlable HTML
		// is delivered instead of the empty SPA shell.
		if clean != "/" && !strings.HasSuffix(clean, ".html") {
			if info, err := os.Stat(p + ".html"); err == nil && !info.IsDir() {
				http.ServeFile(w, r, p+".html")
				return
			}
		}
		// SPA fallback.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}
}
