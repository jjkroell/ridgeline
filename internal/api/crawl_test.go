package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const googlebotPhone = "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
const iphone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"

func uaRequest(h http.Handler, path, ua string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("User-Agent", ua)
	h.ServeHTTP(rr, r)
	return rr
}

// The route list must name exactly the pages the web app has, or a real page
// would answer 404.
func TestAppRoutesMatchWeb(t *testing.T) {
	root := filepath.Join("..", "..", "web", "src", "routes")
	if _, err := os.Stat(root); err != nil {
		t.Skip("web sources not present")
	}
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() && rel == "reports" {
			return filepath.SkipDir // dev-only pages, not in the repo
		}
		if d.IsDir() || d.Name() != "+page.svelte" {
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		route := "/"
		if dir != "." {
			route = "/" + dir
		}
		route = previewPath(route)
		if strings.Contains(route, "[") {
			route = route[:strings.Index(route, "[")] + "x"
			if !isAppRoute(route) {
				t.Errorf("dynamic route %s not recognised", route)
			}
			return nil
		}
		found[route] = true
		if !isAppRoute(route) {
			t.Errorf("route %s missing from appRoutes", route)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for r := range appRoutes {
		if !found[r] {
			t.Errorf("appRoutes has %s, which the web app does not", r)
		}
	}
}

func TestCrawlerMobileRedirect(t *testing.T) {
	_, key, h := seoTestEnv(t, "")
	for from, to := range map[string]string{
		"/m": "/", "/m/": "/", "/m/nodes": "/nodes", "/m/nodes/" + key: "/nodes/" + key,
		"/m/channels?channel=sos": "/channels?channel=sos", "/m/settings": "/settings",
	} {
		rr := uaRequest(h, from, googlebotPhone)
		if rr.Code != 301 || rr.Header().Get("Location") != to {
			t.Errorf("crawler %s: %d %q, want 301 %q", from, rr.Code, rr.Header().Get("Location"), to)
		}
		// The /m app stays put for people, and caches must not mix the two up.
		if rr = uaRequest(h, from, iphone); rr.Code != 200 || rr.Header().Get("Vary") == "" {
			t.Errorf("phone %s: %d vary %q", from, rr.Code, rr.Header().Get("Vary"))
		}
	}
	if rr := uaRequest(h, "/nodes", googlebotPhone); rr.Code != 200 {
		t.Errorf("crawler /nodes: %d", rr.Code)
	}
	for _, p := range []string{"/m/%5Cevil.example", "/m/%2Fevil.example", "/m//evil.example", "/m/nosuch"} {
		rr := uaRequest(h, p, googlebotPhone)
		if loc := rr.Header().Get("Location"); strings.HasPrefix(loc, "//") || strings.Contains(loc, `\`) || strings.Contains(loc, "://") {
			t.Errorf("%s redirects to %q", p, loc)
		}
	}
}

func TestUnknownPathIs404(t *testing.T) {
	_, _, h := seoTestEnv(t, "")
	for _, p := range []string{"/nosuch", "/meshcore", "/m/nosuch", "/nodes/a/b"} {
		if rr := previewRequest(h, "GET", p); rr.Code != 404 || rr.Body.String() != seoShell {
			t.Errorf("%s: %d", p, rr.Code)
		}
	}
	for _, p := range []string{"/", "/login", "/verify-email?token=x", "/observers/abc", "/m/login", "/contact", "/privacy", "/nodes/"} {
		if rr := previewRequest(h, "GET", p); rr.Code != 200 {
			t.Errorf("%s: %d, want 200", p, rr.Code)
		}
	}
	if rr := previewRequest(h, "GET", "/about"); rr.Code != 301 {
		t.Errorf("/about: %d", rr.Code)
	}
}

func TestDevIsNotIndexed(t *testing.T) {
	for _, c := range []struct{ env, want string }{{"", ""}, {"dev", "noindex, nofollow"}} {
		s := &Server{env: c.env, webDir: t.TempDir()}
		h := s.crawlRules(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		if got := previewRequest(h, "GET", "/").Header().Get("X-Robots-Tag"); got != c.want {
			t.Errorf("env %q: X-Robots-Tag %q, want %q", c.env, got, c.want)
		}
	}
}
