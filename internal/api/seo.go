package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// Search engines and AI crawlers that don't run JavaScript only ever see the
// SPA shell. shareHandler already writes each public route's head; this adds
// what a crawler needs beyond the head: a search-length title and description,
// a crawlable summary in the shell's <noscript> block, JSON-LD for the page,
// and a sitemap listing every recently heard node. Everything here comes from
// the same public node list /api/nodes serves.

// seoSitemapDays is how recently a node must have been heard to be listed in
// the sitemap and the directory summary. Older pages still resolve; they just
// aren't advertised to crawlers.
const seoSitemapDays = 30

// seoDirectoryMax caps the node links in the /nodes crawl summary.
const seoDirectoryMax = 400

// seoSitemapTTL is how long the generated sitemap is reused.
const seoSitemapTTL = 10 * time.Minute

const seoRegion = "Vancouver Island and the Lower Mainland of British Columbia"

type seoSection struct {
	Title, Description, Heading string
	Paras                       []string
	Freq                        string // sitemap changefreq
	Priority                    string
}

// seoSections mirror the titles and descriptions each route's Seo component
// sets once the app runs, so a crawler sees the same thing with or without
// JavaScript. /identity is omitted: robots.txt keeps it out of search.
var seoSections = map[string]seoSection{
	"/": {
		Title:       "Ridgeline — Live MeshCore Mesh Observatory for coastal BC",
		Description: "Live dashboard for the MeshCore LoRa mesh across Vancouver Island and the Lower Mainland — nodes, repeaters, coverage and packets on 909.000 MHz.",
		Heading:     "Ridgeline — MeshCore mesh observatory for coastal British Columbia",
		Paras: []string{
			"Ridgeline is a live observatory for the MeshCore LoRa mesh network across " + seoRegion + ". Receive-only observer stations report every packet they hear, and Ridgeline turns that into a node and repeater directory, a live packet feed, coverage maps and link-health analytics.",
			"MeshCore is an off-grid, long-range text messaging mesh built on low-cost LoRa radios. Repeaters on hilltops and rooftops relay messages between companion radios, with no internet or cell service needed. The coastal BC mesh runs on 909.000 MHz.",
		},
		Freq: "hourly", Priority: "1.0",
	},
	"/nodes": {
		Title:       "MeshCore Node & Repeater Directory · Ridgeline",
		Description: "Every node and repeater on the MeshCore LoRa mesh across Vancouver Island and the Lower Mainland — status, location and last-heard on 909.000 MHz.",
		Heading:     "MeshCore node and repeater directory",
		Paras: []string{
			"Every repeater, companion radio and room server heard on the MeshCore mesh across " + seoRegion + ", with when it was last heard. Each node has its own page with its adverts, relay activity, the observers that hear it and its coverage.",
		},
		Freq: "hourly", Priority: "0.9",
	},
	"/live": {
		Title:       "Live MeshCore Packet Feed · Ridgeline",
		Description: "Watch MeshCore mesh packets stream in real time across coastal BC — adverts, messages and traces on 909.000 MHz.",
		Heading:     "Live MeshCore packet feed",
		Paras: []string{
			"A real-time stream of the packets Ridgeline's observers hear on the MeshCore mesh across " + seoRegion + ": node adverts, public channel messages, traces and acknowledgements, with the path each one took through the repeaters.",
		},
		Freq: "always", Priority: "0.7",
	},
	"/map": {
		Title:       "MeshCore Coverage Map — Vancouver Island & Lower Mainland · Ridgeline",
		Description: "Interactive coverage map of the MeshCore LoRa mesh across coastal British Columbia — node and repeater locations on 909.000 MHz.",
		Heading:     "MeshCore coverage map for coastal British Columbia",
		Paras: []string{
			"An interactive map of the MeshCore mesh across " + seoRegion + ": where repeaters and other nodes are, the radio links observed between them, and the coverage they provide.",
		},
		Freq: "daily", Priority: "0.8",
	},
	"/live-map": {
		Title:       "Live MeshCore Signal Map · Ridgeline",
		Description: "Real-time signal map of the MeshCore LoRa mesh — watch packets and links light up across Vancouver Island and the Lower Mainland of British Columbia.",
		Heading:     "Live MeshCore signal map",
		Paras: []string{
			"Watch packets travel the MeshCore mesh as they are heard: each transmission lights up the repeaters and links it crossed across " + seoRegion + ".",
		},
		Freq: "hourly", Priority: "0.7",
	},
	"/analytics": {
		Title:       "MeshCore Mesh Analytics · Ridgeline",
		Description: "Network health, link quality and activity analytics for the MeshCore LoRa mesh across Vancouver Island and the Lower Mainland of British Columbia.",
		Heading:     "MeshCore mesh analytics",
		Paras: []string{
			"Network health for the MeshCore mesh across " + seoRegion + ": traffic over time, the busiest repeaters, link quality between nodes, hop counts and path-hash sizes.",
		},
		Freq: "daily", Priority: "0.7",
	},
	"/topology": {
		Title:       "Topology — Ridgeline",
		Description: "Interactive mesh topology for the MeshCore network — the relay backbone of nodes that hand off traffic to one another, inferred from observed flood paths.",
		Heading:     "MeshCore mesh topology",
		Paras: []string{
			"The relay backbone of the MeshCore mesh: which repeaters hand traffic to one another, inferred from the flood paths of packets heard across " + seoRegion + ".",
		},
		Freq: "daily", Priority: "0.6",
	},
	"/channels": {
		Title:       "MeshCore Channels · Ridgeline",
		Description: "Public channel activity on the MeshCore LoRa mesh across Vancouver Island and the Lower Mainland of coastal British Columbia.",
		Heading:     "MeshCore public channels",
		Paras: []string{
			"Messages on the MeshCore mesh's public and hashtag channels across " + seoRegion + ", including #sos, the mesh's permanent emergency channel.",
		},
		Freq: "hourly", Priority: "0.6",
	},
	"/observers": {
		Title:       "MeshCore Observers — receive-only mesh listeners · Ridgeline",
		Description: "The receive-only observer stations that feed Ridgeline's view of the MeshCore LoRa mesh across Vancouver Island and the Lower Mainland.",
		Heading:     "MeshCore observer stations",
		Paras: []string{
			"Observers are receive-only stations that report every MeshCore packet they hear to Ridgeline. Their combined view is what the maps, feed and analytics are built from.",
		},
		Freq: "daily", Priority: "0.6",
	},
}

