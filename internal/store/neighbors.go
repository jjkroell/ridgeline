package store

// Neighbour reports: the zero-hop neighbour table a repeater publishes on its
// MQTT /neighbors topic (openHop's NeighborsPublisher, firmware
// WITH_MQTT_NEIGHBORS), typically once a day. Unlike the path-adjacency
// "neighbours" analytics infers from flood paths, each row here is a node the
// reporter heard directly, named by full public key, with the SNR it heard it
// at — a measurement, not an inference.
//
// Only the latest report per reporter is kept: the table is a snapshot, and
// the previous one says nothing the new one does not.

import (
	"encoding/json"
	"sort"
	"strings"
)

// NeighborEntry is one row of a neighbour table.
type NeighborEntry struct {
	PublicKey string `json:"publicKey"`
	// SNR is how well the REPORTER hears this neighbour, not the reverse.
	SNR          *float64 `json:"snr,omitempty"`
	HeardSecsAgo int64    `json:"heardSecsAgo"` // relative to ReportedAt; -1 = unknown (firmware sent null)
	Scopes       string   `json:"scopes,omitempty"`
	Status       string   `json:"status,omitempty"` // responded | timeout | send_failed
}

// NeighborReport is a reporter's latest neighbour table.
type NeighborReport struct {
	Reporter     string          `json:"reporter"` // pubkey UPPER
	ObserverID   string          `json:"observerId"`
	ReportedAt   string          `json:"reportedAt"`
	SelfScopes   string          `json:"selfScopes,omitempty"`
	DefaultScope string          `json:"defaultScope,omitempty"`
	Neighbors    []NeighborEntry `json:"neighbors"`
}

func (s *Store) loadNeighborReports() error {
	rows, err := s.db.Query(`SELECT reporter, observer_id, reported_at, self_scopes, default_scope, rows_json FROM neighbor_reports`)
	if err != nil {
		return err
	}
	defer rows.Close()
	m := map[string]NeighborReport{}
	for rows.Next() {
		var r NeighborReport
		var js string
		if err := rows.Scan(&r.Reporter, &r.ObserverID, &r.ReportedAt, &r.SelfScopes, &r.DefaultScope, &js); err != nil {
			return err
		}
		if json.Unmarshal([]byte(js), &r.Neighbors) != nil {
			continue // a bad row is skipped, not fatal; the next publish replaces it
		}
		m[r.Reporter] = r
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.neighborMu.Lock()
	s.neighborReports = m
	s.neighborMu.Unlock()
	return nil
}

// SaveNeighborReport replaces the reporter's stored table.
func (s *Store) SaveNeighborReport(r NeighborReport) error {
	r.Reporter = strings.ToUpper(r.Reporter)
	if r.Neighbors == nil {
		r.Neighbors = []NeighborEntry{}
	}
	js, err := json.Marshal(r.Neighbors)
	if err != nil {
		return err
	}
	s.mu.Lock()
	_, err = s.db.Exec(`
		INSERT INTO neighbor_reports (reporter, observer_id, reported_at, self_scopes, default_scope, rows_json)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(reporter) DO UPDATE SET
			observer_id   = excluded.observer_id,
			reported_at   = excluded.reported_at,
			self_scopes   = excluded.self_scopes,
			default_scope = excluded.default_scope,
			rows_json     = excluded.rows_json`,
		r.Reporter, r.ObserverID, r.ReportedAt, r.SelfScopes, r.DefaultScope, string(js))
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.neighborMu.Lock()
	if s.neighborReports == nil {
		s.neighborReports = map[string]NeighborReport{}
	}
	s.neighborReports[r.Reporter] = r
	s.neighborMu.Unlock()
	return nil
}

// NeighborReportOf returns the reporter's latest table, if any.
func (s *Store) NeighborReportOf(pubkey string) (NeighborReport, bool) {
	s.neighborMu.RLock()
	defer s.neighborMu.RUnlock()
	r, ok := s.neighborReports[strings.ToUpper(pubkey)]
	return r, ok
}

// NeighborSighting is one reporter that lists a node in its table.
type NeighborSighting struct {
	Reporter   string `json:"reporter"`
	ReportedAt string `json:"reportedAt"`
	Entry      NeighborEntry
}

// ReportersListing returns every report whose table lists pubkey, newest
// report first. A reporter never lists itself, so that case cannot arise.
func (s *Store) ReportersListing(pubkey string) []NeighborSighting {
	pk := strings.ToUpper(pubkey)
	s.neighborMu.RLock()
	defer s.neighborMu.RUnlock()
	var out []NeighborSighting
	for _, r := range s.neighborReports {
		for _, e := range r.Neighbors {
			if strings.ToUpper(e.PublicKey) == pk {
				out = append(out, NeighborSighting{Reporter: r.Reporter, ReportedAt: r.ReportedAt, Entry: e})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReportedAt > out[j].ReportedAt })
	return out
}

// IsObserverBlocked reports whether an observer id is on the blocklist.
func (s *Store) IsObserverBlocked(observerID string) bool {
	s.blockMu.RLock()
	defer s.blockMu.RUnlock()
	return observerID != "" && s.blockedObservers[observerID]
}
