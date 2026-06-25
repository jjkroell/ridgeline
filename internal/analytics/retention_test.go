package analytics

import (
	"sort"
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

func TestStaleNodeKeys(t *testing.T) {
	cutoff := "2026-06-01T00:00:00Z"
	nodes := []store.Node{
		{PublicKey: "AAAA", LastSeen: "2026-05-01T00:00:00Z"}, // old advert → stale
		{PublicKey: "BBBB", LastSeen: "2026-06-15T00:00:00Z"}, // recent advert → keep
		{PublicKey: "CCCC", LastSeen: "2026-05-02T00:00:00Z"}, // old advert but relaying → keep
		{PublicKey: "DDDD", LastSeen: "2026-06-01T00:00:00Z"}, // exactly at cutoff → keep
		{PublicKey: "EEEE", LastSeen: ""},                     // never seen → skip
	}
	keep := map[string]LiveSignal{"CCCC": {RelayCount1h: 3}}

	got := StaleNodeKeys(nodes, keep, cutoff)
	sort.Strings(got)
	want := []string{"AAAA"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("StaleNodeKeys = %v, want %v", got, want)
	}
}
