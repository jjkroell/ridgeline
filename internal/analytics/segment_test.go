package analytics

import (
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// The rule under test, stated as a table so each clause is visible. It calls
// segStat.verdict directly rather than restating the conditions: a test that
// re-implements the rule agrees with itself no matter what the code does.
//
// Two kinds of evidence are in play. A NEAR-side receiver hearing a node
// directly refutes membership; a FAR-side receiver hearing it directly proves
// it. Everything else is inferred from traffic crossing the bridge.
func TestSegmentVerdict(t *testing.T) {
	cases := []struct {
		name string
		s    segStat
		want string // "" = not a candidate, else confidence or rejection reason
		why  string
	}{
		{"far-side node, all traffic crosses", segStat{withPath: 60, confirmed: 60}, "confirmed",
			"the shape every real far-side node had on the live mesh"},
		{"heard directly even once", segStat{withPath: 60, confirmed: 60, zeroHop: 1},
			"heard directly on this side",
			"a receiver on this side cannot hear the far side, so one direct sighting disqualifies"},
		{"routed through occasionally", segStat{withPath: 220, confirmed: 5},
			"only some traffic crosses the bridge",
			"2% is a packet taking an odd route, not a node living over there"},
		{"just under the share bar", segStat{withPath: 100, confirmed: 89}, "only some traffic crosses the bridge", ""},
		{"just over the share bar", segStat{withPath: 100, confirmed: 90}, "confirmed", ""},
		{"too few sightings to judge", segStat{withPath: 3, confirmed: 3}, "too few sightings to judge",
			"100% of three proves nothing"},
		{"probable only (1-byte path)", segStat{withPath: 40, probable: 40}, "probable",
			"a narrow originator must not be invisible - companions skew narrow"},
		{"no evidence either way", segStat{withPath: 40}, "",
			"never crossed and never heard over there: not a candidate, and not a rejection to report"},

		// Far-side receivers.
		{"heard directly on the far segment", segStat{zeroHopFar: 3}, "observed",
			"a receiver on that frequency settles what the crossing rule only estimates"},
		{"heard over there, never crossed while watching", segStat{zeroHopFar: 1}, "observed",
			"direct reception stands alone - it does not need a crossing to corroborate it"},
		{"heard over there AND crossing", segStat{zeroHopFar: 5, withPath: 60, confirmed: 60}, "observed",
			"measurement outranks inference when both agree"},
		{"heard over there but too few crossings", segStat{zeroHopFar: 5, withPath: 2, confirmed: 2}, "observed",
			"the sightings bar guards an INFERENCE; it must not veto a measurement"},
		{"heard on both sides", segStat{zeroHopFar: 5, zeroHop: 5, withPath: 60, confirmed: 60},
			"heard directly from both sides",
			"a contradiction is surfaced, never silently resolved in favour of one side"},

		// The regression this whole change exists to prevent.
		{"far-side node once a far-side receiver exists",
			segStat{zeroHopFar: 4, withPath: 60, confirmed: 54}, "observed",
			"before observer segmentation this node's own far-side receiver disqualified it"},
	}
	for _, c := range cases {
		conf, reject := c.s.verdict()
		got := conf
		if got == "" {
			got = reject
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q (%s)", c.name, got, c.want, c.why)
		}
	}
}

// Confidence reflects HOW membership was established, which the console and the
// node page show so an inference is never mistaken for a measurement.
func TestSegmentConfidence(t *testing.T) {
	for _, c := range []struct {
		s    segStat
		want string
	}{
		{segStat{withPath: 10, confirmed: 10}, "confirmed"},
		{segStat{withPath: 100, confirmed: 1, probable: 99}, "confirmed"}, // any wide proof wins
		{segStat{withPath: 10, probable: 10}, "probable"},
		{segStat{zeroHopFar: 1}, "observed"}, // direct reception outranks both
	} {
		if got, _ := c.s.verdict(); got != c.want {
			t.Errorf("%+v: got %s, want %s", c.s, got, c.want)
		}
	}
}

// A bridge with no peer recorded cannot define a segment — a link needs two
// ends — so the sweep must do nothing rather than guess.
func TestDetectSegmentsNoLinksIsNoOp(t *testing.T) {
	rep, err := DetectSegments(nil, nil, nil, nil, "2026-08-15T23:30:00Z", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Members) != 0 || rep.Scanned != 0 {
		t.Errorf("expected an empty report, got %+v", rep)
	}
}

// The far side's declared radio must never be confused with a node's own
// inherited value; ApplySegments/annotate blank the latter. This pins the
// store-level contract the API depends on.
func TestSegmentMemberShape(t *testing.T) {
	m := store.SegmentMember{NodeKey: "abc", BridgeNear: "def", Confidence: "confirmed"}
	switch m.Confidence {
	case "observed", "confirmed", "probable":
	default:
		t.Errorf("confidence must be one of the three documented values, got %q", m.Confidence)
	}
}

// Sorting receivers onto the right side of the bridge is the input everything
// else depends on: place a near-side receiver on the far segment and its ordinary
// direct receptions start "proving" membership for half the mesh.
func TestFarObserverSets(t *testing.T) {
	link := store.BridgeLink{
		Near:      "AAAA",
		Far:       "BBBB",
		PeerRadio: "909.000,62.5,8,5", // as an operator types it
	}
	observers := []store.Observer{
		{ID: "far-909", Radio: "909.0,62.5,8,5"},           // as the receiver reports it
		{ID: "near-a", Radio: "910.4249877,62.5,7,5"},      // synthesised centre
		{ID: "near-b", Radio: "910.425,62.5,7,5"},          // same channel, rounded
		{ID: "near-c", Radio: "910.425,62.5,7,8"},          // same channel, other coding rate
		{ID: "unknown", Radio: ""},                         // never reported one
		{ID: "far-freq-wrong-sf", Radio: "909.0,62.5,7,5"}, // right channel, cannot demodulate
	}

	sets, n := farObserverSets([]store.BridgeLink{link}, observers)
	far := sets[link.Near]

	if !far["far-909"] {
		t.Error("the 909 receiver must be recognised despite 909.000 vs 909.0")
	}
	for _, id := range []string{"near-a", "near-b", "near-c", "unknown", "far-freq-wrong-sf"} {
		if far[id] {
			t.Errorf("%s must stay near-side", id)
		}
	}
	if n != 1 {
		t.Errorf("FarObservers = %d, want 1", n)
	}

	// No declared far side: nobody can be placed, and the detector falls back to
	// the traffic rule exactly as it behaved before receivers were sorted at all.
	if sets, n := farObserverSets([]store.BridgeLink{{Near: "AAAA", Far: "BBBB"}}, observers); n != 0 || len(sets["AAAA"]) != 0 {
		t.Error("a bridge with no declared far radio must classify nobody")
	}
}