// Prerendered pages: real HTML of their own, listed only in the sitemap.
var seoStaticPages = []struct{ Path, Freq, Priority string }{
	{"/about", "monthly", "0.9"},
	{"/hash-ids", "monthly", "0.8"},
}

// seoNav is the crawl summary's link list, the same set as app.html's.
var seoNav = [][2]string{
	{"/about", "About Ridgeline & the MeshCore mesh"},
	{"/nodes", "Node & repeater directory"},
	{"/live", "Live packet feed"},
	{"/map", "Coverage map"},
	{"/live-map", "Live signal map"},
	{"/analytics", "Network analytics"},
	{"/channels", "Channels"},
	{"/observers", "Observer receivers"},
	{"/hash-ids", "Multi-byte hash IDs"},
}

// seoDoc is the crawler-facing part of a page: the head's title and
// description, the <noscript> summary and the JSON-LD block.
type seoDoc struct {
	Title, Description string
	Body               string // HTML, already escaped
	JSONLD             string
}

func seoRoleWord(role string) string {
	switch strings.ToLower(role) {
	case "repeater":
		return "repeater"
	case "chatnode", "companion":
		return "companion radio"
	case "roomserver", "room", "room_server":
		return "room server"
	case "sensor":
		return "sensor"
	default:
		return "node"
	}
}

func seoDate(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ""
	}
	return t.UTC().Format("2 Jan 2006")
}

// publicNodes is the node list as /api/nodes publishes it: not retired, not
// quarantined.
func (s *Server) publicNodes() ([]store.Node, error) {
	all, err := s.cachedNodes()
	if err != nil {
		return nil, err
	}
	out := make([]store.Node, 0, len(all))
	for _, n := range activeNodes(all) {
		if !s.store.IsNodeBlocked(n.PublicKey) {
			out = append(out, n)
		}
	}
	return out, nil
}

// recentNodes returns public nodes heard in the last seoSitemapDays, newest first.
func (s *Server) recentNodes() ([]store.Node, error) {
	nodes, err := s.publicNodes()
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -seoSitemapDays)
	out := nodes[:0:0]
	for _, n := range nodes {
		if t, err := time.Parse(time.RFC3339Nano, n.LastSeen); err == nil && t.After(cutoff) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastSeen > out[j].LastSeen })
	return out, nil
}

