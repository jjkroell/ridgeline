package analytics

import (
	"strings"
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

func key(prefix string) string {
	return prefix + strings.Repeat("0", 64-len(prefix))
}

func nd(k, name string, adv, hs int) store.Node {
	return store.Node{
		PublicKey:   key(k),
		Name:        name,
		AdvertCount: adv,
		HashSize:    hs,
		LastSeen:    "2026-06-23T00:00:00Z",
	}
}

func TestFindArtifacts(t *testing.T) {
	nodes := []store.Node{
		// 3-byte cohort: corrupted copy sharing 4 leading bytes → HIGH.
		nd("ABCDEF12", "Cougar", 100, 3),  // canonical
		nd("ABCDEF1299", "Cougar?", 1, 3), // shares ABCDEF12 (4 bytes) → high
		// 1-byte cohort: genuine distinct collision (share 1 byte, real names) → NOT flagged.
		nd("AA", "Node Alpha", 50, 1),
		nd("AABB", "Node Beta", 40, 1),
		// 1-byte cohort: identical name, short shared prefix → MEDIUM (could be a
		// ground look-alike key; never auto-scrubbed).
		nd("CC", "Repeater One", 100, 1), // canonical
		nd("CC22", "Repeater One", 1, 1), // identical name → medium
		// 1-byte cohort: garbage name next to a dominant node → MEDIUM.
		nd("DD", "Real Repeater", 50, 1),
		nd("DD22", "=", 1, 1),
		// 1-byte cohort: rarely-heard, shares 2 bytes with dominant → MEDIUM (not auto-scrubbed).
		nd("FFAA", "Busy Node", 100, 1),
		nd("FFAA99", "Other", 1, 1),
		// Cohort isolation: same first byte but different configured length → NOT a collision.
		nd("EE", "One-byte Node", 10, 1),
		nd("EE2222", "Three-byte Node", 1, 3),
	}

	arts := FindArtifacts(nodes)
	if len(arts) != 4 {
		t.Fatalf("expected 4 artifacts (1 high + 3 medium), got %d: %+v", len(arts), arts)
	}

	got := map[string]string{} // key prefix → confidence
	for _, a := range arts {
		got[a.Key[:10]] = a.Confidence
	}
	want := map[string]string{
		key("ABCDEF1299")[:10]: "high",
		key("CC22")[:10]:       "medium",
		key("DD22")[:10]:       "medium",
		key("FFAA99")[:10]:     "medium",
	}
	for k, conf := range want {
		if got[k] != conf {
			t.Errorf("key %s: want confidence %q, got %q", k, conf, got[k])
		}
	}

	// Genuine distinct nodes and cohort-isolated nodes must never be flagged.
	for _, a := range arts {
		switch a.Key {
		case key("AA"), key("AABB"), key("EE"), key("EE2222"):
			t.Errorf("flagged a genuine/isolated node as artifact: %s (%s)", a.Name, a.Reason)
		}
	}

	// Auto-scrub keys = high confidence only: just the 4-byte structural match.
	scrub := HighConfidenceArtifactKeys(nodes)
	if len(scrub) != 1 || scrub[0] != key("ABCDEF1299") {
		t.Fatalf("expected only the 4-byte match in the scrub set, got %v", scrub)
	}
}

// A look-alike key ground to a genuine node's prefix, copying its name and
// replaying one advert to inflate the raw count, must neither outrank the
// genuine node nor get it auto-scrubbed.
func TestGroundLookalikeCannotScrubGenuineNode(t *testing.T) {
	genuine := nd("F916", "Hilltop Rptr", 6, 1)
	genuine.AdvertTxCount = 6
	genuine.FirstSeen = "2026-01-01T00:00:00Z"
	attacker := nd("F94A", "Hilltop Rptr", 40, 1) // one advert replayed 40×
	attacker.AdvertTxCount = 1
	attacker.FirstSeen = "2026-06-22T00:00:00Z"

	for _, k := range HighConfidenceArtifactKeys([]store.Node{genuine, attacker}) {
		if k == genuine.PublicKey {
			t.Fatal("genuine node was put in the auto-scrub set")
		}
	}
	arts := FindArtifacts([]store.Node{genuine, attacker})
	if len(arts) != 1 || arts[0].Key != attacker.PublicKey || arts[0].Confidence != "medium" {
		t.Fatalf("want the newer look-alike flagged medium for review, got %+v", arts)
	}
}
