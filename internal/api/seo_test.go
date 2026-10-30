package api

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

const seoShell = `<!doctype html><html><head><meta charset="utf-8"><title>Generic</title></head><body><div id="app">SPA</div><noscript><main><h1>Generic summary</h1></main></noscript><script src="/app.js"></script></body></html>`

// seoTestEnv is shareTestEnv with a shell that carries a <noscript> summary and
// a node heard just now (so it is inside the sitemap window).
func seoTestEnv(t *testing.T, name string) (*store.Store, string, http.Handler) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	pkt, err := meshcore.DecodeHex(claimAdvertHex)
	if err != nil {
		t.Fatal(err)
	}
	if name != "" {
		pkt.Advert.Name = name
	}
	if err = st.Record(store.Observation{Packet: pkt, RawHex: claimAdvertHex, ReceivedAt: time.Now().UTC().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{"index.html": seoShell, "share-site.json": `{"name":"Ridgeline","url":"https://mesh.example"}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", dir)
	return st, pkt.Advert.PublicKey, s.Handler()
}

var ldRe = regexp.MustCompile(`<script type="application/ld\+json" data-ridgeline-seo>(.*?)</script>`)

func TestSEONodePage(t *testing.T) {
	_, key, h := seoTestEnv(t, `Evil</noscript><script>alert(1)</script>`)
	rr := previewRequest(h, "GET", "/m/nodes/"+strings.ToLower(key))
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "Generic summary") || strings.Count(body, "<noscript>") != 1 {
		t.Fatal("shell summary not replaced exactly once")
	}
	if strings.Contains(body, "<script>alert(1)") {
		t.Fatal("node name not escaped")
	}
	if !strings.Contains(body, `<div id="app">SPA</div>`) || !strings.Contains(body, `<script src="/app.js"></script></body>`) {
		t.Fatal("app shell changed")
	}
	for _, want := range []string{"Public key: " + key, "Last heard ", `href="/nodes"`, " is a MeshCore "} {
		if !strings.Contains(body, want) {
			t.Errorf("summary missing %q", want)
		}
	}
	tags := headTags(t, body)
	if len(tags["title"]) != 1 || !strings.Contains(tags["title"][0], "— MeshCore") || tags["canonical"][0] != "https://mesh.example/nodes/"+key {
		t.Fatalf("head: %v %v", tags["title"], tags["canonical"])
	}
	m := ldRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no JSON-LD")
	}
	var ld struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(m[1]), &ld); err != nil || len(ld.Graph) != 2 || ld.Graph[1]["@type"] != "BreadcrumbList" {
		t.Fatalf("JSON-LD: %v %s", err, m[1])
	}
}

func TestSEOMissingNodeIs404(t *testing.T) {
	_, _, h := seoTestEnv(t, "")
	rr := previewRequest(h, "GET", "/nodes/"+strings.Repeat("A", 64))
	if rr.Code != 404 || rr.Body.String() != seoShell {
		t.Fatalf("unknown node: %d", rr.Code)
	}
	if rr := previewRequest(h, "GET", "/nodes/nope"); rr.Code != 200 {
		t.Fatal("malformed key should stay the plain SPA fallback")
	}
}

func TestSEODirectoryAndSections(t *testing.T) {
	_, key, h := seoTestEnv(t, "")
	body := previewRequest(h, "GET", "/nodes").Body.String()
	if !strings.Contains(body, `<a href="/nodes/`+key+`">`) {
		t.Fatal("directory summary lacks the node link")
	}
	for path, sec := range seoSections {
		body := previewRequest(h, "GET", path).Body.String()
		if !strings.Contains(body, sec.Heading) && !strings.Contains(body, strings.ReplaceAll(sec.Heading, "&", "&amp;")) {
			t.Errorf("%s: no crawl summary", path)
		}
		if headTags(t, body)["description"][0] != sec.Description {
			t.Errorf("%s: description", path)
		}
	}
}

func TestSitemap(t *testing.T) {
	_, key, h := seoTestEnv(t, "")
	rr := previewRequest(h, "GET", "/sitemap.xml")
	if rr.Code != 200 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "application/xml") {
		t.Fatal(rr.Code, rr.Header())
	}
	var set struct {
		URLs []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(rr.Body.Bytes(), &set); err != nil {
		t.Fatal(err)
	}
	locs := map[string]string{}
	for _, u := range set.URLs {
		locs[u.Loc] = u.LastMod
	}
	for _, want := range []string{"https://mesh.example/", "https://mesh.example/about", "https://mesh.example/nodes"} {
		if _, ok := locs[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	if lm, ok := locs["https://mesh.example/nodes/"+key]; !ok || len(lm) != 10 {
		t.Fatalf("node URL/lastmod: %q %v", lm, ok)
	}
}

// Stored roles are meshcore.DeviceRole names, not lowercase words.
func TestSEORoleNames(t *testing.T) {
	for role, want := range map[string]string{"Repeater": "repeater", "ChatNode": "companion radio", "RoomServer": "room server", "Sensor": "sensor", "Unknown": "node"} {
		if got := seoRoleWord(role); got != want {
			t.Errorf("seoRoleWord(%q) = %q", role, got)
		}
	}
	if shareRole("ChatNode") != "Companion" || shareRole("RoomServer") != "Room server" {
		t.Error("share card role labels")
	}
}
