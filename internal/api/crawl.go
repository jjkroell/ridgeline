package api

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// searchCrawlers are the search engines' user-agent tokens (lower case). Keep in
// step with web/src/lib/crawler.ts, which keeps the same crawlers on the
// desktop routes once the app runs.
var searchCrawlers = []string{
	"googlebot", "google-inspectiontool", "googleother", "bingbot", "bingpreview",
	"applebot", "duckduckbot", "duckassistbot", "yandexbot", "qwantbot", "slurp",
	"oai-searchbot", "claude-searchbot", "perplexitybot",
}

func isSearchCrawler(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	for _, c := range searchCrawlers {
		if strings.Contains(ua, c) {
			return true
		}
	}
	return false
}

// appRoutes are the SPA's desktop routes (each may also exist under /m).
// TestAppRoutesMatchWeb checks them against web/src/routes.
var appRoutes = map[string]bool{
	"/": true, "/about": true, "/account": true, "/admin": true, "/analytics": true,
	"/channels": true, "/contact": true, "/faq": true, "/firmware-builder": true,
	"/hash-ids": true, "/identity": true, "/live": true, "/live-map": true,
	"/login": true, "/map": true, "/nodes": true, "/observers": true, "/privacy": true,
	"/reset-password": true, "/settings": true, "/mayday": true, "/survey": true, "/topology": true,
	"/verify-email": true,
}

// isAppRoute reports whether the app has a page at this desktop path.
func isAppRoute(p string) bool {
	if appRoutes[p] {
		return true
	}
	for _, prefix := range []string{"/nodes/", "/observers/"} {
		if rest, ok := strings.CutPrefix(p, prefix); ok {
			return rest != "" && !strings.Contains(rest, "/")
		}
	}
	return false
}

// crawlRules wraps the web UI with what search engines need and people don't
// notice. A dev instance is never indexed. A crawler asking for a phone (/m)
// page goes to the desktop URL that is its canonical, so each page has one
// address: Google was spending its crawl on the /m copies. And a path the app
// has no page for is a 404, still carrying the app so people see its error page.
func (s *Server) crawlRules(next http.Handler) http.Handler {
	shell, _ := os.ReadFile(filepath.Join(s.webDir, "index.html"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.env != "" {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		// The emergency channel moved from #sos to #mayday (v0.24.88); its guide
		// moved with it, and says so while the integration is switched off.
		if clean == "/sos" || clean == "/m/sos" {
			http.Redirect(w, r, strings.TrimSuffix(clean, "sos")+"mayday", http.StatusMovedPermanently)
			return
		}
		target := previewPath(clean)
		if clean == "/m" || strings.HasPrefix(clean, "/m/") {
			w.Header().Add("Vary", "User-Agent")
			// Only to a route we know, so this can't be turned into an open redirect.
			if isSearchCrawler(r) && isAppRoute(target) {
				w.Header().Set("Cache-Control", "no-store")
				u := url.URL{Path: target, RawQuery: r.URL.RawQuery}
				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
				return
			}
		}
		if shell != nil && !isAppRoute(target) && !s.isWebFile(clean) && (path.Ext(clean) == "" || path.Ext(clean) == ".html") && !strings.HasPrefix(clean, "/_app/") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNotFound)
			if r.Method != http.MethodHead {
				w.Write(shell)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isWebFile reports whether the build has a file for this path, either as named
// or as a prerendered "<path>.html".
func (s *Server) isWebFile(clean string) bool {
	p := filepath.Join(s.webDir, filepath.FromSlash(clean))
	for _, f := range []string{p, p + ".html"} {
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
