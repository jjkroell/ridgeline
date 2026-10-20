package api

import (
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// neighborReportMaxAge hides tables older than this. A repeater publishes daily
// by default (up to every 14 days), so two weeks keeps the slowest schedule
// visible while dropping a reporter that has stopped publishing.
const neighborReportMaxAge = 15 * 24 * time.Hour

// RFNeighbor is one directly-heard neighbour as shown on a node page.
type RFNeighbor struct {
	PublicKey string   `json:"publicKey"`
	Name      string   `json:"name,omitempty"`
	Role      string   `json:"role,omitempty"`
	SNR       *float64 `json:"snr,omitempty"` // as heard by the reporter
	HeardAt   string   `json:"heardAt"`       // reportedAt − heard_secs_ago
	Scopes    string   `json:"scopes,omitempty"`
	Status    string   `json:"status,omitempty"`
}

// RFNeighbors is what published neighbour tables say about one node: the table
// it published itself (Reported), and the other reporters whose tables list it
// (HeardBy — there SNR is how well that reporter hears THIS node).
type RFNeighbors struct {
	ReportedAt   string       `json:"reportedAt,omitempty"`
	SelfScopes   string       `json:"selfScopes,omitempty"`
	DefaultScope string       `json:"defaultScope,omitempty"`
	Reported     []RFNeighbor `json:"reported"`
	HeardBy      []RFNeighbor `json:"heardBy"`
}

func (s *Server) rfNeighbors(pubkey string, nodes []store.Node) *RFNeighbors {
	byKey := make(map[string]*store.Node, len(nodes))
	for i := range nodes {
		byKey[strings.ToUpper(nodes[i].PublicKey)] = &nodes[i]
	}
	cutoff := time.Now().Add(-neighborReportMaxAge)
	fresh := func(at string) (time.Time, bool) {
		t, err := time.Parse(time.RFC3339, at)
		return t, err == nil && t.After(cutoff)
	}
	row := func(pk string, e store.NeighborEntry, reportedAt time.Time) RFNeighbor {
		n := RFNeighbor{
			PublicKey: pk, SNR: e.SNR, Scopes: e.Scopes, Status: e.Status,
			HeardAt: reportedAt.Add(-time.Duration(e.HeardSecsAgo) * time.Second).UTC().Format(time.RFC3339),
		}
		if nd := byKey[pk]; nd != nil {
			n.Name, n.Role = nd.Name, nd.Role
		}
		return n
	}
	out := &RFNeighbors{Reported: []RFNeighbor{}, HeardBy: []RFNeighbor{}}
	if r, ok := s.store.NeighborReportOf(pubkey); ok {
		if t, ok := fresh(r.ReportedAt); ok {
			out.ReportedAt, out.SelfScopes, out.DefaultScope = r.ReportedAt, r.SelfScopes, r.DefaultScope
			for _, e := range r.Neighbors {
				if s.store.IsNodeBlocked(e.PublicKey) {
					continue
				}
				out.Reported = append(out.Reported, row(e.PublicKey, e, t))
			}
		}
	}
	for _, sg := range s.store.ReportersListing(pubkey) {
		t, ok := fresh(sg.ReportedAt)
		if !ok || s.store.IsNodeBlocked(sg.Reporter) {
			continue
		}
		n := row(sg.Reporter, sg.Entry, t)
		n.Scopes = "" // the listed node's scopes, not the reporter's — meaningless here
		out.HeardBy = append(out.HeardBy, n)
	}
	if out.ReportedAt == "" && len(out.HeardBy) == 0 {
		return nil
	}
	return out
}