func seoParas(b *strings.Builder, paras []string) {
	for _, p := range paras {
		fmt.Fprintf(b, "<p>%s</p>\n", html.EscapeString(p))
	}
}

func seoNavHTML(b *strings.Builder) {
	b.WriteString("<h2>Explore the network</h2>\n<ul>\n")
	for _, l := range seoNav {
		fmt.Fprintf(b, "<li><a href=\"%s\">%s</a></li>\n", l[0], html.EscapeString(l[1]))
	}
	b.WriteString("</ul>\n<p>Ridgeline's maps and live feed need JavaScript. Please enable it for the full experience.</p>\n")
}

func seoWrap(inner string) string {
	return `<main style="max-width: 44rem; margin: 0 auto; padding: 2rem 1.25rem; font-family: system-ui, sans-serif; line-height: 1.6;">` + "\n" + inner + "</main>\n"
}

// seoFor builds the crawler view of a public page. It never fails the page:
// if the node list can't be read, the summary just leaves it out.
func (s *Server) seoFor(site shareSite, p *sharePreview) *seoDoc {
	esc := html.EscapeString
	crumbs := [][2]string{{"Ridgeline", "/"}}
	var b strings.Builder
	doc := &seoDoc{}
	page := map[string]any{"@type": "WebPage"}

	if p.Node != nil {
		var n *store.Node
		if nodes, err := s.publicNodes(); err == nil {
			for i := range nodes {
				if nodes[i].PublicKey == p.Node.PublicKey {
					n = &nodes[i]
					break
				}
			}
		}
		role := seoRoleWord(p.Node.Role)
		doc.Title = fmt.Sprintf("%s — MeshCore %s · %s", p.Title, role, site.Name)
		desc := fmt.Sprintf("%s is a MeshCore %s on the mesh across Vancouver Island and the Lower Mainland, BC.", p.Title, role)
		var facts [][2]string
		facts = append(facts, [2]string{"Role", strings.ToUpper(role[:1]) + role[1:]})
		facts = append(facts, [2]string{"Public key", p.Node.PublicKey})
		if n != nil {
			if n.HashSize > 0 && n.HashSize <= 3 {
				facts = append(facts, [2]string{"Path hash ID", p.Node.PublicKey[:2*n.HashSize] + fmt.Sprintf(" (%d-byte)", n.HashSize)})
			}
			if d := seoDate(n.FirstSeen); d != "" {
				facts = append(facts, [2]string{"First heard", d})
			}
			if d := seoDate(n.LastSeen); d != "" {
				facts = append(facts, [2]string{"Last heard", d})
				desc += " Last heard " + d + "."
				page["dateModified"] = n.LastSeen
			}
		}
		desc += " Adverts, relay activity, observers and coverage."
		doc.Description = desc
		fmt.Fprintf(&b, "<h1>%s</h1>\n<p>%s</p>\n<ul>\n", esc(p.Title), esc(desc))
		for _, f := range facts {
			fmt.Fprintf(&b, "<li>%s: %s</li>\n", esc(f[0]), esc(f[1]))
		}
		b.WriteString("</ul>\n<p><a href=\"/nodes\">← All MeshCore nodes and repeaters</a> · <a href=\"/map\">Coverage map</a></p>\n")
		crumbs = append(crumbs, [2]string{"Nodes", "/nodes"}, [2]string{p.Title, p.Path})
		page["about"] = map[string]any{"@type": "Thing", "name": p.Title, "description": "MeshCore " + role + " " + p.Node.PublicKey[:12]}
	} else {
		sec, ok := seoSections[p.Path]
		if !ok {
			return nil
		}
		doc.Title, doc.Description = sec.Title, sec.Description
		fmt.Fprintf(&b, "<h1>%s</h1>\n", esc(sec.Heading))
		seoParas(&b, sec.Paras)
		if p.Path == "/nodes" {
			if nodes, err := s.recentNodes(); err == nil && len(nodes) > 0 {
				fmt.Fprintf(&b, "<h2>Heard in the last %d days</h2>\n<ul>\n", seoSitemapDays)
				for i, n := range nodes {
					if i == seoDirectoryMax {
						break
					}
					name := cleanShareText(n.Name, 100)
					if name == "" {
						name = "Node " + n.PublicKey[:12]
					}
					fmt.Fprintf(&b, "<li><a href=\"/nodes/%s\">%s</a> — %s, last heard %s</li>\n", n.PublicKey, esc(name), seoRoleWord(n.Role), seoDate(n.LastSeen))
				}
				b.WriteString("</ul>\n")
			}
		}
		if p.Path != "/" {
			crumbs = append(crumbs, [2]string{sec.Heading, p.Path})
		}
	}
	seoNavHTML(&b)
	doc.Body = seoWrap(b.String())

	page["@id"] = site.URL + p.Path + "#webpage"
	page["url"] = site.URL + p.Path
	page["name"] = doc.Title
	page["description"] = doc.Description
	page["inLanguage"] = "en-CA"
	page["isPartOf"] = map[string]string{"@id": site.URL + "/#website"}
	graph := []any{page}
	if len(crumbs) > 1 {
		items := make([]map[string]any, len(crumbs))
		for i, c := range crumbs {
			items[i] = map[string]any{"@type": "ListItem", "position": i + 1, "name": c[0], "item": site.URL + c[1]}
		}
		page["breadcrumb"] = map[string]string{"@id": site.URL + p.Path + "#breadcrumb"}
		graph = append(graph, map[string]any{"@type": "BreadcrumbList", "@id": site.URL + p.Path + "#breadcrumb", "itemListElement": items})
	}
	// json.Marshal escapes <, > and &, so no value can close the script element.
	if ld, err := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph}); err == nil {
		doc.JSONLD = string(ld)
	}
	return doc
}

