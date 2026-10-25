package store

import (
	"strings"
	"testing"
)

// RecentGroupText backs an anonymous endpoint; it must read only the requested
// window via idx_obs_received, never scan the whole observations table.
func TestRecentGroupTextUsesWindowIndex(t *testing.T) {
	st := testStore(t)
	rows, err := st.db.Query("EXPLAIN QUERY PLAN "+recentGroupTextSQL, "2026-01-01T00:00:00Z", 50)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatalf("scan: %v", err)
		}
		plan = append(plan, detail)
	}
	joined := strings.Join(plan, " | ")
	if !strings.Contains(joined, "idx_obs_received") || strings.Contains(joined, "SCAN observations") {
		t.Fatalf("RecentGroupText must SEARCH idx_obs_received, got plan: %s", joined)
	}
}

// One row per message hash, from its newest copy, newest message first.
func TestRecentGroupTextOneRowPerMessage(t *testing.T) {
	st := testStore(t)
	ins := func(hash, raw, at string) {
		if _, err := st.db.Exec(`
			INSERT INTO observations (message_hash, raw_hex, route_type, payload_type, path_hops, observer_id, received_at)
			VALUES (?,?,'Flood','GroupText',0,'obs',?)`, hash, raw, at); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	ins("h1", "AA01", "2026-06-01T00:00:01Z")
	ins("h1", "AA02", "2026-06-01T00:00:02Z")
	ins("h2", "BB01", "2026-06-01T00:00:03Z")
	ins("h0", "OLD0", "2025-01-01T00:00:00Z") // outside the window
	got, err := st.RecentGroupText("2026-01-01T00:00:00Z", 50)
	if err != nil {
		t.Fatalf("RecentGroupText: %v", err)
	}
	if len(got) != 2 || got[0].RawHex != "BB01" || got[1].RawHex != "AA02" {
		t.Fatalf("want [BB01 AA02], got %+v", got)
	}
}
