package analytics

import (
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// One fresh row ground to a genuine repeater's 2-byte prefix must not cancel
// the repeater's relay credit, in retention or in the hop resolver.
func TestLookalikeRowKeepsEstablishedHopOwner(t *testing.T) {
	repeater := store.Node{
		PublicKey: "F9160000000000000000", AdvertTxCount: 40,
		FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-10-01T00:00:00Z",
	}
	lookalike := store.Node{
		PublicKey: "F9169999999999999999", AdvertTxCount: 1,
		FirstSeen: "2026-10-20T00:00:00Z", LastSeen: "2026-10-20T00:00:00Z",
	}
	all := []store.Node{repeater, lookalike}

	stale := FilterRelayedWithin([]string{repeater.PublicKey}, all, map[string]bool{"F916": true}, 2)
	if len(stale) != 0 {
		t.Errorf("relaying repeater left in the purge list: %v", stale)
	}
	if got := newPrefixResolverMin(all, 2)("F916"); got != repeater.PublicKey {
		t.Errorf("resolver gave %q, want the established repeater", got)
	}

	// Two established rows on one prefix are genuinely ambiguous.
	twin := repeater
	twin.PublicKey = "F9168888888888888888"
	if got := newPrefixResolverMin([]store.Node{repeater, twin}, 2)("F916"); got != "" {
		t.Errorf("two established owners should stay ambiguous, got %q", got)
	}
}