// isNodePagePath reports whether path is a node page for a well-formed key
// (/nodes/<64 hex>, or its /m twin).
func isNodePagePath(path string) bool {
	key, ok := strings.CutPrefix(previewPath(path), "/nodes/")
	if !ok || len(key) != 64 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}

type sitemapCache struct {
	mu   sync.Mutex
	at   time.Time
	body []byte
}

// sitemap serves /sitemap.xml: the public sections, the prerendered pages and
// every node heard in the last seoSitemapDays.
func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	if s.site.URL == "" {
		http.NotFound(w, r) // a sitemap needs absolute URLs
		return
	}
	c := &s.sitemapCache
	c.mu.Lock()
	if c.body == nil || time.Since(c.at) > seoSitemapTTL {
		body, err := s.buildSitemap()
		if err != nil {
			c.mu.Unlock()
			s.log.Error("build sitemap", "error", err)
			http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		c.body, c.at = body, time.Now()
	}
	body := c.body
	c.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, "sitemap.xml", time.Time{}, bytes.NewReader(body))
}

func (s *Server) buildSitemap() ([]byte, error) {
	origin := s.site.URL
	if origin == "" {
		return nil, fmt.Errorf("no public site URL (share-site.json)")
	}
	nodes, err := s.recentNodes()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	url := func(path, lastmod, freq, prio string) {
		fmt.Fprintf(&b, "\t<url>\n\t\t<loc>%s</loc>\n", html.EscapeString(origin+path))
		if lastmod != "" {
			fmt.Fprintf(&b, "\t\t<lastmod>%s</lastmod>\n", lastmod)
		}
		fmt.Fprintf(&b, "\t\t<changefreq>%s</changefreq>\n\t\t<priority>%s</priority>\n\t</url>\n", freq, prio)
	}
	paths := make([]string, 0, len(seoSections))
	for p := range seoSections {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		a, c := seoSections[paths[i]], seoSections[paths[j]]
		if a.Priority != c.Priority {
			return a.Priority > c.Priority
		}
		return paths[i] < paths[j]
	})
	for _, p := range paths {
		url(p, "", seoSections[p].Freq, seoSections[p].Priority)
	}
	for _, p := range seoStaticPages {
		url(p.Path, "", p.Freq, p.Priority)
	}
	for _, n := range nodes {
		lastmod := ""
		if t, err := time.Parse(time.RFC3339Nano, n.LastSeen); err == nil {
			lastmod = t.UTC().Format("2006-01-02")
		}
		prio := "0.5"
		if r := seoRoleWord(n.Role); r == "repeater" || r == "room server" {
			prio = "0.6"
		}
		url("/nodes/"+n.PublicKey, lastmod, "daily", prio)
	}
	b.WriteString("</urlset>\n")
	return b.Bytes(), nil
}
